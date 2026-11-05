// Package media 探测已登记图片并通过本地 ID 提供受控加载。
package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	_ "golang.org/x/image/webp"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Policy struct {
	WebFallback  bool
	RemoteImages string
	CacheEnabled bool
}
type Repository interface {
	ArticleIDsForInputs(context.Context, string, []database.ArticleInput) ([]string, error)
	RegisterArticleMedia(context.Context, string, database.ArticleInput, []database.MediaCandidate) (bool, error)
	GetMedia(context.Context, string) (database.MediaRecord, error)
	SetMedia(context.Context, database.MediaRecord) error
	BlockMedia(context.Context) error
	PendingMedia(context.Context) ([]string, error)
}

var errUnavailable = errors.New("图片不可用")

type flight struct {
	done   chan struct{}
	cancel context.CancelFunc
	body   []byte
	mime   string
	err    error
}
type Service struct {
	repository Repository
	fetcher    netpolicy.Fetcher
	policy     func(context.Context) Policy
	cache      *cache
	changed    func()
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	closed     bool
	active     map[string]*flight
	queue      chan string
	slots      chan struct{}
	wg         sync.WaitGroup
	now        func() time.Time
}

func New(ctx context.Context, repository Repository, fetcher netpolicy.Fetcher, directory string, policy func(context.Context) Policy, changed func()) (*Service, error) {
	storage, err := newCache(directory)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Service{repository: repository, fetcher: fetcher, policy: policy, cache: storage, changed: changed, ctx: ctx, cancel: cancel, active: map[string]*flight{}, queue: make(chan string, 64), slots: make(chan struct{}, 2), now: time.Now}
	for range 2 {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case id := <-s.queue:
					if ctx.Err() == nil {
						s.load(ctx, id)
					}
				}
			}
		}()
	}
	return s, nil
}
func (s *Service) allowed(ctx context.Context, record database.MediaRecord) bool {
	if s.policy == nil {
		return false
	}
	policy := s.policy(ctx)
	if policy.RemoteImages == "never" {
		return false
	}
	current, err := s.repository.GetMedia(ctx, record.ArticleID)
	if err != nil || current.Generation != record.Generation {
		return false
	}
	return policy.RemoteImages == "always" || (policy.RemoteImages == "subscriptions" && !current.Archived)
}
func (s *Service) Enqueue(id string) {
	select {
	case <-s.ctx.Done():
		return
	case s.queue <- id:
	default:
	}
}

