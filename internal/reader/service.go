// Package reader 提供清理后的单篇正文与显式受控网页提取；不会执行远程 HTML。
package reader

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"html"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
)

const extractorVersion = "go-readability/v2.1.3"

type Repository interface {
	ReaderArticle(context.Context, string) (database.ReaderArticle, error)
	ReaderDocument(context.Context, string) (database.ReaderDocument, error)
	SaveReaderDocument(context.Context, string, database.ReaderDocument) error
	RegisterReaderResources(context.Context, string, string, string, []database.ReaderResource) error
	ReaderResource(context.Context, string) (database.ReaderResource, error)
}
type Service struct {
	repository Repository
	fetcher    netpolicy.Fetcher
	images     func(context.Context, database.ReaderArticle) bool
	open       func(string) error
	slots      chan struct{}
}

func NewService(repository Repository, fetcher netpolicy.Fetcher, images func(context.Context, database.ReaderArticle) bool, open func(string) error) *Service {
	return &Service{repository: repository, fetcher: fetcher, images: images, open: open, slots: make(chan struct{}, 2)}
}

type Request struct {
	ArticleID string `json:"article_id"`
	Mode      string `json:"mode"`
}
type LinkRequest struct {
	ArticleID  string `json:"article_id"`
	ResourceID string `json:"resource_id"`
}
type Detail struct {
	ArticleID   string `json:"article_id"`
	HTML        string `json:"html"`
	Text        string `json:"text"`
	Mode        string `json:"mode"`
	HasCached   bool   `json:"has_cached"`
	HasOriginal bool   `json:"has_original"`
}
type Result struct {
	OK        bool          `json:"ok"`
	Data      *Detail       `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type OpenResult struct {
	OK        bool          `json:"ok"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}

