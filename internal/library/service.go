// Package library 暴露有界阅读数据，SQL 类型不越过绑定边界。
package library

import (
	"context"
	"crypto/rand"
	"errors"

	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

type Repository interface {
	ListArticles(context.Context, int64) ([]sqlc.ListArticlesRow, error)
	ArticleText(context.Context, string) (string, error)
	SetRead(context.Context, string, bool) error
	SetFavorite(context.Context, string, bool) error
}
type Cover struct {
	URL    string `json:"url"`
	Width  *int64 `json:"width,omitempty"`
	Height *int64 `json:"height,omitempty"`
	Alt    string `json:"alt"`
}
type TextRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}
type Article struct {
	FeedID       string      `json:"feed_id"`
	URL          string      `json:"url"`
	TitleMatches []TextRange `json:"title_matches"`
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	FeedTitle    string      `json:"feed_title"`
	Summary      string      `json:"summary"`
	SortAt       string      `json:"sort_at"`
	IsRead       bool        `json:"is_read"`
	IsFavorite   bool        `json:"is_favorite"`
	Cover        *Cover      `json:"cover,omitempty"`
}
type Page struct {
	NextCursor    string    `json:"next_cursor"`
	SnapshotID    string    `json:"snapshot_id"`
	SearchLimited bool      `json:"search_limited"`
	Items         []Article `json:"items"`
	HasMore       bool      `json:"has_more"`
}
type ListRequest struct {
	Limit int64 `json:"limit"`
}
type ListResult struct {
	OK        bool          `json:"ok"`
	Data      *Page         `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type IDRequest struct {
	ArticleID string `json:"article_id"`
}
type StateRequest struct {
	ArticleID string `json:"article_id"`
	Value     bool   `json:"value"`
}
type Detail struct {
	ArticleID   string `json:"article_id"`
	ContentText string `json:"content_text"`
}
type DetailResult struct {
	OK        bool          `json:"ok"`
	Data      *Detail       `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type State struct {
	ArticleID string `json:"article_id"`
	Value     bool   `json:"value"`
}
type StateResult struct {
	OK        bool          `json:"ok"`
	Data      *State        `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type Service struct {
	repository   Repository
	coverAllowed func(context.Context) bool
}

func NewService(repository Repository, allowed ...func(context.Context) bool) *Service {
	s := &Service{repository: repository}
	if len(allowed) > 0 {
		s.coverAllowed = allowed[0]
	}
	return s
}
func safeError(err error) *app.AppError {
	code, message, retry := "DatabaseError", "无法读取或更新本地文章，请重试。", true
	switch {
	case errors.Is(err, database.ErrCursorExpired):
		code, message, retry = "CursorExpired", "列表已变化，请重新载入。", true
	case errors.Is(err, database.ErrConflict):
		code, message, retry = "Conflict", "目标已更新，请重新载入。", true
	case errors.Is(err, database.ErrNotFound):
		code, message, retry = "NotFound", "文章已不存在。", false
	case errors.Is(err, database.ErrInvalidInput):
		code, message, retry = "ValidationError", "请求参数无效。", false
	case errors.Is(err, context.Canceled):
		code, message, retry = "Cancelled", "操作已取消。", false
	case errors.Is(err, context.DeadlineExceeded):
		code, message = "Timeout", "读取本地文章超时，请重试。"
	}
	return &app.AppError{Code: code, Message: message, Retryable: retry}
}
func (s *Service) List(ctx context.Context, request ListRequest) ListResult {
	result := ListResult{RequestID: rand.Text()}
	limit := request.Limit
	if limit == 0 {
		limit = 200
	}
	if limit < 1 || limit > 500 {
		result.Error = safeError(database.ErrInvalidInput)
		return result
	}
	if s.repository == nil {
		result.Error = safeError(errors.New("unavailable"))
		return result
	}
	rows, err := s.repository.ListArticles(ctx, limit+1)
	if err != nil {
		result.Error = safeError(err)
		return result
	}
	page := &Page{Items: []Article{}, HasMore: int64(len(rows)) > limit}
	if page.HasMore {
		rows = rows[:limit]
	}
	covers := s.coverAllowed == nil || s.coverAllowed(ctx)
	for _, row := range rows {
		article := projectArticle(row, covers)
		page.Items = append(page.Items, article)
	}
	result.OK = true
	result.Data = page
	return result
}
func (s *Service) Get(ctx context.Context, request IDRequest) DetailResult {
	result := DetailResult{RequestID: rand.Text()}
	if request.ArticleID == "" || len(request.ArticleID) > 256 {
		result.Error = safeError(database.ErrInvalidInput)
		return result
	}
	if s.repository == nil {
		result.Error = safeError(errors.New("unavailable"))
		return result
	}
	text, err := s.repository.ArticleText(ctx, request.ArticleID)
	if err != nil {
		result.Error = safeError(err)
		return result
	}
	result.OK = true
	result.Data = &Detail{ArticleID: request.ArticleID, ContentText: text}
	return result
}
func (s *Service) SetRead(ctx context.Context, request StateRequest) StateResult {
	return s.update(ctx, request, false)
}
func (s *Service) SetFavorite(ctx context.Context, request StateRequest) StateResult {
	return s.update(ctx, request, true)
}
func (s *Service) update(ctx context.Context, request StateRequest, favorite bool) StateResult {
	result := StateResult{RequestID: rand.Text()}
	if request.ArticleID == "" || len(request.ArticleID) > 256 {
		result.Error = safeError(database.ErrInvalidInput)
		return result
	}
	if s.repository == nil {
		result.Error = safeError(errors.New("unavailable"))
		return result
	}
	var err error
	if favorite {
		err = s.repository.SetFavorite(ctx, request.ArticleID, request.Value)
	} else {
		err = s.repository.SetRead(ctx, request.ArticleID, request.Value)
	}
	if err != nil {
		result.Error = safeError(err)
		return result
	}
	result.OK = true
	result.Data = &State{ArticleID: request.ArticleID, Value: request.Value}
	return result
}
