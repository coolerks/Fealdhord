package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

var (
	ErrArticleAIDisabled   = errors.New("未启用文章 AI")
	ErrArticleAIModel      = errors.New("未配置文章 AI 模型")
	ErrArticleAIBudget     = errors.New("已达到文章 AI 预算")
	ErrArticleAICategories = errors.New("分类数量超过文章 AI 上限")
)

type ArticleAISnapshot struct {
	Article          sqlc.ArticleAIInputRow
	Settings         sqlc.ArticleAiSetting
	Categories       []sqlc.ArticleAICategoriesRow
	ModelID          string
	ModelRevision    int64
	ProviderRevision int64
	Task             string
	Key              string
	ProfileKey       string
}

type ArticleAIScores struct {
	Importance float64 `json:"importance"`
	Relevance  float64 `json:"relevance"`
	Reason     string  `json:"reason"`
}

type ArticleAIMatch struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

func (s *Store) ArticleAISettings(ctx context.Context) (sqlc.ArticleAiSetting, error) {
	return s.queries.GetArticleAISettings(ctx)
}

func (s *Store) SaveArticleAISettings(ctx context.Context, input sqlc.SaveArticleAISettingsParams) (row sqlc.ArticleAiSetting, err error) {
	err = s.transaction(ctx, func(q *sqlc.Queries) error {
		row, err = q.SaveArticleAISettings(ctx, input)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrConflict
		}
		if err != nil {
			return err
		}
		return q.IncrementSearchGeneration(ctx)
	})
	return row, err
}

func loadArticleAISnapshot(ctx context.Context, q *sqlc.Queries, id, task string, preview bool) (ArticleAISnapshot, error) {
	out := ArticleAISnapshot{Task: task}
	if task != "classification" && task != "featured" {
		return out, ErrInvalidInput
	}
	settings, err := q.GetArticleAISettings(ctx)
	if err != nil {
		return out, err
	}
	out.Settings = settings
	if !preview && (task == "classification" && settings.Classification == 0 || task == "featured" && settings.Featured == 0) {
		return out, ErrArticleAIDisabled
	}
	assignment, err := q.ResolveModelAssignment(ctx, task)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrArticleAIModel
	}
	if err != nil {
		return out, err
	}
	out.ModelID, out.ModelRevision, out.ProviderRevision = assignment.AiModel.ID, assignment.AiModel.Revision, assignment.AiProvider.Revision
	if task == "classification" {
		out.Categories, err = q.ArticleAICategories(ctx)
		if err != nil {
			return out, err
		}
		if len(out.Categories) > 128 {
			return out, ErrArticleAICategories
		}
	}
	data, err := json.Marshal(out)
	if err != nil {
		return out, err
	}
	sum := sha256.Sum256(append([]byte("article-ai-v1:"), data...))
	out.ProfileKey = hex.EncodeToString(sum[:])
	if id == "" {
		return out, nil
	}
	article, err := q.ArticleAIInput(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return out, ErrNotFound
	}
	if err != nil {
		return out, err
	}
	out.Article = article
	key := sha256.Sum256([]byte(out.ProfileKey + ":" + out.Article.ContentHash))
	out.Key = hex.EncodeToString(key[:])
	return out, nil
}

func (s *Store) ArticleAISnapshot(ctx context.Context, id, task string, preview bool) (ArticleAISnapshot, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return ArticleAISnapshot{}, err
	}
	defer tx.Rollback()
	return loadArticleAISnapshot(ctx, s.queries.WithTx(tx), id, task, preview)
}

func (s *Store) ArticleAICached(ctx context.Context, snapshot ArticleAISnapshot) (bool, error) {
	row, err := s.queries.GetArticleAIResult(ctx, sqlc.GetArticleAIResultParams{ArticleID: snapshot.Article.ID, Task: snapshot.Task})
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return row.InputKey == snapshot.Key, err
}