func safeError(err error) *app.AppError {
	if errors.Is(err, ErrContent) {
		return &app.AppError{Code: "ReaderError", Message: "无法提取可读正文，订阅正文已保留。", Retryable: true}
	}
	result := feed.DataError(err)
	if result.Code == "DatabaseError" {
		result.Message = "无法读取或缓存本地正文，请重试。"
	}
	if result.Code == "ValidationError" {
		result.Message = "正文请求参数无效。"
	}
	if result.Code == "PermissionError" {
		result.Message = "该地址不允许用于原文或正文资源请求。"
	}
	if result.Code == "NotFound" {
		result.Message = "正文尚未缓存或文章已变化。"
	}
	if result.Code == "Conflict" {
		result.Message = "文章已更新，请重新读取正文。"
	}
	return result
}
func failed(err error) Result { return Result{RequestID: rand.Text(), Error: safeError(err)} }
func (s *Service) article(ctx context.Context, id string) (database.ReaderArticle, error) {
	if s.repository == nil || id == "" || len(id) > 256 {
		return database.ReaderArticle{}, database.ErrInvalidInput
	}
	return s.repository.ReaderArticle(ctx, id)
}
func (s *Service) Get(ctx context.Context, request Request) Result {
	if request.Mode == "" {
		request.Mode = "feed"
	}
	if request.Mode != "feed" && request.Mode != "reader" {
		return failed(database.ErrInvalidInput)
	}
	article, err := s.article(ctx, request.ArticleID)
	if err != nil {
		return failed(err)
	}
	base, body, text := article.URL, article.HTML, article.Text
	if base == "" {
		base = article.FeedURL
	}
	cached, cacheErr := s.repository.ReaderDocument(ctx, article.ID)
	if cacheErr != nil && !errors.Is(cacheErr, database.ErrNotFound) {
		return failed(cacheErr)
	}
	hasCached := cacheErr == nil && cached.Version == extractorVersion
	if request.Mode == "reader" {
		if !hasCached {
			return failed(database.ErrNotFound)
		}
		body, text, base = cached.HTML, cached.Text, cached.URL
	}
	if body == "" {
		if text == "" {
			text = article.Summary
		}
		body = "<p>" + html.EscapeString(text) + "</p>"
	}
	images := s.images != nil && s.images(ctx, article)
	display, resources, err := prepareHTML(ctx, article, request.Mode, body, base, images)
	if err != nil {
		return failed(err)
	}
	if err = s.repository.RegisterReaderResources(ctx, article.ID, article.Hash, request.Mode, resources); err != nil {
		return failed(err)
	}
	originalURL, _ := netpolicy.NormalizeURL(article.URL)
	return Result{OK: true, RequestID: rand.Text(), Data: &Detail{ArticleID: article.ID, HTML: display, Text: text, Mode: request.Mode, HasCached: hasCached, HasOriginal: originalURL != ""}}
}
func (s *Service) Extract(ctx context.Context, request Request) (out Result) {
	// 解析器只操作有界字节；局部恢复第三方异常，不允许把堆栈返回界面。
	defer func() {
		if recover() != nil {
			out = failed(ErrContent)
		}
	}()
	article, err := s.article(ctx, request.ArticleID)
	if err != nil {
		return failed(err)
	}
	if article.URL == "" || s.fetcher == nil {
		return failed(ErrContent)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return failed(ctx.Err())
	}
	response, err := s.fetcher.Fetch(ctx, article.URL, netpolicy.Request{MaxBytes: 5 << 20, Timeout: 20 * time.Second, Accept: "text/html,application/xhtml+xml"})
	if err != nil {
		safe := feed.SafeError(err)
		if safe.Code == "NetworkError" {
			safe.Message = "无法读取原网页，请重试。"
		}
		if safe.Code == "PermissionError" {
			safe.Message = "该地址不允许用于原文请求。"
		}
		return Result{RequestID: rand.Text(), Error: safe}
	}
	if response.Status != 200 {
		return failed(ErrContent)
	}
	doc, err := boundedDocument(ctx, string(response.Body))
	if err != nil {
		return failed(err)
	}
	base, err := url.Parse(response.URL)
	if err != nil {
		return failed(ErrContent)
	}
	parser := readability.NewParser()
	parser.MaxElemsToParse = 100000
	parsed, err := parser.ParseAndMutate(doc, base)
	if err != nil {
		return failed(ErrContent)
	}
	if err = ctx.Err(); err != nil {
		return failed(err)
	}
	var htmlBody, textBody bytes.Buffer
	if parsed.RenderHTML(&htmlBody) != nil || parsed.RenderText(&textBody) != nil || strings.TrimSpace(textBody.String()) == "" {
		return failed(ErrContent)
	}
	safe := feed.SanitizeBody(htmlBody.String(), response.URL)
	if len(safe)+textBody.Len() > 4<<20 {
		return failed(ErrContent)
	}
	if err = ctx.Err(); err != nil {
		return failed(err)
	}
	err = s.repository.SaveReaderDocument(ctx, article.ID, database.ReaderDocument{HTML: safe, Text: textBody.String(), URL: response.URL, Hash: article.Hash, Version: extractorVersion})
	if err != nil {
		return failed(err)
	}
	return s.Get(ctx, Request{ArticleID: article.ID, Mode: "reader"})
}
func (s *Service) OpenLink(ctx context.Context, request LinkRequest) OpenResult {
	out := OpenResult{RequestID: rand.Text()}
	if request.ResourceID != "" && len(request.ResourceID) != 64 {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	article, err := s.article(ctx, request.ArticleID)
	target := article.URL
	if err == nil && request.ResourceID != "" {
		var resource database.ReaderResource
		resource, err = s.repository.ReaderResource(ctx, request.ResourceID)
		if err == nil && (resource.Kind != "link" || resource.ArticleID != article.ID) {
			err = database.ErrInvalidInput
		}
		target = resource.URL
	}
	if err == nil {
		target, err = netpolicy.NormalizeURL(target)
	}
	if err != nil {
		out.Error = safeError(err)
		return out
	}
	if s.open == nil || ctx.Err() != nil {
		out.Error = safeError(database.ErrInvalidInput)
		return out
	}
	if err = s.open(target); err != nil {
		out.Error = &app.AppError{Code: "IOError", Message: "无法在系统浏览器打开链接。", Retryable: true}
		return out
	}
	out.OK = true
	return out
}
