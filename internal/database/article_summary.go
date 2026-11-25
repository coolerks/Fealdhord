package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func (s *Store) ArticleSummarySource(ctx context.Context, id string, mode, version string) (string, error) {
	return articleSummarySource(ctx, s.queries, id, mode, version)
}

func articleSummarySource(ctx context.Context, q *sqlc.Queries, id, mode, version string) (string, error) {
	source, err := q.GetArticleSummarySource(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if mode == "feed" && source.FeedContentExplicit != 1 ||
		mode == "reader" && source.ReaderSourceVersion == "" {
		return "", ErrNotFound
	}
	if mode == "feed" && source.ContentHash != version ||
		mode == "reader" && source.ReaderSourceVersion != version {
		return "", ErrConflict
	}
	return source.Title, nil
}

func (s *Store) GetArticleSummary(ctx context.Context, params sqlc.GetArticleSummaryParams) (sqlc.ArticleSummary, error) {
	return s.queries.GetArticleSummary(ctx, params)
}

func (s *Store) SaveCurrentArticleSummary(ctx context.Context, params sqlc.SaveArticleSummaryParams) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if _, err := articleSummarySource(ctx, q, params.ArticleID, params.SourceMode, params.SourceVersion); err != nil {
			return err
		}
		settings, err := q.GetArticleAISettings(ctx)
		if err != nil {
			return err
		}
		if params.Status == "skipped_length" {
			if params.HanLimit != settings.SummaryHanLimit || params.EnglishLimit != settings.SummaryEnglishLimit {
				return ErrConflict
			}
		} else {
			target := strings.TrimSpace(settings.TranslationTargetLang)
			if target == "" {
				target = "zh-CN"
			}
			if !strings.EqualFold(params.TargetLang, target) {
				return ErrConflict
			}
		}
		return q.SaveArticleSummary(ctx, params)
	})
}
