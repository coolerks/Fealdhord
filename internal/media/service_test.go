package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"hash/crc32"
	"image"
	"image/png"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type fetchFunc func(context.Context, string, netpolicy.Request) (netpolicy.Response, error)

func (f fetchFunc) Fetch(ctx context.Context, url string, req netpolicy.Request) (netpolicy.Response, error) {
	return f(ctx, url, req)
}
func fixture(t *testing.T) (*database.Store, string, []byte) {
	t.Helper()
	store, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	id, err := store.AddSubscription(context.Background(), database.SubscriptionInput{URL: "https://example.test/feed", Title: "订阅"}, []database.ArticleInput{{GUID: "one", Title: "文章", Text: "正文"}}, database.FeedCheck{At: 100, Next: 200})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := store.ListArticles(context.Background(), 10)
	article := rows[0].ID
	_, err = store.RegisterArticleMedia(context.Background(), article, database.ArticleInput{GUID: "one", Title: "文章", Text: "正文"}, []database.MediaCandidate{{URL: "https://example.test/bad", Source: "content"}, {URL: "https://example.test/good.png", Source: "summary"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = id
	var body bytes.Buffer
	if err = png.Encode(&body, image.NewRGBA(image.Rect(0, 0, 80, 80))); err != nil {
		t.Fatal(err)
	}
	return store, article, body.Bytes()
}
func TestFallbackLocalIDAndCache(t *testing.T) {
	store, id, picture := fixture(t)
	var calls atomic.Int32
	service, err := New(context.Background(), store, fetchFunc(func(_ context.Context, url string, _ netpolicy.Request) (netpolicy.Response, error) {
		calls.Add(1)
		body := []byte("<html>not image</html>")
		if url == "https://example.test/good.png" {
			body = picture
		}
		return netpolicy.Response{URL: url, Status: 200, Body: body}, nil
	}), t.TempDir(), func(context.Context) Policy { return Policy{RemoteImages: "subscriptions", CacheEnabled: true} }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	for range 2 {
		out := httptest.NewRecorder()
		service.ServeHTTP(out, httptest.NewRequest("GET", "/media/"+id, nil))
		if out.Code != 200 || out.Header().Get("Content-Type") != "image/png" || out.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("受控媒体失败 %+v", out)
		}
		if !bytes.Equal(out.Body.Bytes(), picture) {
			t.Fatal("媒体内容改变")
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("缓存未命中：%d", calls.Load())
	}
	record, _ := store.GetMedia(context.Background(), id)
	if record.Width != 80 || record.Status != "ready" || record.Source != "summary" {
		t.Fatal(record)
	}
	out := httptest.NewRecorder()
	service.ServeHTTP(out, httptest.NewRequest("GET", "/media/unknown?url=https://example.test/evil", nil))
	if out.Code != 404 || calls.Load() != 2 {
		t.Fatal("本地 handler 成了任意代理")
	}
}
func TestNeverAndArchivedPrivacy(t *testing.T) {
	store, id, picture := fixture(t)
	var calls atomic.Int32
	var policy atomic.Value
	policy.Store(Policy{RemoteImages: "never", CacheEnabled: false})
	service, err := New(context.Background(), store, fetchFunc(func(_ context.Context, url string, _ netpolicy.Request) (netpolicy.Response, error) {
		calls.Add(1)
		return netpolicy.Response{URL: url, Status: 200, Body: picture}, nil
	}), t.TempDir(), func(context.Context) Policy { return policy.Load().(Policy) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	out := httptest.NewRecorder()
	service.ServeHTTP(out, httptest.NewRequest("GET", "/media/"+id, nil))
	if out.Code != 404 || calls.Load() != 0 {
		t.Fatal("禁图仍请求")
	}
	record, _ := store.GetMedia(context.Background(), id)
	sub, _ := store.GetSubscription(context.Background(), record.FeedID)
	if err := store.ArchiveFeed(context.Background(), sub.ID, sub.Revision); err != nil {
		t.Fatal(err)
	}
	policy.Store(Policy{RemoteImages: "subscriptions", CacheEnabled: false})
	out = httptest.NewRecorder()
	service.ServeHTTP(out, httptest.NewRequest("GET", "/media/"+id, nil))
	if out.Code != 404 || calls.Load() != 0 {
		t.Fatal("已归档源绕过订阅图片策略")
	}
	policy.Store(Policy{RemoteImages: "always", CacheEnabled: false})
	out = httptest.NewRecorder()
	service.ServeHTTP(out, httptest.NewRequest("GET", "/media/"+id, nil))
	if out.Code != 200 || calls.Load() != 1 {
		t.Fatal("已登记图片 Always 不可用")
	}
}
func TestImagePixelGuardAndCacheTTL(t *testing.T) {
	_, _, picture := fixture(t)
	oversized := append([]byte(nil), picture...)
	binary.BigEndian.PutUint32(oversized[16:20], 10000)
	binary.BigEndian.PutUint32(oversized[20:24], 10000)
	binary.BigEndian.PutUint32(oversized[29:33], crc32.ChecksumIEEE(oversized[12:29]))
	if _, _, err := imageInfo(oversized); err == nil {
		t.Fatal("超像素图片进入解码")
	}
	if _, _, err := imageInfo([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)); err == nil {
		t.Fatal("SVG 未拒绝")
	}
	cache, err := newCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	cache.now = func() time.Time { return now }
	key := cacheKey("https://example.test/image")
	if err := cache.put(key, picture, true); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache.get(key, false); ok {
		t.Fatal("禁缓存时读取磁盘")
	}
	now = now.Add(6 * 24 * time.Hour)
	if _, ok := cache.get(key, true); !ok {
		t.Fatal("缓存提前失效")
	}
	now = now.Add(2 * 24 * time.Hour)
	if _, ok := cache.get(key, true); ok {
		t.Fatal("访问续命了 7 天 TTL")
	}
}

func TestSwitchToNeverCancelsInFlightImage(t *testing.T) {
	store, id, _ := fixture(t)
	var policy atomic.Value
	policy.Store(Policy{RemoteImages: "always", CacheEnabled: false})
	entered := make(chan struct{})
	service, err := New(context.Background(), store, fetchFunc(func(ctx context.Context, _ string, _ netpolicy.Request) (netpolicy.Response, error) {
		close(entered)
		<-ctx.Done()
		return netpolicy.Response{}, ctx.Err()
	}), t.TempDir(), func(context.Context) Policy { return policy.Load().(Policy) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	done := make(chan int, 1)
	go func() {
		out := httptest.NewRecorder()
		service.ServeHTTP(out, httptest.NewRequest("GET", "/media/"+id, nil))
		done <- out.Code
	}()
	<-entered
	policy.Store(Policy{RemoteImages: "never", CacheEnabled: false})
	if err := service.PolicyChanged(context.Background(), policy.Load().(Policy)); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 404 {
			t.Fatal("禁图后仍返回图片")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("未取消媒体请求")
	}
	record, _ := store.GetMedia(context.Background(), id)
	if record.Status != "blocked" {
		t.Fatalf("旧请求覆盖禁图状态 %+v", record)
	}
}
