package media

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"image/png"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
)

func TestCopyInlineImageValidatesPolicyAndWritesPNG(t *testing.T) {
	store, article, _ := fixture(t)
	id := registerInline(t, store, article)
	var source bytes.Buffer
	if err := jpeg.Encode(&source, image.NewRGBA(image.Rect(0, 0, 64, 80)), nil); err != nil {
		t.Fatal(err)
	}
	var policy atomic.Value
	policy.Store(Policy{RemoteImages: "subscriptions", CacheEnabled: true})
	var fetches atomic.Int32
	service, err := New(context.Background(), store, fetchFunc(func(_ context.Context, address string, _ netpolicy.Request) (netpolicy.Response, error) {
		fetches.Add(1)
		return netpolicy.Response{URL: address, Status: 200, Body: source.Bytes()}, nil
	}), t.TempDir(), func(context.Context) Policy { return policy.Load().(Policy) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	var writes int
	api := NewAPI(service, func(data []byte) error {
		writes++
		image, err := png.Decode(bytes.NewReader(data))
		if err != nil || image.Bounds().Dx() != 64 || image.Bounds().Dy() != 80 {
			t.Fatalf("剪贴板未收到预期 PNG：%v", err)
		}
		return nil
	})
	ctx := context.Background()
	if result := api.CopyInlineImage(ctx, id); !result.OK || writes != 1 {
		t.Fatalf("登记图片无法复制：%+v", result)
	}
	for _, invalid := range []string{strings.Repeat("b", 64), "https://example.test/inline.png"} {
		if result := api.CopyInlineImage(ctx, invalid); result.OK {
			t.Fatalf("非图片资源被复制：%+v", result)
		}
	}
	policy.Store(Policy{RemoteImages: "never", CacheEnabled: true})
	if result := api.CopyInlineImage(ctx, id); result.OK || writes != 1 || fetches.Load() != 1 {
		t.Fatalf("复制绕过媒体策略或读取了外部地址：%+v", result)
	}
}

func registerInline(t *testing.T, store *database.Store, articleID string) string {
	t.Helper()
	article, err := store.ReaderArticle(context.Background(), articleID)
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 64)
	err = store.RegisterReaderResources(context.Background(), articleID, article.Hash, "feed", []database.ReaderResource{{ID: id, Kind: "image", URL: "https://example.test/inline.png"}, {ID: strings.Repeat("b", 64), Kind: "link", URL: "https://example.test/link"}})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func TestInlineRegisteredOnlyCacheAndArchivePolicy(t *testing.T) {
	store, article, picture := fixture(t)
	id := registerInline(t, store, article)
	var calls atomic.Int32
	var policy atomic.Value
	policy.Store(Policy{RemoteImages: "subscriptions", CacheEnabled: true})
	service, err := New(context.Background(), store, fetchFunc(func(_ context.Context, url string, _ netpolicy.Request) (netpolicy.Response, error) {
		calls.Add(1)
		return netpolicy.Response{URL: url, Status: 200, Body: picture}, nil
	}), t.TempDir(), func(context.Context) Policy { return policy.Load().(Policy) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	request := func(id string) int {
		out := httptest.NewRecorder()
		service.ServeHTTP(out, httptest.NewRequest("GET", "/media/inline/"+id, nil))
		return out.Code
	}
	if request(id) != 200 || request(id) != 200 || calls.Load() != 1 {
		t.Fatal("登记图片或缓存不可用", calls.Load())
	}
	for _, bad := range []string{strings.Repeat("b", 64), strings.Repeat("c", 64), "https://example.test/arbitrary"} {
		if request(bad) != 404 {
			t.Fatal("正文媒体变成任意代理", bad)
		}
	}
	record, _ := store.ReaderArticle(context.Background(), article)
	sub, _ := store.GetSubscription(context.Background(), func() string { media, _ := store.GetMedia(context.Background(), article); return media.FeedID }())
	if err = store.ArchiveFeed(context.Background(), sub.ID, sub.Revision); err != nil {
		t.Fatal(err)
	}
	if request(id) != 404 || calls.Load() != 1 {
		t.Fatal("归档源仍绕过订阅策略")
	}
	policy.Store(Policy{RemoteImages: "always", CacheEnabled: true})
	if request(id) != 200 {
		t.Fatal("Always 应允许已登记归档图")
	}
	if _, err = store.UpsertArticle(context.Background(), database.ArticleInput{FeedID: sub.ID, GUID: "one", Title: "更新", Text: record.Text + "更新"}); err != nil {
		t.Fatal(err)
	}
	if request(id) != 404 {
		t.Fatal("陈旧文章媒体未失效")
	}
}
func TestInlineNeverCancelsInFlight(t *testing.T) {
	store, article, _ := fixture(t)
	id := registerInline(t, store, article)
	var policy atomic.Value
	policy.Store(Policy{RemoteImages: "subscriptions", CacheEnabled: true})
	entered := make(chan struct{})
	var calls atomic.Int32
	service, err := New(context.Background(), store, fetchFunc(func(ctx context.Context, _ string, _ netpolicy.Request) (netpolicy.Response, error) {
		calls.Add(1)
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
		service.ServeHTTP(out, httptest.NewRequest("GET", "/media/inline/"+id, nil))
		done <- out.Code
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("请求未进入")
	}
	never := Policy{RemoteImages: "never", CacheEnabled: true}
	policy.Store(never)
	if err = service.PolicyChanged(context.Background(), never); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != 404 {
			t.Fatal(code)
		}
	case <-time.After(time.Second):
		t.Fatal("禁图未取消在途请求")
	}
	out := httptest.NewRecorder()
	service.ServeHTTP(out, httptest.NewRequest("GET", "/media/inline/"+id, nil))
	if out.Code != 404 || calls.Load() != 1 {
		t.Fatal("Never 发出新图片请求")
	}
}
