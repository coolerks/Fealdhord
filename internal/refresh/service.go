// Package refresh 合并手动／到期刷新，在有界任务中原子提交每个源。
package refresh

import (
	"context"
	"crypto/rand"
	"errors"
	"github.com/coolerks/Fealdhord/internal/app"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"math"
	mathrand "math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const EventName = "fealdhord:refresh"

type Repository interface {
	Subscriptions(context.Context, bool) ([]database.Subscription, error)
	DueSubscriptions(context.Context, int64) ([]database.Subscription, error)
	GetSubscription(context.Context, string) (database.Subscription, error)
	CommitFeed(context.Context, string, string, []database.ArticleInput, database.FeedCheck) ([]string, error)
	FailSubscription(context.Context, string, string, string, string, int64, int64) error
}
type StartRequest struct {
	FeedIDs []string `json:"feed_ids"`
}
type OperationRequest struct {
	OperationID string `json:"operation_id"`
}
type FeedResult struct {
	FeedID  string `json:"feed_id"`
	Status  string `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
type Snapshot struct {
	Generation  int64        `json:"generation"`
	OperationID string       `json:"operation_id"`
	Status      string       `json:"status"`
	Completed   int          `json:"completed"`
	Total       int          `json:"total"`
	Sequence    int          `json:"sequence"`
	Results     []FeedResult `json:"results"`
}
type Result struct {
	OK        bool          `json:"ok"`
	Data      *Snapshot     `json:"data,omitempty"`
	Error     *app.AppError `json:"error,omitempty"`
	RequestID string        `json:"request_id"`
}
type Event struct {
	SchemaVersion int      `json:"schema_version"`
	EventID       string   `json:"event_id"`
	OperationID   string   `json:"operation_id"`
	Sequence      int      `json:"sequence"`
	Type          string   `json:"type"`
	OccurredAt    string   `json:"occurred_at"`
	Payload       Snapshot `json:"payload"`
}
type run struct {
	snapshot      Snapshot
	ctx           context.Context
	cancel        context.CancelFunc
	ids           []string
	seen          map[string]bool
	next, workers int
}
type Service struct {
	mu                sync.Mutex
	repository        Repository
	fetcher           netpolicy.Fetcher
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	closed, scheduled bool
	active            *run
	history           map[string]*run
	order             []string
	hosts             map[string]chan struct{}
	now               func() time.Time
	jitter            func() float64
	emit              func(Event)
	changed           func()
	afterCommit       func(context.Context, string, *feed.Document, []string)
	generation        int64
}

func NewService(ctx context.Context, repository Repository, fetcher netpolicy.Fetcher, emit func(Event), changed func()) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{repository: repository, fetcher: fetcher, ctx: ctx, cancel: cancel, history: map[string]*run{}, hosts: map[string]chan struct{}{}, now: time.Now, jitter: mathrand.Float64, emit: emit, changed: changed}
}

// SetAfterCommit 仅装配时调用，在开始调度／接收请求前设置。
func (s *Service) SetAfterCommit(hook func(context.Context, string, *feed.Document, []string)) {
	s.afterCommit = hook
}
func snapshot(r *run) Snapshot {
	out := r.snapshot
	out.Results = append([]FeedResult{}, r.snapshot.Results...)
	return out
}
func success(value Snapshot) Result { return Result{OK: true, Data: &value, RequestID: rand.Text()} }
func failure(err error) Result      { return Result{Error: feed.SafeError(err), RequestID: rand.Text()} }
func (s *Service) publish(value Snapshot) {
	if s.emit != nil {
		s.emit(Event{SchemaVersion: 1, EventID: rand.Text(), OperationID: value.OperationID, Sequence: value.Sequence, Type: "refresh.progress", OccurredAt: s.now().UTC().Format(time.RFC3339Nano), Payload: value})
	}
}
func (s *Service) Start(ctx context.Context, request StartRequest) Result {
	if s.repository == nil || s.fetcher == nil || len(request.FeedIDs) > 5000 {
		return failure(database.ErrInvalidInput)
	}
	ids := request.FeedIDs
	if len(ids) == 0 {
		rows, err := s.repository.Subscriptions(ctx, false)
		if err != nil {
			return failure(err)
		}
		ids = make([]string, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
	}
	if len(ids) > 5000 {
		return failure(database.ErrInvalidInput)
	}
	for _, id := range ids {
		if id == "" || len(id) > 256 {
			return failure(database.ErrInvalidInput)
		}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return failure(context.Canceled)
	}
	r := s.active
	if r != nil && r.snapshot.Status != "running" {
		s.mu.Unlock()
		return failure(context.Canceled)
	}
	if r == nil {
		s.generation++
		runCtx, cancel := context.WithCancel(s.ctx)
		r = &run{snapshot: Snapshot{Generation: s.generation, OperationID: rand.Text(), Status: "running", Results: []FeedResult{}}, ctx: runCtx, cancel: cancel, seen: map[string]bool{}}
		s.active = r
		s.history[r.snapshot.OperationID] = r
		s.order = append(s.order, r.snapshot.OperationID)
		if len(s.order) > 32 {
			delete(s.history, s.order[0])
			s.order = s.order[1:]
		}
	}
	newIDs := []string{}
	pending := map[string]bool{}
	for _, id := range ids {
		if !r.seen[id] && !pending[id] {
			pending[id] = true
			newIDs = append(newIDs, id)
		}
	}
	if len(r.ids)+len(newIDs) > 5000 {
		s.mu.Unlock()
		return failure(database.ErrInvalidInput)
	}
	for _, id := range newIDs {
		r.seen[id] = true
	}
	r.ids = append(r.ids, newIDs...)
	r.snapshot.Total = len(r.ids)
	r.snapshot.Sequence++
	if len(r.ids) == 0 {
		r.snapshot.Status = "completed"
		r.cancel()
		s.active = nil
	}
	for r.workers < 4 && r.workers < len(r.ids)-r.snapshot.Completed {
		r.workers++
		s.wg.Add(1)
		go s.worker(r)
	}
	value := snapshot(r)
	s.mu.Unlock()
	s.publish(value)
	return success(value)
}
func (s *Service) Get(_ context.Context, request OperationRequest) Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.history[request.OperationID]
	if r == nil {
		return failure(database.ErrNotFound)
	}
	return success(snapshot(r))
}
func (s *Service) Cancel(_ context.Context, request OperationRequest) Result {
	s.mu.Lock()
	r := s.history[request.OperationID]
	if r == nil {
		s.mu.Unlock()
		return failure(database.ErrNotFound)
	}
	if r.snapshot.Status == "running" {
		r.cancel()
		r.snapshot.Status = "cancelling"
		r.snapshot.Sequence++
	}
	value := snapshot(r)
	s.mu.Unlock()
	s.publish(value)
	return success(value)
}
func (s *Service) worker(r *run) {
	defer s.wg.Done()
	for {
		s.mu.Lock()
		if r.next >= len(r.ids) {
			r.workers--
			s.mu.Unlock()
			return
		}
		id := r.ids[r.next]
		r.next++
		s.mu.Unlock()
		outcome := s.refresh(r.ctx, id)
		s.mu.Lock()
		r.snapshot.Completed++
		r.snapshot.Results = append(r.snapshot.Results, outcome)
		r.snapshot.Sequence++
		if r.snapshot.Completed == r.snapshot.Total {
			r.snapshot.Status = "completed"
			failed := false
			for _, item := range r.snapshot.Results {
				if item.Status == "failed" {
					failed = true
				}
			}
			if r.ctx.Err() != nil {
				r.snapshot.Status = "cancelled"
			} else if failed {
				r.snapshot.Status = "partial"
			}
			r.cancel()
			if s.active == r {
				s.active = nil
			}
		}
		value := snapshot(r)
		s.mu.Unlock()
		s.publish(value)
	}
}
func (s *Service) hostSlot(host string) chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.hosts[host]
	if slot == nil {
		slot = make(chan struct{}, 2)
		s.hosts[host] = slot
	}
	return slot
}
func (s *Service) refresh(ctx context.Context, id string) FeedResult {
	out := FeedResult{FeedID: id, Status: "failed"}
	if ctx.Err() != nil {
		out.Status = "cancelled"
		return out
	}
	row, err := s.repository.GetSubscription(ctx, id)
	if err != nil {
		safe := feed.SafeError(err)
		out.Code = safe.Code
		out.Message = safe.Message
		return out
	}
	if row.Archived {
		out.Status = "skipped"
		return out
	}
	now := s.now().UTC()
	if row.LastErrorCode == "RateLimitError" && row.NextCheckAt != nil && *row.NextCheckAt > now.UnixMilli() {
		out.Status = "skipped"
		out.Code = "RateLimitError"
		out.Message = "订阅源正在限流，请稍后重试。"
		return out
	}
	u, err := url.Parse(row.URL)
	if err != nil {
		out.Code = "ValidationError"
		out.Message = "订阅网址无效。"
		return out
	}
	slot := s.hostSlot(u.Hostname())
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	case <-ctx.Done():
		out.Status = "cancelled"
		return out
	}
	response, err := s.fetcher.Fetch(ctx, row.URL, netpolicy.Request{ETag: row.ETag, LastModified: row.LastModified})
	var document *feed.Document
	if err == nil && response.Status == http.StatusOK {
		document, err = feed.Parse(ctx, response.Body, response.URL)
	}
	if err == nil && response.Status != http.StatusOK && response.Status != http.StatusNotModified {
		err = errors.New("http status")
	}
	if ctx.Err() != nil {
		out.Status = "cancelled"
		return out
	}
	if err != nil {
		safe := feed.SafeError(err)
		out.Code = safe.Code
		out.Message = safe.Message
		delay := time.Duration(math.Min(math.Pow(2, float64(min(row.ErrorCount, 7)))*float64(5*time.Minute), float64(24*time.Hour)))
		delay = time.Duration(float64(delay) * (1 + s.jitter()*.2))
		if delay > 24*time.Hour {
			delay = 24 * time.Hour
		}
		if response.Status == 429 {
			out.Code = "RateLimitError"
			out.Message = "订阅源暂时限流，请在重试时间后刷新。"
			if retry := retryAfter(response.Header.Get("Retry-After"), now); retry > delay {
				delay = retry
			}
		}
		if saveErr := s.repository.FailSubscription(ctx, id, row.URL, out.Code, out.Message, now.UnixMilli(), now.Add(delay).UnixMilli()); saveErr != nil {
			out.Code = "DatabaseError"
			out.Message = "无法保存刷新状态，请重试。"
		}
		return out
	}
	check := database.FeedCheck{At: now.UnixMilli(), Next: now.Add(30 * time.Minute).UnixMilli(), ETag: response.Header.Get("ETag"), LastModified: response.Header.Get("Last-Modified"), NotModified: response.Status == 304}
	var items []database.ArticleInput
	if document != nil {
		items = feed.ArticleInputs(document)
	}
	ids, err := s.repository.CommitFeed(ctx, id, row.URL, items, check)
	if err != nil {
		safe := feed.SafeError(err)
		out.Code = safe.Code
		out.Message = safe.Message
		if errors.Is(err, context.Canceled) {
			out.Status = "cancelled"
		}
		return out
	}
	out.Status = "completed"
	if s.changed != nil {
		s.changed()
	}
	if document != nil && s.afterCommit != nil {
		s.afterCommit(s.ctx, id, document, ids)
	}
	return out
}
func retryAfter(raw string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil && seconds >= 0 {
		if seconds > 7*24*3600 {
			seconds = 7 * 24 * 3600
		}
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(raw); err == nil && at.After(now) {
		delay := at.Sub(now)
		if delay > 7*24*time.Hour {
			delay = 7 * 24 * time.Hour
		}
		return delay
	}
	return 0
}
func (s *Service) StartScheduler() {
	s.mu.Lock()
	if s.closed || s.scheduled {
		s.mu.Unlock()
		return
	}
	s.scheduled = true
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		s.refreshDue()
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.C:
				s.refreshDue()
			}
		}
	}()
}
func (s *Service) refreshDue() {
	if s.repository == nil {
		return
	}
	rows, err := s.repository.DueSubscriptions(s.ctx, s.now().UTC().UnixMilli())
	if err != nil {
		return
	}
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	if len(ids) > 0 {
		s.Start(s.ctx, StartRequest{FeedIDs: ids})
	}
}
func (s *Service) Close() { s.mu.Lock(); s.closed = true; s.cancel(); s.mu.Unlock(); s.wg.Wait() }
