// Package articlesummary generates and caches a compact digest of a complete article.
package articlesummary

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/reader"
	"github.com/coolerks/Fealdhord/internal/translation"
)

const (
	summaryQueueLimit = 2 * time.Minute
	summaryRunLimit   = 5 * time.Minute
)

type ModelResolver interface {
	Resolve(context.Context, string) (aiconfig.Model, ai.Model, error)
}

type Reader interface {
	Get(context.Context, reader.Request) reader.Result
}

type Source struct {
	ArticleID string `json:"articleId"`
	Mode      string `json:"mode"`
	Version   string `json:"version"`
}

type Request struct {
	Source    Source `json:"source"`
	Automatic bool   `json:"automatic"`
}

type Result struct {
	Status string `json:"status"`
	Text   string `json:"text"`
}

type Service struct {
	store    *database.Store
	resolver ModelResolver
	reader   Reader
	slots    chan struct{}
	mu       sync.Mutex
	active   map[runKey]*run
}

type runKey struct {
	Source
	targetLang string
}

type run struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	result  Result
	err     error
}

func New(store *database.Store, resolver ModelResolver, bodyReader Reader) *Service {
	return &Service{store: store, resolver: resolver, reader: bodyReader, slots: make(chan struct{}, 2), active: make(map[runKey]*run)}
}

func (s *Service) check(ctx context.Context, source Source) (string, error) {
	if s.store == nil {
		return "", failure("StorageError")
	}
	if source.ArticleID == "" || len(source.ArticleID) > 256 || len(source.Version) > 128 ||
		source.Version == "" || (source.Mode != "feed" && source.Mode != "reader") {
		return "", failure("ValidationError")
	}
	title, err := s.store.ArticleSummarySource(ctx, source.ArticleID, source.Mode, source.Version)
	return title, publicError(err)
}

