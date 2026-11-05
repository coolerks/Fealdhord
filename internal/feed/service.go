package feed

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"net/http"
	"strings"
	"time"
)

type Repository interface {
	Subscriptions(context.Context, bool) ([]database.Subscription, error)
	GetSubscription(context.Context, string) (database.Subscription, error)
	AddSubscription(context.Context, database.SubscriptionInput, []database.ArticleInput, database.FeedCheck) (string, error)
	UpdateSubscription(context.Context, string, int64, database.SubscriptionInput) error
	ArchiveFeed(context.Context, string, int64) error
	RestoreSubscription(context.Context, string, int64) error
	Categories(context.Context) ([]database.CategorySummary, error)
}
type Service struct {
	repository Repository
	fetcher    netpolicy.Fetcher
	now        func() time.Time
	onChange   func()
}

func NewService(repository Repository, fetcher netpolicy.Fetcher, onChange func()) *Service {
	return &Service{repository: repository, fetcher: fetcher, now: time.Now, onChange: onChange}
}

type Feed struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	URL         string   `json:"url"`
	SiteURL     string   `json:"site_url"`
	Description string   `json:"description"`
	Revision    int64    `json:"revision"`
	Archived    bool     `json:"archived"`
	LastError   string   `json:"last_error"`
	NextCheckAt *string  `json:"next_check_at,omitempty"`
	CategoryIDs []string `json:"category_ids"`
}
type Page struct {
	Items      []Feed                     `json:"items"`
	Categories []database.CategorySummary `json:"categories"`
}
type ListRequest struct {
	IncludeArchived bool `json:"include_archived"`
}
type URLRequest struct {
	URL string `json:"url"`
}
type IDRequest struct {
	ID string `json:"id"`
}
type RevisionRequest struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
}
type AddRequest struct {
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	CategoryIDs []string `json:"category_ids"`
}
type UpdateRequest struct {
	ID          string   `json:"id"`
	Revision    int64    `json:"revision"`
	URL         string   `json:"url"`
	Title       string   `json:"title"`
	CategoryIDs []string `json:"category_ids"`
}
type Result struct {
	OK        bool          `json:"ok"`
	Data      *Feed         `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type ListResult struct {
	OK        bool          `json:"ok"`
	Data      *Page         `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type DiscoverData struct {
	Candidates []Candidate `json:"candidates"`
}
type DiscoverResult struct {
	OK        bool          `json:"ok"`
	Data      *DiscoverData `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}

func SafeError(err error) *app.AppError {
	code, message, retry := "NetworkError", "无法连接订阅源，请稍后重试。", true
	switch {
	case errors.Is(err, netpolicy.ErrBlocked):
		code, message, retry = "PermissionError", "该地址不允许用于订阅或媒体请求。", false
	case errors.Is(err, netpolicy.ErrInvalidURL), errors.Is(err, database.ErrInvalidInput):
		code, message, retry = "ValidationError", "网址或订阅参数无效。", false
	case errors.Is(err, netpolicy.ErrTooLarge):
		code, message, retry = "ValidationError", "远端内容超过安全大小限制。", false
	case errors.Is(err, ErrParse):
		code, message, retry = "FeedParseError", "订阅内容无法解析，原数据已保留。", true
	case errors.Is(err, ErrNoFeed):
		code, message, retry = "NotFound", "网页没有提供可用订阅地址。", false
	case errors.Is(err, database.ErrNotFound):
		code, message, retry = "NotFound", "订阅已不存在。", false
	case errors.Is(err, database.ErrConflict), errors.Is(err, database.ErrIdentityConflict):
		code, message, retry = "Conflict", "订阅或文章身份已变化，请重新载入。", true
	case errors.Is(err, context.Canceled):
		code, message, retry = "Cancelled", "操作已取消。", false
	case errors.Is(err, context.DeadlineExceeded):
		code, message = "Timeout", "请求超时，请稍后重试。"
	}
	return &app.AppError{Code: code, Message: message, Retryable: retry}
}
func wire(row database.Subscription) Feed {
	result := Feed{ID: row.ID, Title: row.Title, URL: row.URL, SiteURL: row.SiteURL, Description: row.Description, Revision: row.Revision, Archived: row.Archived, LastError: row.LastError, CategoryIDs: row.CategoryIDs}
	if row.NextCheckAt != nil {
		value := time.UnixMilli(*row.NextCheckAt).UTC().Format(time.RFC3339Nano)
		result.NextCheckAt = &value
	}
	if result.CategoryIDs == nil {
		result.CategoryIDs = []string{}
	}
	return result
}
func (s *Service) List(ctx context.Context, request ListRequest) ListResult {
	out := ListResult{RequestID: rand.Text()}
	if s.repository == nil {
		out.Error = &app.AppError{Code: "DatabaseError", Message: "本地订阅数据库不可用。", Retryable: true}
		return out
	}
	rows, err := s.repository.Subscriptions(ctx, request.IncludeArchived)
	if err != nil {
		out.Error = SafeError(err)
		return out
	}
	page := &Page{Items: []Feed{}}
	page.Categories, err = s.repository.Categories(ctx)
	if err != nil {
		out.Error = SafeError(err)
		return out
	}
	if page.Categories == nil {
		page.Categories = []database.CategorySummary{}
	}
	for _, row := range rows {
		page.Items = append(page.Items, wire(row))
	}
	out.OK = true
	out.Data = page
	return out
}
func (s *Service) Get(ctx context.Context, request IDRequest) Result {
	out := Result{RequestID: rand.Text()}
	if s.repository == nil || request.ID == "" {
		out.Error = SafeError(database.ErrInvalidInput)
		return out
	}
	row, err := s.repository.GetSubscription(ctx, request.ID)
	if err != nil {
		out.Error = SafeError(err)
		return out
	}
	value := wire(row)
	out.OK = true
	out.Data = &value
	return out
}
func (s *Service) Discover(ctx context.Context, request URLRequest) DiscoverResult {
	out := DiscoverResult{RequestID: rand.Text()}
	if s.fetcher == nil {
		out.Error = SafeError(netpolicy.ErrInvalidURL)
		return out
	}
	candidates, err := Discover(ctx, s.fetcher, request.URL)
	if err != nil {
		out.Error = SafeError(err)
		return out
	}
	out.OK = true
	out.Data = &DiscoverData{Candidates: candidates}
	return out
}
func FetchDocument(ctx context.Context, fetcher netpolicy.Fetcher, raw string, options netpolicy.Request) (*Document, netpolicy.Response, error) {
	response, err := fetcher.Fetch(ctx, raw, options)
	if err != nil {
		return nil, response, err
	}
	if response.Status != http.StatusOK {
		return nil, response, ErrParse
	}
	document, err := Parse(ctx, response.Body, response.URL)
	return document, response, err
}
func ArticleInputs(document *Document) []database.ArticleInput {
	items := make([]database.ArticleInput, 0, len(document.Items))
	for _, item := range document.Items {
		items = append(items, database.ArticleInput{GUID: item.GUID, Title: item.Title, URL: item.URL, Author: item.Author, Summary: item.Summary, HTML: item.HTML, Text: item.Text, PublishedAt: item.PublishedAt, UpdatedAt: item.UpdatedAt})
	}
	return items
}
func (s *Service) Add(ctx context.Context, request AddRequest) Result {
	if s.repository == nil || s.fetcher == nil {
		return Result{RequestID: rand.Text(), Error: SafeError(database.ErrInvalidInput)}
	}
	document, response, err := FetchDocument(ctx, s.fetcher, request.URL, netpolicy.Request{})
	if err != nil {
		return Result{RequestID: rand.Text(), Error: SafeError(err)}
	}
	canonical, err := netpolicy.NormalizeURL(request.URL)
	if err != nil {
		return Result{RequestID: rand.Text(), Error: SafeError(err)}
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = document.Title
	}
	now := s.now().UTC().UnixMilli()
	id, err := s.repository.AddSubscription(ctx, database.SubscriptionInput{URL: canonical, Title: title, SiteURL: document.SiteURL, Description: document.Description, CategoryIDs: request.CategoryIDs}, ArticleInputs(document), database.FeedCheck{At: now, Next: now + int64(30*time.Minute/time.Millisecond), ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified")})
	if err != nil {
		return Result{RequestID: rand.Text(), Error: SafeError(err)}
	}
	if s.onChange != nil {
		s.onChange()
	}
	return s.Get(ctx, IDRequest{ID: id})
}
func (s *Service) Update(ctx context.Context, request UpdateRequest) Result {
	if s.repository == nil {
		return Result{RequestID: rand.Text(), Error: SafeError(database.ErrInvalidInput)}
	}
	old, err := s.repository.GetSubscription(ctx, request.ID)
	if err != nil {
		return Result{RequestID: rand.Text(), Error: SafeError(err)}
	}
	input := database.SubscriptionInput{URL: old.URL, Title: request.Title, SiteURL: old.SiteURL, Description: old.Description, CategoryIDs: request.CategoryIDs}
	if request.URL != "" {
		input.URL, err = netpolicy.NormalizeURL(request.URL)
		if err != nil {
			return Result{RequestID: rand.Text(), Error: SafeError(err)}
		}
	}
	if input.URL != old.URL {
		if s.fetcher == nil {
			return Result{RequestID: rand.Text(), Error: SafeError(database.ErrInvalidInput)}
		}
		document, _, err := FetchDocument(ctx, s.fetcher, input.URL, netpolicy.Request{})
		if err != nil {
			return Result{RequestID: rand.Text(), Error: SafeError(err)}
		}
		input.SiteURL = document.SiteURL
		input.Description = document.Description
	}
	if err = s.repository.UpdateSubscription(ctx, request.ID, request.Revision, input); err != nil {
		return Result{RequestID: rand.Text(), Error: SafeError(err)}
	}
	if s.onChange != nil {
		s.onChange()
	}
	return s.Get(ctx, IDRequest{ID: request.ID})
}
func (s *Service) Remove(ctx context.Context, request RevisionRequest) Result {
	return s.archive(ctx, request, false)
}
func (s *Service) Restore(ctx context.Context, request RevisionRequest) Result {
	return s.archive(ctx, request, true)
}
func (s *Service) archive(ctx context.Context, request RevisionRequest, restore bool) Result {
	if s.repository == nil {
		return Result{RequestID: rand.Text(), Error: SafeError(database.ErrInvalidInput)}
	}
	var err error
	if restore {
		err = s.repository.RestoreSubscription(ctx, request.ID, request.Revision)
	} else {
		err = s.repository.ArchiveFeed(ctx, request.ID, request.Revision)
	}
	if err != nil {
		return Result{RequestID: rand.Text(), Error: SafeError(err)}
	}
	if s.onChange != nil {
		s.onChange()
	}
	return s.Get(ctx, IDRequest{ID: request.ID})
}
