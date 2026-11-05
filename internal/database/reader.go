package database

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type ReaderArticle struct {
	ID, URL, HTML, Text, Summary, Hash, FeedURL string
	Archived                                    bool
}
type ReaderDocument struct {
	HTML, Text, URL, Hash, Version string
	FetchedAt                      int64
}
type ReaderResource struct {
	ID, ArticleID, ArticleHash, Mode, Kind, URL string
	Archived                                    bool
}

func (s *Store) ReaderArticle(ctx context.Context, id string) (ReaderArticle, error) {
	var out ReaderArticle
	err := s.db.QueryRowContext(ctx, `SELECT a.id,a.url,a.content_html,a.content_text,a.summary,a.content_hash,f.url,f.archived_at IS NOT NULL FROM articles a JOIN feeds f ON f.id=a.feed_id WHERE a.id=?`, id).Scan(&out.ID, &out.URL, &out.HTML, &out.Text, &out.Summary, &out.Hash, &out.FeedURL, &out.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return out, err
}
func (s *Store) ReaderDocument(ctx context.Context, id string) (ReaderDocument, error) {
	var out ReaderDocument
	err := s.db.QueryRowContext(ctx, `SELECT r.sanitized_html,r.content_text,r.source_url,r.content_hash,r.extractor_version,r.fetched_at FROM reader_contents r JOIN articles a ON a.id=r.article_id AND a.content_hash=r.content_hash WHERE r.article_id=?`, id).Scan(&out.HTML, &out.Text, &out.URL, &out.Hash, &out.Version, &out.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return out, err
}
func (s *Store) SaveReaderDocument(ctx context.Context, id string, document ReaderDocument) error {
	if len(document.HTML)+len(document.Text) > 4<<20 {
		return ErrInvalidInput
	}
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		article, err := q.GetArticle(ctx, id)
		if err != nil {
			return err
		}
		if article.ContentHash != document.Hash {
			return ErrConflict
		}
		// 同一事务沿用 q 的连接，避免单连接 Store 的事务内再取连接。
		return q.SaveReaderDocument(ctx, sqlc.SaveReaderDocumentParams{ArticleID: id, SanitizedHtml: document.HTML, ContentText: document.Text, SourceUrl: document.URL, ContentHash: document.Hash, ExtractorVersion: document.Version, FetchedAt: time.Now().UTC().UnixMilli()})
	})
}
func (s *Store) RegisterReaderResources(ctx context.Context, id, hash, mode string, resources []ReaderResource) error {
	if len(resources) > 288 || (mode != "feed" && mode != "reader") {
		return ErrInvalidInput
	}
	return s.transaction(ctx, func(q *sqlc.Queries) error {
		article, err := q.GetArticle(ctx, id)
		if err != nil {
			return err
		}
		if article.ContentHash != hash {
			return ErrConflict
		}
		if err = q.RemoveReaderResources(ctx, sqlc.RemoveReaderResourcesParams{ArticleID: id, Mode: mode}); err != nil {
			return err
		}
		for _, resource := range resources {
			if (resource.Kind != "image" && resource.Kind != "link") || len(resource.ID) != 64 || resource.URL == "" || len(resource.URL) > 8192 {
				return ErrInvalidInput
			}
			if err = q.AddReaderResource(ctx, sqlc.AddReaderResourceParams{ID: resource.ID, ArticleID: id, ArticleHash: hash, Mode: mode, Kind: resource.Kind, Url: resource.URL}); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Store) ReaderResource(ctx context.Context, id string) (ReaderResource, error) {
	var out ReaderResource
	err := s.db.QueryRowContext(ctx, `SELECT r.id,r.article_id,r.article_hash,r.mode,r.kind,r.url,f.archived_at IS NOT NULL FROM reader_resources r JOIN articles a ON a.id=r.article_id AND a.content_hash=r.article_hash JOIN feeds f ON f.id=a.feed_id WHERE r.id=?`, id).Scan(&out.ID, &out.ArticleID, &out.ArticleHash, &out.Mode, &out.Kind, &out.URL, &out.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		err = ErrNotFound
	}
	return out, err
}