func (s *Service) cached(ctx context.Context, source Source, settings sqlc.ArticleAiSetting) (Result, error) {
	record, err := s.store.GetArticleSummary(ctx, sqlc.GetArticleSummaryParams{
		ArticleID: source.ArticleID, SourceMode: source.Mode, SourceVersion: source.Version,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return Result{Status: "none"}, nil
	}
	if err != nil {
		return Result{}, publicError(err)
	}
	if record.Status == "skipped_length" &&
		(record.HanLimit != settings.SummaryHanLimit || record.EnglishLimit != settings.SummaryEnglishLimit) {
		return Result{Status: "none"}, nil
	}
	if record.Status != "skipped_length" && record.TargetLang != translation.NormalizeTargetLang(settings.TranslationTargetLang) {
		return Result{Status: "none"}, nil
	}
	if record.Status == "generated" {
		return Result{Status: "generated", Text: record.SummaryText}, nil
	}
	return Result{Status: "skipped"}, nil
}

func (s *Service) Get(ctx context.Context, source Source) (Result, error) {
	if _, err := s.check(ctx, source); err != nil {
		return Result{}, err
	}
	settings, err := s.store.ArticleAISettings(ctx)
	if err != nil {
		return Result{}, publicError(err)
	}
	return s.cached(ctx, source, settings)
}

func (s *Service) Generate(ctx context.Context, request Request) (Result, error) {
	source := request.Source
	if _, err := s.check(ctx, source); err != nil {
		return Result{}, err
	}
	settings, err := s.store.ArticleAISettings(ctx)
	if err != nil {
		return Result{}, publicError(err)
	}
	result, err := s.cached(ctx, source, settings)
	if err != nil || result.Status != "none" || (request.Automatic && settings.SummarizeOnOpen != 1) {
		return result, err
	}

	key := runKey{Source: source, targetLang: translation.NormalizeTargetLang(settings.TranslationTargetLang)}
	s.mu.Lock()
	job := s.active[key]
	if job != nil && job.waiters == 0 {
		s.mu.Unlock()
		select {
		case <-job.done:
			return s.Generate(ctx, request)
		case <-ctx.Done():
			return Result{}, publicError(ctx.Err())
		}
	}
	if job == nil {
		jobCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		job = &run{done: make(chan struct{}), cancel: cancel}
		s.active[key] = job
		go s.complete(jobCtx, key, job)
	}
	job.waiters++
	s.mu.Unlock()

	select {
	case <-job.done:
		return job.result, job.err
	case <-ctx.Done():
		s.mu.Lock()
		job.waiters--
		if job.waiters == 0 {
			job.cancel()
		}
		s.mu.Unlock()
		return Result{}, publicError(ctx.Err())
	}
}

func (s *Service) complete(ctx context.Context, key runKey, job *run) {
	defer job.cancel()
	job.result, job.err = s.generate(ctx, key)
	s.mu.Lock()
	delete(s.active, key)
	close(job.done)
	s.mu.Unlock()
}

func (s *Service) generate(ctx context.Context, key runKey) (Result, error) {
	queuedAt := time.Now()
	queueCtx, finishQueue := context.WithTimeout(ctx, summaryQueueLimit)
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-queueCtx.Done():
		err := queueCtx.Err()
		finishQueue()
		if errors.Is(err, context.DeadlineExceeded) {
			slog.WarnContext(ctx, "文章摘要排队超时", "component", "articlesummary", "stage", "queue", "code", "Timeout",
				"duration_ms", time.Since(queuedAt).Milliseconds())
		}
		return Result{}, publicError(err)
	}
	finishQueue()
	ctx, cancel := context.WithTimeout(ctx, summaryRunLimit)
	defer cancel()

	source := key.Source
	title, err := s.check(ctx, source)
	if err != nil {
		return Result{}, err
	}
	settings, err := s.store.ArticleAISettings(ctx)
	if err != nil {
		return Result{}, publicError(err)
	}
	if key.targetLang != translation.NormalizeTargetLang(settings.TranslationTargetLang) {
		return Result{}, failure("Conflict")
	}
	if cached, err := s.cached(ctx, source, settings); err != nil || cached.Status != "none" {
		return cached, err
	}
	detail := s.reader.Get(ctx, reader.Request{ArticleID: source.ArticleID, Mode: source.Mode})
	if !detail.OK || detail.Data == nil {
		if detail.Error != nil {
			return Result{}, failure(detail.Error.Code)
		}
		return Result{}, failure("BodyUnavailable")
	}
	if detail.Data.SourceVersion != source.Version {
		return Result{}, failure("Conflict")
	}
	body, dense := readableBody(detail.Data.HTML, detail.Data.Text)
	if body == "" || incomplete(body) {
		return Result{}, failure("BodyUnavailable")
	}
	short := withinLimits(body, settings.SummaryHanLimit, settings.SummaryEnglishLimit)
	record := sqlc.SaveArticleSummaryParams{
		ArticleID: source.ArticleID, SourceMode: source.Mode, SourceVersion: source.Version,
	}
	if short && !dense {
		record.Status = "skipped_length"
		record.HanLimit = settings.SummaryHanLimit
		record.EnglishLimit = settings.SummaryEnglishLimit
	} else {
		if s.resolver == nil {
			return Result{}, failure("ConfigurationError")
		}
		model, adapter, err := s.resolver.Resolve(ctx, "summary")
		if err != nil {
			return Result{}, publicError(err)
		}
		text, err := summarize(ctx, model, adapter, title, body, key.targetLang, short)
		if err != nil {
			return Result{}, publicError(err)
		}
		record.ModelID = model.ID
		record.TargetLang = key.targetLang
		record.SummaryText = text
		record.Status = "skipped_model"
		if text != "" {
			record.Status = "generated"
		}
	}
	record.UpdatedAt = time.Now().UTC().UnixMilli()
	if err := s.store.SaveCurrentArticleSummary(ctx, record); err != nil {
		return Result{}, publicError(err)
	}
	if record.Status == "generated" {
		return Result{Status: "generated", Text: record.SummaryText}, nil
	}
	return Result{Status: "skipped"}, nil
}

func failure(code string) error { return &ai.ModelError{Code: code} }

func publicError(err error) error {
	var modelErr *ai.ModelError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.Canceled):
		return failure("Cancelled")
	case errors.Is(err, context.DeadlineExceeded):
		return failure("Timeout")
	case errors.Is(err, database.ErrNotFound):
		return failure("BodyUnavailable")
	case errors.Is(err, database.ErrConflict):
		return failure("Conflict")
	case errors.As(err, &modelErr):
		return modelErr
	default:
		return failure("StorageError")
	}
}
