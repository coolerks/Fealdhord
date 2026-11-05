package database

import (
	"context"
	"database/sql"
	"errors"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"time"
)

type MediaCandidate struct{ URL, Source, Alt string }
type MediaRecord struct {
	ArticleID, FeedID, ArticleURL, Status, URL, RemoteURL, Source, Alt, CacheKey, MIME string
	Generation, Width, Height                                                          int64
	CheckedAt, RetryAfter                                                              *int64
	Archived                                                                           bool
	Candidates                                                                         []MediaCandidate
}

func (s *Store) ArticleIDsForInputs(ctx context.Context, feedID string, items []ArticleInput) ([]string, error) {
	ids := make([]string, len(items))
	for i, item := range items {
		kind, value := "fingerprint", articleHash(item)
		if item.GUID != "" {
			kind, value = "guid", item.GUID
		} else if item.URL != "" {
			normalized, err := normalizeURL(item.URL)
			if err != nil {
				continue
			}
			kind, value = "url", normalized
		}
		id, err := s.queries.FindIdentity(ctx, sqlc.FindIdentityParams{FeedID: feedID, Kind: kind, Value: value})
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		ids[i] = id
	}
	return ids, nil
}
func (s *Store) RegisterArticleMedia(ctx context.Context, id string, input ArticleInput, candidates []MediaCandidate) (changed bool, err error) {
	if id == "" || len(candidates) > 24 {
		return false, ErrInvalidInput
	}
	err = s.transaction(ctx, func(q *sqlc.Queries) error {
		article, err := q.GetArticle(ctx, id)
		if err != nil {
			return err
		}
		if article.ContentHash != articleHash(input) {
			return ErrConflict
		}
		old, err := q.GetMediaCandidates(ctx, id)
		if err != nil {
			return err
		}
		same := len(old) == len(candidates)
		if same {
			for i, item := range old {
				candidate := candidates[i]
				if item.Url != candidate.URL || item.Source != candidate.Source || item.Alt != candidate.Alt {
					same = false
					break
				}
			}
		}
		if same {
			if _, err := q.GetArticleMedia(ctx, id); err == nil {
				return nil
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if err := q.RegisterMedia(ctx, id); err != nil {
			return err
		}
		if err := q.DeleteMediaCandidates(ctx, id); err != nil {
			return err
		}
		for index, candidate := range candidates {
			if candidate.URL == "" || len(candidate.URL) > 8192 || len(candidate.Alt) > 1024 {
				return ErrInvalidInput
			}
			if err := q.AddMediaCandidate(ctx, sqlc.AddMediaCandidateParams{ArticleID: id, Position: int64(index), Url: candidate.URL, Source: candidate.Source, Alt: candidate.Alt}); err != nil {
				return err
			}
		}
		changed = true
		return nil
	})
	return changed, err
}
func (s *Store) GetMedia(ctx context.Context, id string) (MediaRecord, error) {
	row, err := s.queries.GetArticleMedia(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return MediaRecord{}, ErrNotFound
	}
	if err != nil {
		return MediaRecord{}, err
	}
	result := MediaRecord{ArticleID: row.ArticleID, FeedID: row.FeedID, ArticleURL: row.ArticleUrl, Status: row.Status, URL: row.Url.String, RemoteURL: row.RemoteUrl, Source: row.Source.String, Alt: row.Alt, CacheKey: row.CacheKey.String, MIME: row.Mime, Generation: row.Generation, Width: row.Width.Int64, Height: row.Height.Int64, CheckedAt: intPointer(row.CheckedAt), RetryAfter: intPointer(row.RetryAfter), Archived: row.ArchivedAt.Valid, Candidates: []MediaCandidate{}}
	candidates, err := s.queries.GetMediaCandidates(ctx, id)
	if err != nil {
		return MediaRecord{}, err
	}
	for _, candidate := range candidates {
		result.Candidates = append(result.Candidates, MediaCandidate{URL: candidate.Url, Source: candidate.Source, Alt: candidate.Alt})
	}
	return result, nil
}
func (s *Store) SetMedia(ctx context.Context, record MediaRecord) error {
	if record.Status != "none" && record.Status != "pending" && record.Status != "ready" && record.Status != "failed" && record.Status != "blocked" {
		return ErrInvalidInput
	}
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		rows, err := q.SetMediaState(ctx, sqlc.SetMediaStateParams{ArticleID: record.ArticleID, Generation: record.Generation, Status: record.Status, Url: nullString(record.URL), RemoteUrl: record.RemoteURL, Source: nullString(record.Source), Width: sql.NullInt64{Int64: record.Width, Valid: record.Width > 0}, Height: sql.NullInt64{Int64: record.Height, Valid: record.Height > 0}, Alt: record.Alt, CheckedAt: nullTime(record.CheckedAt), RetryAfter: nullTime(record.RetryAfter), CacheKey: nullString(record.CacheKey), Mime: record.MIME})
		if err == nil && rows != 1 {
			return ErrConflict
		}
		return err
	})
}
func (s *Store) BlockMedia(ctx context.Context) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error { return q.BlockAllMedia(ctx) })
}
func (s *Store) PendingMedia(ctx context.Context) ([]string, error) {
	return s.queries.ListPendingMedia(ctx, sql.NullInt64{Int64: time.Now().UTC().UnixMilli(), Valid: true})
}
