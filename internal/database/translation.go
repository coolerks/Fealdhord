package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type ArticleTranslationSummary struct {
	Title   string
	Summary string
}

type ArticleTranslationInput struct {
	ID                string
	FeedID            string
	Title             string
	Summary           string
	ContentHTML       string
	ContentText       string
	URL               string
	ContentHash       string
	FeedURL           string
	AutoTranslateMode string
}

func (s *Store) GetArticleForTranslation(ctx context.Context, id string) (ArticleTranslationInput, error) {
	row, err := s.queries.GetArticleForTranslation(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return ArticleTranslationInput{}, ErrNotFound
	}
	if err != nil {
		return ArticleTranslationInput{}, err
	}
	mode := row.AutoTranslateMode
	if mode == "" {
		mode = "global"
	}
	return ArticleTranslationInput{
		ID:                row.ID,
		FeedID:            row.FeedID,
		Title:             row.Title,
		Summary:           row.Summary,
		ContentHTML:       row.ContentHtml,
		ContentText:       row.ContentText,
		URL:               row.Url,
		ContentHash:       row.ContentHash,
		FeedURL:           row.FeedUrl,
		AutoTranslateMode: mode,
	}, nil
}

func (s *Store) GetArticleTranslation(ctx context.Context, articleID, targetLang, sourceMode string) (sqlc.ArticleTranslation, error) {
	row, err := s.queries.GetArticleTranslation(ctx, sqlc.GetArticleTranslationParams{
		ArticleID:  articleID,
		TargetLang: targetLang,
		SourceMode: sourceMode,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return sqlc.ArticleTranslation{}, ErrNotFound
	}
	return row, err
}

func (s *Store) SaveArticleTranslation(ctx context.Context, params sqlc.SaveArticleTranslationParams) error {
	return s.queries.SaveArticleTranslation(ctx, params)
}

func (s *Store) CurrentTranslationSource(ctx context.Context, articleID string) (sqlc.GetCurrentTranslationSourceRow, error) {
	return s.queries.GetCurrentTranslationSource(ctx, articleID)
}

func (s *Store) TranslationBodyText(ctx context.Context, articleID, mode, version string) (string, error) {
	return s.queries.GetTranslationBodyText(ctx, sqlc.GetTranslationBodyTextParams{
		ArticleID: articleID, SourceMode: mode, SourceVersion: version,
	})
}

// SaveCurrentArticleTranslation prevents an in-flight result from replacing a newer source.
func (s *Store) SaveCurrentArticleTranslation(ctx context.Context, params sqlc.SaveArticleTranslationParams) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		source, err := q.GetCurrentTranslationSource(ctx, params.ArticleID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		if source.ContentHash != params.ContentHash ||
			(params.SourceMode == "reader" && (params.SourceVersion == "" || source.ReaderSourceVersion != params.SourceVersion)) {
			return ErrConflict
		}
		return q.SaveArticleTranslation(ctx, params)
	})
}

func (s *Store) DeleteArticleTranslations(ctx context.Context, articleID, targetLang string) error {
	if targetLang == "" {
		return s.queries.DeleteAllArticleTranslations(ctx, articleID)
	}
	return s.queries.DeleteArticleTranslations(ctx, sqlc.DeleteArticleTranslationsParams{
		ArticleID:  articleID,
		TargetLang: targetLang,
	})
}

