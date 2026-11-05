package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

var (
	ErrIdentityConflict = errors.New("文章身份冲突，原内容未覆盖")
	ErrNotFound         = errors.New("目标不存在")
	ErrConflict         = errors.New("目标已更新")
	ErrInvalidInput     = errors.New("输入无效")
)

func normalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Hostname() == "" || u.User != nil {
		return "", ErrInvalidInput
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", ErrInvalidInput
	}
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	u.Host = host
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String(), nil
}
func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}
func nullTime(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}
func (s *Store) transaction(ctx context.Context, work func(*sqlc.Queries) error) error {
	s.write.Lock()
	defer s.write.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := work(s.queries.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// EnsureFeed 仅登记身份，不发起网络请求；恢复归档订阅仍使用原 ID。
func (s *Store) EnsureFeed(ctx context.Context, rawURL, title string) (id string, err error) {
	canonical, err := normalizeURL(rawURL)
	if err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 4096 {
		return "", ErrInvalidInput
	}
	err = s.transaction(ctx, func(q *sqlc.Queries) error {
		feed, err := q.GetFeedByURL(ctx, canonical)
		if err == nil {
			id = feed.ID
			if !feed.ArchivedAt.Valid {
				return nil
			}
			return q.RestoreFeed(ctx, sqlc.RestoreFeedParams{ID: id, UpdatedAt: time.Now().UTC().UnixMilli()})
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		id = rand.Text()
		now := time.Now().UTC().UnixMilli()
		return q.CreateFeed(ctx, sqlc.CreateFeedParams{ID: id, Title: title, Url: rawURL, NormalizedUrl: canonical, CreatedAt: now, UpdatedAt: now})
	})
	return id, err
}

func (s *Store) ArchiveFeed(ctx context.Context, id string, revision int64) error {
	now := time.Now().UTC().UnixMilli()
	rows, err := s.queries.ArchiveFeed(ctx, sqlc.ArchiveFeedParams{ID: id, Revision: revision, ArchivedAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now})
	if err == nil && rows != 1 {
		if _, loadErr := s.queries.GetFeed(ctx, id); errors.Is(loadErr, sql.ErrNoRows) {
			return ErrNotFound
		} else if loadErr != nil {
			return loadErr
		}
		return ErrConflict
	}
	return err
}

type ArticleInput struct {
	FeedID, GUID, Title, URL, Author, Summary, HTML, Text string
	PublishedAt, UpdatedAt                                *int64
	FetchedAt                                             int64
}

func articleHash(input ArticleInput) string {
	sum := sha256.Sum256([]byte(input.Title + "\n" + input.Author + "\n" + input.Summary + "\n" + input.HTML + "\n" + input.Text))
	return hex.EncodeToString(sum[:])
}

type identity struct{ kind, value string }

func (s *Store) UpsertArticle(ctx context.Context, input ArticleInput) (id string, err error) {
	err = s.transaction(ctx, func(q *sqlc.Queries) error { id, err = upsertArticle(ctx, q, input); return err })
	if err != nil {
		return "", err
	}
	return id, nil
}

func upsertArticle(ctx context.Context, q *sqlc.Queries, input ArticleInput) (id string, err error) {
	created := false
	if input.FeedID == "" || len(input.GUID) > 4096 || len(input.URL) > 8192 || len(input.Title) > 4096 || len(input.Summary) > 16384 || len(input.Author) > 1024 || len(input.Text)+len(input.HTML) > 4<<20 {
		return "", ErrInvalidInput
	}
	canonical := ""
	if input.URL != "" {
		canonical, err = normalizeURL(input.URL)
		if err != nil {
			return "", err
		}
	}
	hash := articleHash(input)
	aliases := []identity{}
	if input.GUID != "" {
		aliases = append(aliases, identity{"guid", input.GUID})
	}
	if canonical != "" {
		aliases = append(aliases, identity{"url", canonical})
	}
	if len(aliases) == 0 {
		aliases = append(aliases, identity{"fingerprint", hash})
	}
	if input.FetchedAt == 0 {
		input.FetchedAt = time.Now().UTC().UnixMilli()
	}
	terms := CJKTerms(input.Title + "\n" + input.Author + "\n" + input.Summary + "\n" + input.Text)
	err = func() error {
		matchedGUID := false
		for _, alias := range aliases {
			found, err := q.FindIdentity(ctx, sqlc.FindIdentityParams{FeedID: input.FeedID, Kind: alias.kind, Value: alias.value})
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return err
			}
			if id != "" && id != found {
				return ErrIdentityConflict
			}
			id = found
			if alias.kind == "guid" {
				matchedGUID = true
			}
		}
		if id == "" {
			created = true
			id = rand.Text()
			sortAt := input.FetchedAt
			if input.PublishedAt != nil {
				sortAt = *input.PublishedAt
			}
			if err := q.CreateArticle(ctx, sqlc.CreateArticleParams{ID: id, FeedID: input.FeedID, Guid: nullString(input.GUID), Title: input.Title, Url: input.URL, NormalizedUrl: nullString(canonical), Author: input.Author, Summary: input.Summary, ContentHtml: input.HTML, ContentText: input.Text, CjkTerms: terms, PublishedAt: nullTime(input.PublishedAt), UpdatedAt: nullTime(input.UpdatedAt), FetchedAt: input.FetchedAt, SortAt: sortAt, ContentHash: hash, DedupKey: aliases[0].kind + ":" + aliases[0].value}); err != nil {
				return err
			}
		} else {
			old, err := q.GetArticle(ctx, id)
			if err != nil {
				return err
			}
			if matchedGUID && old.NormalizedUrl.Valid && canonical != "" && old.NormalizedUrl.String != canonical && old.ContentHash != hash {
				return ErrIdentityConflict
			}
			actualURL := input.URL
			if actualURL == "" {
				actualURL = old.Url
			}
			if err := q.UpdateArticle(ctx, sqlc.UpdateArticleParams{ID: id, Guid: nullString(input.GUID), Title: input.Title, Url: actualURL, NormalizedUrl: nullString(canonical), Author: input.Author, Summary: input.Summary, ContentHtml: input.HTML, ContentText: input.Text, CjkTerms: terms, PublishedAt: nullTime(input.PublishedAt), UpdatedAt: nullTime(input.UpdatedAt), ContentHash: hash}); err != nil {
				return err
			}
		}
		for _, alias := range aliases {
			if err := q.InsertIdentity(ctx, sqlc.InsertIdentityParams{ArticleID: id, FeedID: input.FeedID, Kind: alias.kind, Value: alias.value}); err != nil {
				return err
			}
		}
		if !created {
			return nil
		}
		return q.InheritFeedCategories(ctx, sqlc.InheritFeedCategoriesParams{ArticleID: id, FeedID: input.FeedID, CreatedAt: input.FetchedAt, UpdatedAt: input.FetchedAt})
	}()
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Store) ListArticles(ctx context.Context, limit int64) ([]sqlc.ListArticlesRow, error) {
	if limit < 1 || limit > 1000 {
		return nil, ErrInvalidInput
	}
	return s.queries.ListArticles(ctx, limit)
}
func (s *Store) SetRead(ctx context.Context, id string, value bool) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if _, err := q.GetArticle(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		var read int64
		var at sql.NullInt64
		now := time.Now().UTC().UnixMilli()
		if value {
			read = 1
			at = sql.NullInt64{Int64: now, Valid: true}
		}
		return q.SetRead(ctx, sqlc.SetReadParams{ArticleID: id, IsRead: read, ReadAt: at, UpdatedAt: now})
	})
}
func (s *Store) SetFavorite(ctx context.Context, id string, value bool) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if _, err := q.GetArticle(ctx, id); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if value {
			return q.AddFavorite(ctx, sqlc.AddFavoriteParams{ArticleID: id, CreatedAt: time.Now().UTC().UnixMilli()})
		}
		return q.RemoveFavorite(ctx, id)
	})
}
func (s *Store) CreateCategory(ctx context.Context, name, parentID string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 256 {
		return "", ErrInvalidInput
	}
	id := rand.Text()
	now := time.Now().UTC().UnixMilli()
	err := s.queries.CreateCategory(ctx, sqlc.CreateCategoryParams{ID: id, ParentID: nullString(parentID), Name: name, NormalizedName: strings.ToLower(name), CreatedAt: now, UpdatedAt: now})
	return id, err
}
func (s *Store) MoveCategory(ctx context.Context, id, parentID string, revision int64) error {
	rows, err := s.queries.MoveCategory(ctx, sqlc.MoveCategoryParams{ID: id, ParentID: nullString(parentID), Revision: revision, UpdatedAt: time.Now().UTC().UnixMilli()})
	if err == nil && rows != 1 {
		if _, loadErr := s.queries.GetCategory(ctx, id); errors.Is(loadErr, sql.ErrNoRows) {
			return ErrNotFound
		} else if loadErr != nil {
			return loadErr
		}
		return ErrConflict
	}
	return err
}
func (s *Store) DeleteCategory(ctx context.Context, id string) error {
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		category, err := q.GetCategory(ctx, id)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := q.MoveChildrenToParent(ctx, sqlc.MoveChildrenToParentParams{ParentID: category.ParentID, ParentID_2: nullString(id), UpdatedAt: time.Now().UTC().UnixMilli()}); err != nil {
			return err
		}
		_, err = q.DeleteCategory(ctx, id)
		return err
	})
}
func (s *Store) SetMembership(ctx context.Context, articleID, categoryID, source, state string, confidence *float64) error {
	if source != "manual" && source != "feed" && source != "ai" {
		return ErrInvalidInput
	}
	if confidence != nil && (math.IsNaN(*confidence) || math.IsInf(*confidence, 0) || *confidence < 0 || *confidence > 1) {
		return ErrInvalidInput
	}
	if state != "included" && state != "excluded" {
		return ErrInvalidInput
	}
	now := time.Now().UTC().UnixMilli()
	score := sql.NullFloat64{}
	if confidence != nil {
		score = sql.NullFloat64{Float64: *confidence, Valid: true}
	}
	return s.queries.SetMembership(ctx, sqlc.SetMembershipParams{ArticleID: articleID, CategoryID: categoryID, Source: source, State: state, Confidence: score, CreatedAt: now, UpdatedAt: now})
}
func (s *Store) EffectiveMembers(ctx context.Context, categoryID string) ([]string, error) {
	return s.queries.EffectiveMembers(ctx, categoryID)
}
func (s *Store) ResetManualMembership(ctx context.Context, articleID, categoryID string) error {
	return s.queries.ResetManualMembership(ctx, sqlc.ResetManualMembershipParams{ArticleID: articleID, CategoryID: categoryID})
}
func (s *Store) DeleteArticle(ctx context.Context, id string) error {
	rows, err := s.queries.DeleteArticle(ctx, id)
	if err == nil && rows != 1 {
		return ErrNotFound
	}
	return err
}

func (s *Store) SetFeedCategories(ctx context.Context, feedID string, categoryIDs []string, applyExisting bool) error {
	if len(categoryIDs) > 256 {
		return ErrInvalidInput
	}
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		if _, err := q.GetFeed(ctx, feedID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if err := q.RemoveFeedCategories(ctx, feedID); err != nil {
			return err
		}
		now := time.Now().UTC().UnixMilli()
		for _, id := range categoryIDs {
			if err := q.SetFeedCategory(ctx, sqlc.SetFeedCategoryParams{FeedID: feedID, CategoryID: id, CreatedAt: now}); err != nil {
				return err
			}
		}
		if !applyExisting {
			return nil
		}
		if err := q.RemoveInheritedMemberships(ctx, feedID); err != nil {
			return err
		}
		return q.ApplyFeedCategories(ctx, sqlc.ApplyFeedCategoriesParams{FeedID: feedID, CreatedAt: now, UpdatedAt: now})
	})
}

func (s *Store) ArticleText(ctx context.Context, id string) (string, error) {
	article, err := s.queries.GetArticle(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return article.ContentText, err
}
