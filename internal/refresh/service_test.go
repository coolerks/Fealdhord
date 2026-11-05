package refresh

import (
	"context"
	"errors"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fetchFunc func(context.Context, string, netpolicy.Request) (netpolicy.Response, error)

func (f fetchFunc) Fetch(ctx context.Context, url string, request netpolicy.Request) (netpolicy.Response, error) {
	return f(ctx, url, request)
}
func refreshStore(t *testing.T) *database.Store {
	t.Helper()
	s, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func addFeed(t *testing.T, s *database.Store, url string) string {
	t.Helper()
	id, err := s.EnsureFeed(context.Background(), url, "测试源")
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func await(t *testing.T, s *Service, id string) Snapshot {
	t.Helper()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		result := s.Get(context.Background(), OperationRequest{OperationID: id})
		if result.Data.Status != "running" && result.Data.Status != "cancelling" {
			return *result.Data
		}
		select {
		case <-deadline.C:
			t.Fatal("操作未结束")
		case <-ticker.C:
		}
	}
}
func TestMergedRefresh304AndBadValidator(t *testing.T) {
	store := refreshStore(t)
	id := addFeed(t, store, "https://example.test/feed")
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
	f := fetchFunc(func(ctx context.Context, url string, request netpolicy.Request) (netpolicy.Response, error) {
		switch calls.Add(1) {
		case 1:
			close(entered)
			<-release
			return netpolicy.Response{URL: url, Status: 200, Header: http.Header{"Etag": []string{"v1"}}, Body: []byte(`<rss version="2.0"><channel><title>T</title><description>T</description><link>https://example.test/</link><item><guid>one</guid><title>正文</title></item></channel></rss>`)}, nil
		case 2:
			if request.ETag != "v1" {
				t.Error("未发送条件请求")
			}
			return netpolicy.Response{URL: url, Status: 304}, nil
		default:
			return netpolicy.Response{URL: url, Status: 200, Header: http.Header{"Etag": []string{"bad"}}, Body: []byte("malformed")}, nil
		}
	})
	service := NewService(context.Background(), store, f, nil, nil)
	defer service.Close()
	first := service.Start(context.Background(), StartRequest{FeedIDs: []string{id}})
	<-entered
	second := service.Start(context.Background(), StartRequest{FeedIDs: []string{id}})
	if first.Data.OperationID != second.Data.OperationID {
		t.Fatal("同源并发未合并")
	}
	close(release)
	if result := await(t, service, first.Data.OperationID); result.Status != "completed" || calls.Load() != 1 {
		t.Fatalf("刷新失败 %+v", result)
	}
	rows, _ := store.ListArticles(context.Background(), 10)
	before := rows[0].ID
	for i := 0; i < 2; i++ {
		next := service.Start(context.Background(), StartRequest{FeedIDs: []string{id}})
		result := await(t, service, next.Data.OperationID)
		if i == 0 && result.Status != "completed" || i == 1 && result.Status != "partial" {
			t.Fatalf("终态错误 %+v", result)
		}
	}
	row, _ := store.GetSubscription(context.Background(), id)
	rows, _ = store.ListArticles(context.Background(), 10)
	if row.ETag != "v1" || len(rows) != 1 || rows[0].ID != before || row.LastErrorCode != "FeedParseError" {
		t.Fatal("失败覆盖旧数据或 validator")
	}
}
func TestRateLimitPersistsAndCancel(t *testing.T) {
	store := refreshStore(t)
	ctx := context.Background()
	id := addFeed(t, store, "https://limited.test/feed")
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	service := NewService(ctx, store, fetchFunc(func(context.Context, string, netpolicy.Request) (netpolicy.Response, error) {
		calls.Add(1)
		return netpolicy.Response{Status: 429, Header: http.Header{"Retry-After": []string{"7200"}}}, nil
	}), nil, nil)
	service.now = func() time.Time { return now }
	defer service.Close()
	operation := service.Start(ctx, StartRequest{FeedIDs: []string{id}})
	await(t, service, operation.Data.OperationID)
	row, _ := store.GetSubscription(ctx, id)
	if row.LastErrorCode != "RateLimitError" || row.NextCheckAt == nil || *row.NextCheckAt < now.Add(2*time.Hour).UnixMilli() {
		t.Fatal("限流未持久化")
	}
	next := service.Start(ctx, StartRequest{FeedIDs: []string{id}})
	await(t, service, next.Data.OperationID)
	if calls.Load() != 1 {
		t.Fatal("手动刷新绕过 Retry-After")
	}
	other := addFeed(t, store, "https://cancel.test/feed")
	entered := make(chan struct{})
	var once sync.Once
	cancelService := NewService(ctx, store, fetchFunc(func(ctx context.Context, _ string, _ netpolicy.Request) (netpolicy.Response, error) {
		once.Do(func() { close(entered) })
		<-ctx.Done()
		return netpolicy.Response{}, ctx.Err()
	}), nil, nil)
	defer cancelService.Close()
	started := cancelService.Start(ctx, StartRequest{FeedIDs: []string{other}})
	<-entered
	cancelService.Cancel(ctx, OperationRequest{OperationID: started.Data.OperationID})
	if done := await(t, cancelService, started.Data.OperationID); done.Status != "cancelled" {
		t.Fatal(done)
	}
	if _, err := store.GetSubscription(ctx, other); errors.Is(err, context.Canceled) {
		t.Fatal("取消污染仓储")
	}
}

func TestBoundedGlobalAndPerHostConcurrency(t *testing.T) {
	store := refreshStore(t)
	ids := []string{}
	for _, address := range []string{"https://same.test/a", "https://same.test/b", "https://same.test/c", "https://other.test/a", "https://third.test/a", "https://fourth.test/a"} {
		ids = append(ids, addFeed(t, store, address))
	}
	var mu sync.Mutex
	active, hostActive, maxActive, maxHost := 0, 0, 0, 0
	entered := make(chan struct{}, 6)
	release := make(chan struct{})
	service := NewService(context.Background(), store, fetchFunc(func(ctx context.Context, address string, _ netpolicy.Request) (netpolicy.Response, error) {
		same := strings.Contains(address, "same.test")
		mu.Lock()
		active++
		if same {
			hostActive++
		}
		if active > maxActive {
			maxActive = active
		}
		if hostActive > maxHost {
			maxHost = hostActive
		}
		mu.Unlock()
		if same {
			entered <- struct{}{}
		}
		select {
		case <-release:
		case <-ctx.Done():
		}
		mu.Lock()
		active--
		if same {
			hostActive--
		}
		mu.Unlock()
		return netpolicy.Response{URL: address, Status: 304}, nil
	}), nil, nil)
	defer service.Close()
	started := service.Start(context.Background(), StartRequest{FeedIDs: ids})
	<-entered
	<-entered
	close(release)
	await(t, service, started.Data.OperationID)
	if maxActive > 4 || maxHost > 2 || maxHost < 2 {
		t.Fatalf("并发边界不正确 global=%d host=%d", maxActive, maxHost)
	}
}