// NextArticleForTranslation 返回最近窗口内需要后台自动翻译的一篇文章 ID。
// 遵循单源覆盖：off 永不自动翻译，on 强制自动翻译，global 跟随全局 auto_translate 开关。
func (s *Store) NextArticleForTranslation(ctx context.Context, since time.Time) (articleID string, scope string, targetLang string, err error) {
	settings, err := s.queries.GetArticleAISettings(ctx)
	if err != nil {
		return "", "", "", err
	}
	targetLang = strings.TrimSpace(settings.TranslationTargetLang)
	if targetLang == "" {
		targetLang = "zh-CN"
	}
	scope = settings.AutoTranslateScope
	if scope != "full" {
		scope = "summary"
	}
	nowMs := time.Now().UTC().UnixMilli()
	sinceMs := since.UTC().UnixMilli()

	// 先处理 summary 阶段（所有需自动翻译的文章都先翻标题与摘要）。
	querySummary := `SELECT a.id FROM articles a
JOIN feeds f ON f.id=a.feed_id
LEFT JOIN article_translations t ON t.article_id=a.id AND t.target_lang=? AND t.source_mode='summary' AND t.content_hash=a.content_hash
LEFT JOIN article_ai_deferrals d ON d.article_id=a.id AND d.task='translation' AND d.until_at>?
WHERE f.archived_at IS NULL
 AND a.sort_at>=?
 AND (f.auto_translate_mode='on' OR (f.auto_translate_mode='global' AND ?=1))
 AND d.article_id IS NULL
 AND (t.article_id IS NULL OR t.status IN ('pending','partial','failed'))
ORDER BY a.sort_at DESC, a.id DESC LIMIT 1`
	err = s.db.QueryRowContext(ctx, querySummary, targetLang, nowMs, sinceMs, settings.AutoTranslate).Scan(&articleID)
	if err == nil && articleID != "" {
		return articleID, "summary", targetLang, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", "", "", err
	}

	// 有摘要且已判断为目标语言时，后台无需再提取并翻译正文。
	if scope != "full" {
		return "", "", targetLang, nil
	}
	queryFull := `SELECT a.id FROM articles a
JOIN feeds f ON f.id=a.feed_id
JOIN article_translations ts ON ts.article_id=a.id AND ts.target_lang=? AND ts.source_mode='summary' AND ts.content_hash=a.content_hash
 AND (ts.status='completed' OR (ts.status='skipped_same_lang' AND trim(a.summary, char(9,10,13,32))=''))
LEFT JOIN reader_contents r ON r.article_id=a.id AND r.content_hash=a.content_hash AND r.source_version<>''
LEFT JOIN article_translations tf ON tf.article_id=a.id AND tf.target_lang=? AND tf.source_mode='reader' AND tf.content_hash=a.content_hash AND tf.source_version=r.source_version
 AND (tf.status='skipped_same_lang' OR (tf.status='completed' AND tf.translated_blocks=tf.total_blocks))
LEFT JOIN article_ai_deferrals d ON d.article_id=a.id AND d.task='translation_full' AND d.until_at>?
WHERE f.archived_at IS NULL
 AND a.sort_at>=?
 AND (f.auto_translate_mode='on' OR (f.auto_translate_mode='global' AND ?=1))
 AND d.article_id IS NULL
 AND tf.article_id IS NULL
ORDER BY a.sort_at DESC, a.id DESC LIMIT 1`
	err = s.db.QueryRowContext(ctx, queryFull, targetLang, targetLang, nowMs, sinceMs, settings.AutoTranslate).Scan(&articleID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", targetLang, nil
	}
	if err != nil {
		return "", "", "", err
	}
	return articleID, "full", targetLang, nil
}

// ArticleTranslationSummaries 批量读取指定文章在当前目标语言下已完成的标题与摘要译文。
func (s *Store) ArticleTranslationSummaries(ctx context.Context, articleIDs []string) (map[string]ArticleTranslationSummary, error) {
	if len(articleIDs) == 0 {
		return nil, nil
	}
	settings, err := s.queries.GetArticleAISettings(ctx)
	if err != nil {
		return nil, err
	}
	targetLang := strings.TrimSpace(settings.TranslationTargetLang)
	if targetLang == "" {
		targetLang = "zh-CN"
	}
	placeholders := make([]string, len(articleIDs))
	args := make([]any, 0, len(articleIDs)+1)
	args = append(args, targetLang)
	for i, id := range articleIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := `SELECT t.article_id, t.translated_title, t.translated_summary
FROM article_translations t
JOIN articles a ON a.id=t.article_id AND a.content_hash=t.content_hash
WHERE t.target_lang=? AND t.source_mode='summary' AND t.status='completed'
 AND t.article_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]ArticleTranslationSummary, len(articleIDs))
	for rows.Next() {
		var id, title, summary string
		if err := rows.Scan(&id, &title, &summary); err != nil {
			return nil, err
		}
		if title != "" || summary != "" {
			out[id] = ArticleTranslationSummary{Title: title, Summary: summary}
		}
	}
	return out, rows.Err()
}

// ReserveTranslationAttempt 复用文章 AI 滚动小时预算，防止后台自动翻译失控消耗。
func (s *Store) ReserveTranslationAttempt(ctx context.Context, estimate int64, now time.Time) error {
	if estimate < 1 {
		estimate = 1
	}
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if err := q.PruneArticleAIAttempts(ctx, now.Add(-time.Hour).UTC().UnixMilli()); err != nil {
			return err
		}
		rows, err := q.ReserveArticleAIAttempt(ctx, sqlc.ReserveArticleAIAttemptParams{
			ID:        randomID(),
			StartedAt: now.UTC().UnixMilli(),
			Estimate:  estimate,
		})
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrArticleAIBudget
		}
		return nil
	})
}

func randomID() string {
	return "tr-" + rand.Text()
}

func (s *Store) GetTranslationProvider(ctx context.Context, id string) (sqlc.TranslationProvider, error) {
	row, err := s.queries.GetTranslationProvider(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return sqlc.TranslationProvider{}, ErrNotFound
	}
	return row, err
}

func (s *Store) ListTranslationProviders(ctx context.Context) ([]sqlc.TranslationProvider, error) {
	return s.queries.ListTranslationProviders(ctx)
}

func (s *Store) SaveTranslationProvider(ctx context.Context, params sqlc.SaveTranslationProviderParams) error {
	return s.queries.SaveTranslationProvider(ctx, params)
}
