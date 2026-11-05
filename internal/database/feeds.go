package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"strings"
	"time"
)

type Subscription struct {
	ID, Title, URL, SiteURL, Description, ETag, LastModified, LastError, LastErrorCode string
	Revision, ErrorCount                                                               int64
	Archived                                                                           bool
	CheckedAt, SuccessAt, NextCheckAt                                                  *int64
	CategoryIDs                                                                        []string
}
type SubscriptionInput struct {
	ApplyExisting                    *bool
	URL, Title, SiteURL, Description string
	CategoryIDs                      []string
}
type FeedCheck struct {
	At, Next           int64
	ETag, LastModified string
	NotModified        bool
}

func intPointer(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
func subscription(row sqlc.Feed) Subscription {
	return Subscription{ID: row.ID, Title: row.Title, URL: row.NormalizedUrl, SiteURL: row.SiteUrl, Description: row.Description, ETag: row.Etag, LastModified: row.LastModified, LastError: row.LastError, LastErrorCode: row.LastErrorCode, Revision: row.Revision, ErrorCount: row.ErrorCount, Archived: row.ArchivedAt.Valid, CheckedAt: intPointer(row.LastCheckedAt), SuccessAt: intPointer(row.LastSuccessAt), NextCheckAt: intPointer(row.NextCheckAt)}
}
func (s *Store) GetSubscription(ctx context.Context, id string) (Subscription, error) {
	row, err := s.queries.GetFeed(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Subscription{}, ErrNotFound
	}
	if err != nil {
		return Subscription{}, err
	}
	result := subscription(row)
	result.CategoryIDs, err = s.queries.ListFeedCategories(ctx, id)
	if result.CategoryIDs == nil {
		result.CategoryIDs = []string{}
	}
	return result, err
}
func (s *Store) Subscriptions(ctx context.Context, includeArchived bool) ([]Subscription, error) {
	rows, err := s.queries.ListAllFeeds(ctx)
	if err != nil {
		return nil, err
	}
	result := []Subscription{}
	for _, row := range rows {
		if !includeArchived && row.ArchivedAt.Valid {
			continue
		}
		item := subscription(row)
		item.CategoryIDs, err = s.queries.ListFeedCategories(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		if item.CategoryIDs == nil {
			item.CategoryIDs = []string{}
		}
		result = append(result, item)
	}
	return result, nil
}
func (s *Store) DueSubscriptions(ctx context.Context, at int64) ([]Subscription, error) {
	rows, err := s.queries.ListDueFeeds(ctx, sql.NullInt64{Int64: at, Valid: true})
	if err != nil {
		return nil, err
	}
	out := []Subscription{}
	for _, row := range rows {
		out = append(out, subscription(row))
	}
	return out, nil
}
func feedInput(input SubscriptionInput) (SubscriptionInput, error) {
	canonical, err := normalizeURL(input.URL)
	if err != nil {
		return input, err
	}
	input.URL = canonical
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" || len(input.Title) > 4096 || len(input.Description) > 16384 || len(input.SiteURL) > 8192 || len(input.CategoryIDs) > 256 {
		return input, ErrInvalidInput
	}
	return input, nil
}
func setCategories(ctx context.Context, q *sqlc.Queries, id string, categories []string) error {
	if categories == nil {
		return nil
	}
	for _, category := range categories {
		if _, err := q.GetCategory(ctx, category); errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidInput
		} else if err != nil {
			return err
		}
	}
	if err := q.RemoveFeedCategories(ctx, id); err != nil {
		return err
	}
	now := time.Now().UTC().UnixMilli()
	for _, category := range categories {
		if err := q.SetFeedCategory(ctx, sqlc.SetFeedCategoryParams{FeedID: id, CategoryID: category, CreatedAt: now}); err != nil {
			return err
		}
	}
	return nil
}
func saveDetails(ctx context.Context, q *sqlc.Queries, id string, revision int64, input SubscriptionInput) error {
	rows, err := q.SaveFeedDetails(ctx, sqlc.SaveFeedDetailsParams{ID: id, Revision: revision, Title: input.Title, Url: input.URL, NormalizedUrl: input.URL, SiteUrl: input.SiteURL, Description: input.Description, UpdatedAt: time.Now().UTC().UnixMilli(), NormalizedUrl_2: input.URL, NormalizedUrl_3: input.URL, NormalizedUrl_4: input.URL})
	if err != nil {
		return err
	}
	if rows != 1 {
		return ErrConflict
	}
	return setCategories(ctx, q, id, input.CategoryIDs)
}
func commitFeed(ctx context.Context, q *sqlc.Queries, id, expectedURL string, items []ArticleInput, check FeedCheck) ([]string, error) {
	current, err := q.GetFeed(ctx, id)
	if err != nil {
		return nil, err
	}
	if current.ArchivedAt.Valid || current.NormalizedUrl != expectedURL {
		return nil, ErrConflict
	}
	if len(check.ETag) > 8192 || len(check.LastModified) > 8192 || len(items) > 10000 {
		return nil, ErrInvalidInput
	}
	ids := []string{}
	for _, item := range items {
		item.FeedID = id
		item.FetchedAt = check.At
		articleID, err := upsertArticle(ctx, q, item)
		if err != nil {
			return nil, err
		}
		ids = append(ids, articleID)
	}
	at, next := sql.NullInt64{Int64: check.At, Valid: true}, sql.NullInt64{Int64: check.Next, Valid: true}
	if check.NotModified {
		if len(items) != 0 {
			return nil, ErrInvalidInput
		}
		err = q.SaveFeedNotModified(ctx, sqlc.SaveFeedNotModifiedParams{ID: id, LastCheckedAt: at, LastSuccessAt: at, NextCheckAt: next, UpdatedAt: check.At})
	} else {
		err = q.SaveFeedSuccess(ctx, sqlc.SaveFeedSuccessParams{ID: id, Etag: check.ETag, LastModified: check.LastModified, LastCheckedAt: at, LastSuccessAt: at, NextCheckAt: next, UpdatedAt: check.At})
	}
	return ids, err
}
func (s *Store) CommitFeed(ctx context.Context, id, expectedURL string, items []ArticleInput, check FeedCheck) (ids []string, err error) {
	err = s.transaction(ctx, func(q *sqlc.Queries) error { ids, err = commitFeed(ctx, q, id, expectedURL, items, check); return err })
	if err != nil {
		return nil, err
	}
	return ids, nil
}
func (s *Store) AddSubscription(ctx context.Context, input SubscriptionInput, items []ArticleInput, check FeedCheck) (id string, err error) {
	input, err = feedInput(input)
	if err != nil {
		return "", err
	}
	err = s.transaction(ctx, func(q *sqlc.Queries) error {
		row, loadErr := q.GetFeedByURL(ctx, input.URL)
		if loadErr == nil {
			id = row.ID
			if !row.ArchivedAt.Valid {
				return nil
			}
			if err := q.RestoreFeed(ctx, sqlc.RestoreFeedParams{ID: id, UpdatedAt: check.At}); err != nil {
				return err
			}
			if err := setCategories(ctx, q, id, input.CategoryIDs); err != nil {
				return err
			}
		} else if errors.Is(loadErr, sql.ErrNoRows) {
			id = rand.Text()
			if err := q.CreateFeed(ctx, sqlc.CreateFeedParams{ID: id, Title: input.Title, Url: input.URL, NormalizedUrl: input.URL, CreatedAt: check.At, UpdatedAt: check.At}); err != nil {
				return err
			}
			if err := saveDetails(ctx, q, id, 0, input); err != nil {
				return err
			}
		} else {
			return loadErr
		}
		_, err := commitFeed(ctx, q, id, input.URL, items, check)
		return err
	})
	if err != nil {
		return "", err
	}
	return id, nil
}
func (s *Store) UpdateSubscription(ctx context.Context, id string, revision int64, input SubscriptionInput) error {
	input, err := feedInput(input)
	if err != nil {
		return err
	}
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		old, err := q.GetFeed(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if old.ArchivedAt.Valid {
			return ErrConflict
		}
		if duplicate, err := q.GetFeedByURL(ctx, input.URL); err == nil && duplicate.ID != id {
			return ErrConflict
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		categoriesChanged := false
		if input.CategoryIDs != nil {
			previous, err := q.ListFeedCategories(ctx, id)
			if err != nil {
				return err
			}
			categoriesChanged = len(previous) != len(input.CategoryIDs)
			wanted := map[string]bool{}
			for _, category := range input.CategoryIDs {
				wanted[category] = true
			}
			for _, category := range previous {
				if !wanted[category] {
					categoriesChanged = true
				}
			}
		}
		if err := saveDetails(ctx, q, id, revision, input); err != nil {
			return err
		}
		if categoriesChanged && (input.ApplyExisting == nil || *input.ApplyExisting) {
			if err := q.RemoveInheritedMemberships(ctx, id); err != nil {
				return err
			}
			now := time.Now().UTC().UnixMilli()
			return q.ApplyFeedCategories(ctx, sqlc.ApplyFeedCategoriesParams{FeedID: id, CreatedAt: now, UpdatedAt: now})
		}
		return nil
	})
}
func (s *Store) RestoreSubscription(ctx context.Context, id string, revision int64) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		old, err := q.GetFeed(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if old.Revision != revision {
			return ErrConflict
		}
		if !old.ArchivedAt.Valid {
			return nil
		}
		return q.RestoreFeed(ctx, sqlc.RestoreFeedParams{ID: id, UpdatedAt: time.Now().UTC().UnixMilli()})
	})
}
func (s *Store) FailSubscription(ctx context.Context, id, expectedURL, code, message string, at, next int64) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		return q.SaveFeedFailure(ctx, sqlc.SaveFeedFailureParams{ID: id, NormalizedUrl: expectedURL, LastError: message, LastErrorCode: code, LastCheckedAt: sql.NullInt64{Int64: at, Valid: true}, NextCheckAt: sql.NullInt64{Int64: next, Valid: true}, UpdatedAt: at})
	})
}

type CategorySummary struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	ParentID *string `json:"parent_id,omitempty"`
}

func (s *Store) Categories(ctx context.Context) ([]CategorySummary, error) {
	rows, err := s.queries.ListCategories(ctx)
	if err != nil {
		return nil, err
	}
	out := []CategorySummary{}
	for _, row := range rows {
		item := CategorySummary{ID: row.ID, Name: row.Name}
		if row.ParentID.Valid {
			v := row.ParentID.String
			item.ParentID = &v
		}
		out = append(out, item)
	}
	return out, nil
}