func (s *Store) ReserveArticleAI(ctx context.Context, snapshot ArticleAISnapshot, preview bool, estimate int64, now time.Time) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if err := verifyArticleAISnapshot(ctx, q, snapshot, preview); err != nil {
			return err
		}
		if err := q.PruneArticleAIAttempts(ctx, now.Add(-time.Hour).UnixMilli()); err != nil {
			return err
		}
		n, err := q.ReserveArticleAIAttempt(ctx, sqlc.ReserveArticleAIAttemptParams{ID: rand.Text(), StartedAt: now.UnixMilli(), Estimate: estimate})
		if err == nil && n == 0 {
			return ErrArticleAIBudget
		}
		return err
	})
}

func verifyArticleAISnapshot(ctx context.Context, q *sqlc.Queries, snapshot ArticleAISnapshot, preview bool) error {
	current, err := loadArticleAISnapshot(ctx, q, snapshot.Article.ID, snapshot.Task, preview)
	if err != nil {
		return err
	}
	if current.Key != snapshot.Key {
		return ErrConflict
	}
	return nil
}

func (s *Store) CommitArticleAI(ctx context.Context, snapshot ArticleAISnapshot, scores ArticleAIScores, matches []ArticleAIMatch, input, output sql.NullInt64, now time.Time) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if err := verifyArticleAISnapshot(ctx, q, snapshot, false); err != nil {
			return err
		}
		if snapshot.Task == "classification" {
			if err := q.DeleteArticleAIMemberships(ctx, snapshot.Article.ID); err != nil {
				return err
			}
			for _, match := range matches {
				err := q.AddArticleAIMembership(ctx, sqlc.AddArticleAIMembershipParams{
					ArticleID: snapshot.Article.ID, CategoryID: match.ID, Confidence: sql.NullFloat64{Float64: match.Confidence, Valid: true},
					Reason: sql.NullString{String: match.Reason, Valid: true}, ModelRef: sql.NullString{String: snapshot.ModelID, Valid: true},
					CreatedAt: now.UnixMilli(), UpdatedAt: now.UnixMilli(),
				})
				if err != nil {
					return err
				}
			}
		}
		if err := q.SaveArticleAIResult(ctx, sqlc.SaveArticleAIResultParams{
			ArticleID: snapshot.Article.ID, Task: snapshot.Task, InputKey: snapshot.Key, ProfileKey: snapshot.ProfileKey, ContentHash: snapshot.Article.ContentHash,
			ModelID: snapshot.ModelID, ModelRevision: snapshot.ModelRevision, ProviderRevision: snapshot.ProviderRevision, SettingsRevision: snapshot.Settings.Revision,
			Importance: scores.Importance, Relevance: scores.Relevance, Reason: scores.Reason, InputTokens: input, OutputTokens: output, EvaluatedAt: now.UnixMilli(),
		}); err != nil {
			return err
		}
		if snapshot.Task == "featured" {
			return q.IncrementSearchGeneration(ctx)
		}
		return nil
	})
}

func (s *Store) NextArticleAI(ctx context.Context, task string, cutoff time.Time) (string, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	q := s.queries.WithTx(tx)
	profile, err := loadArticleAISnapshot(ctx, q, "", task, false)
	if err != nil {
		return "", err
	}
	if task == "classification" && len(profile.Categories) == 0 {
		return "", nil
	}
	var id string
	err = tx.QueryRowContext(ctx, `SELECT a.id FROM articles a JOIN feeds f ON f.id=a.feed_id
WHERE f.archived_at IS NULL AND a.sort_at>=?
AND NOT EXISTS(SELECT 1 FROM article_ai_deferrals d WHERE d.article_id=a.id AND d.task=? AND d.until_at>?)
AND NOT EXISTS(SELECT 1 FROM article_ai_results r WHERE r.article_id=a.id AND r.task=? AND r.content_hash=a.content_hash AND r.profile_key=?)
ORDER BY a.sort_at DESC,a.id DESC LIMIT 1`, cutoff.UnixMilli(), task, time.Now().UnixMilli(), task, profile.ProfileKey).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

func (s *Store) DeferArticleAI(ctx context.Context, id, task string, until time.Time) error {
	return s.queries.RecordArticleAIDefer(ctx, sqlc.RecordArticleAIDeferParams{ArticleID: id, Task: task, UntilAt: until.UnixMilli()})
}