// Ingest 在源提交后保存候选，HTTP 探测始终由队列或按需加载执行。
func (s *Service) Ingest(ctx context.Context, feedID string, document *feed.Document, ids []string) {
	if s.repository == nil {
		return
	}
	inputs := feed.ArticleInputs(document)
	if ids == nil {
		var err error
		ids, err = s.repository.ArticleIDsForInputs(ctx, feedID, inputs)
		if err != nil {
			return
		}
	}
	for index, item := range document.Items {
		if index >= len(ids) || ids[index] == "" {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		candidates := make([]database.MediaCandidate, 0, len(item.Images))
		for _, candidate := range item.Images {
			candidates = append(candidates, database.MediaCandidate{URL: candidate.URL, Source: candidate.Source, Alt: candidate.Alt})
		}
		changed, err := s.repository.RegisterArticleMedia(ctx, ids[index], inputs[index], candidates)
		if err != nil {
			continue
		}
		if !changed {
			record, err := s.repository.GetMedia(ctx, ids[index])
			if err != nil || record.Status == "ready" {
				continue
			}
		}
		s.Enqueue(ids[index])
	}
}

// 原始图片直接交给 WebView 前，拒绝 PNG／WebP 动画块，避免解码器只校验首帧。
func animatedImage(body []byte, kind string) bool {
	offset := 8
	if kind == "webp" {
		offset = 12
	}
	if kind != "png" && kind != "webp" {
		return false
	}
	for offset+8 <= len(body) {
		var size uint32
		var tag string
		overhead := 8
		if kind == "png" {
			size = binary.BigEndian.Uint32(body[offset : offset+4])
			tag = string(body[offset+4 : offset+8])
			overhead = 12
		} else {
			tag = string(body[offset : offset+4])
			size = binary.LittleEndian.Uint32(body[offset+4 : offset+8])
		}
		if tag == "acTL" || tag == "ANIM" || tag == "ANMF" {
			return true
		}
		advance := uint64(size) + uint64(overhead)
		if kind == "webp" {
			advance += uint64(size % 2)
		}
		if advance > uint64(len(body)-offset) {
			break
		}
		offset += int(advance)
	}
	return false
}
func imageHeader(body []byte) (image.Config, string, error) {
	config, kind, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return config, "", err
	}
	if config.Width < 64 || config.Height < 64 || int64(config.Width) > 40000000/int64(config.Height) {
		return config, "", errUnavailable
	}
	if animatedImage(body, kind) || (kind != "png" && kind != "jpeg" && kind != "webp") {
		return config, "", errUnavailable
	}
	return config, kind, nil
}
func imageInfo(body []byte) (image.Config, string, error) {
	config, kind, err := imageHeader(body)
	if err != nil {
		return config, "", err
	}
	decoded, format, err := image.Decode(bytes.NewReader(body))
	if err != nil || format != kind || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return config, "", errUnavailable
	}
	return config, "image/" + kind, nil
}
func diskAllowed(policy Policy, raw string, headers http.Header) bool {
	if !policy.CacheEnabled {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	for key := range u.Query() {
		key = strings.ToLower(key)
		if strings.Contains(key, "token") || strings.Contains(key, "signature") || strings.Contains(key, "password") || key == "key" || key == "sig" || key == "auth" {
			return false
		}
	}
	control := strings.ToLower(headers.Get("Cache-Control"))
	return !strings.Contains(control, "no-store") && !strings.Contains(control, "private") && headers.Get("Set-Cookie") == ""
}
func (s *Service) load(ctx context.Context, id string) ([]byte, string, error) {
	if s.repository == nil || s.fetcher == nil {
		return nil, "", errUnavailable
	}
	record, err := s.repository.GetMedia(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if !s.allowed(ctx, record) {
		record.Status = "blocked"
		if err := s.repository.SetMedia(ctx, record); err != nil {
			return nil, "", err
		}
		return nil, "", errUnavailable
	}
	policy := s.policy(ctx)
	if record.Status == "ready" && record.CheckedAt != nil && s.now().UnixMilli()-*record.CheckedAt < int64(ttl/time.Millisecond) {
		if body, ok := s.cache.get(record.CacheKey, policy.CacheEnabled); ok {
			return body, record.MIME, nil
		}
	}
	if record.Status == "failed" && record.RetryAfter != nil && *record.RetryAfter > s.now().UnixMilli() {
		return nil, "", errUnavailable
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, "", context.Canceled
	}
	if previous := s.active[id]; previous != nil {
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-previous.done:
			return previous.body, previous.mime, previous.err
		}
	}
	requestCtx, cancel := context.WithCancel(ctx)
	call := &flight{done: make(chan struct{}), cancel: cancel}
	s.active[id] = call
	s.wg.Add(1)
	s.mu.Unlock()
	defer func() {
		cancel()
		s.mu.Lock()
		delete(s.active, id)
		close(call.done)
		s.mu.Unlock()
		s.wg.Done()
	}()
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
		call.body, call.mime, call.err = s.probe(requestCtx, record)
	case <-requestCtx.Done():
		call.err = requestCtx.Err()
	}
	return call.body, call.mime, call.err
}
func (s *Service) probe(ctx context.Context, record database.MediaRecord) ([]byte, string, error) {
	candidates := record.Candidates
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	pageTried := false
	for index := 0; index < 5; index++ {
		if index >= len(candidates) {
			if pageTried || !s.policy(ctx).WebFallback || record.ArticleURL == "" || !s.allowed(ctx, record) {
				break
			}
			pageTried = true
			response, err := s.fetcher.Fetch(ctx, record.ArticleURL, netpolicy.Request{MaxBytes: 5 << 20, Timeout: 20 * time.Second, Accept: "text/html"})
			if err != nil || response.Status != 200 {
				break
			}
			for _, item := range feed.PageImages(response.Body, response.URL) {
				candidates = append(candidates, database.MediaCandidate{URL: item.URL, Source: item.Source, Alt: item.Alt})
			}
			if index >= len(candidates) {
				break
			}
		}
		candidate := candidates[index]
		if ctx.Err() != nil {
			return nil, "", ctx.Err()
		}
		if !s.allowed(ctx, record) {
			return nil, "", errUnavailable
		}
		key := cacheKey(candidate.URL)
		policy := s.policy(ctx)
		body, ok := s.cache.get(key, policy.CacheEnabled)
		headers := http.Header{}
		finalURL := candidate.URL
		if !ok {
			response, err := s.fetcher.Fetch(ctx, candidate.URL, netpolicy.Request{MaxBytes: netpolicy.ImageLimit, Timeout: 8 * time.Second, Accept: "image/png,image/jpeg,image/webp"})
			if err != nil || response.Status != 200 {
				continue
			}
			body = response.Body
			headers = response.Header
			finalURL = response.URL
		}
		if len(body) > int(netpolicy.ImageLimit) {
			continue
		}
		config, mime, err := imageInfo(body)
		if err != nil {
			continue
		}
		if ctx.Err() != nil || !s.allowed(ctx, record) {
			return nil, "", errUnavailable
		}
		if !ok && diskAllowed(Policy{CacheEnabled: true}, candidate.URL, headers) && diskAllowed(Policy{CacheEnabled: true}, finalURL, headers) {
			if err := s.cache.put(key, body, diskAllowed(s.policy(ctx), candidate.URL, headers)); err != nil {
				continue
			}
		}
		record.Status = "ready"
		record.URL = "/media/" + record.ArticleID + "?v=" + strconv.FormatInt(record.Generation, 10)
		record.RemoteURL = finalURL
		record.Source = candidate.Source
		record.Alt = candidate.Alt
		record.Width = int64(config.Width)
		record.Height = int64(config.Height)
		record.CacheKey = key
		record.MIME = mime
		at := s.now().UTC().UnixMilli()
		record.CheckedAt = &at
		record.RetryAfter = nil
		if err := s.repository.SetMedia(ctx, record); err != nil {
			return nil, "", err
		}
		if s.changed != nil {
			s.changed()
		}
		return body, mime, nil
	}
	record.Status = "failed"
	if len(candidates) == 0 {
		record.Status = "none"
	}
	at := s.now().UTC().UnixMilli()
	retry := at + int64(time.Hour/time.Millisecond)
	record.CheckedAt = &at
	record.RetryAfter = &retry
	if err := s.repository.SetMedia(ctx, record); err != nil {
		return nil, "", err
	}
	return nil, "", errUnavailable
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/media/inline/") {
		s.serveInline(w, r)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/media/")
	if id == r.URL.Path || len(id) == 0 || len(id) > 128 {
		http.NotFound(w, r)
		return
	}
	for _, char := range id {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
			http.NotFound(w, r)
			return
		}
	}
	body, mime, err := s.load(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	record, err := s.repository.GetMedia(r.Context(), id)
	if err != nil || !s.allowed(r.Context(), record) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == "GET" {
		w.Write(body)
	}
}
func (s *Service) PolicyChanged(ctx context.Context, policy Policy) error {
	s.mu.Lock()
	for _, call := range s.active {
		call.cancel()
	}
	s.mu.Unlock()
	if !policy.CacheEnabled {
		if err := s.cache.clear(); err != nil {
			return err
		}
	}
	if policy.RemoteImages == "never" {
		return s.repository.BlockMedia(ctx)
	}
	ids, err := s.repository.PendingMedia(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.Enqueue(id)
	}
	return nil
}
func (s *Service) ClearCache() error { return s.cache.clear() }
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	s.cancel()
	for _, call := range s.active {
		call.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
}
