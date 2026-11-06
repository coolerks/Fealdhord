package reader

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
)

type fetchFunc func(context.Context, string, netpolicy.Request) (netpolicy.Response, error)

func (f fetchFunc) Fetch(ctx context.Context, url string, request netpolicy.Request) (netpolicy.Response, error) {
	return f(ctx, url, request)
}
func fixture(t *testing.T) (*database.Store, string, string) {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "reader.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	f, err := store.EnsureFeed(ctx, "https://example.test/feed", "测试源")
	if err != nil {
		t.Fatal(err)
	}
	body := `<h2>中文标题</h2><pre><code>fmt.Println("栈")</code></pre><table><tr><th>列</th></tr><tr><td>值</td></tr></table><p>正文<a href="/relative">链接</a></p><img src="https://images.test/picture.png" width="800" onerror="attack()"><script>attack()</script><iframe src="https://bad.test"></iframe><div style="background:url(https://bad.test)">安全文字</div>`
	id, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: f, GUID: "one", URL: "https://example.test/article", Title: "文章", HTML: body, Text: "原订阅正文"})
	if err != nil {
		t.Fatal(err)
	}
	return store, f, id
}
func TestSafeHTMLAndVersionedRegisteredResources(t *testing.T) {
	store, f, id := fixture(t)
	ctx := context.Background()
	opened := ""
	service := NewService(store, nil, func(context.Context, database.ReaderArticle) bool { return true }, func(url string) error { opened = url; return nil })
	result := service.Get(ctx, Request{ArticleID: id})
	if !result.OK {
		t.Fatal(result)
	}
	if !result.Data.HasOriginal || result.Data.OriginalURL != "https://example.test/article" {
		t.Fatal("原始网页地址与当前文章不匹配")
	}
	body := result.Data.HTML
	for _, blocked := range []string{"onerror", "attack()", "<script", "iframe", "style=", "https://"} {
		if strings.Contains(body, blocked) {
			t.Fatal("危险资源穿透", blocked, body)
		}
	}
	for _, expected := range []string{"<pre>", "<code>", "<table>", "/media/inline/", "data-reader-link=", `href="#reader-link"`} {
		if !strings.Contains(body, expected) {
			t.Fatal("排版或登记资源丢失", expected, body)
		}
	}
	imageID := regexp.MustCompile(`/media/inline/([a-f0-9]{64})`).FindStringSubmatch(body)[1]
	linkID := regexp.MustCompile(`data-reader-link="([a-f0-9]{64})"`).FindStringSubmatch(body)[1]
	image, err := store.ReaderResource(ctx, imageID)
	if err != nil || image.URL != "https://images.test/picture.png" {
		t.Fatal(image, err)
	}
	if openedResult := service.OpenLink(ctx, LinkRequest{ArticleID: id, ResourceID: linkID}); !openedResult.OK || opened != "https://example.test/relative" {
		t.Fatal(openedResult, opened)
	}
	if openedResult := service.OpenLink(ctx, LinkRequest{ArticleID: id, ResourceID: imageID}); openedResult.OK {
		t.Fatal("图片ID不能当链接打开")
	}
	if _, err = store.UpsertArticle(ctx, database.ArticleInput{FeedID: f, GUID: "one", Title: "更新", Text: "新正文"}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ReaderResource(ctx, imageID); !errors.Is(err, database.ErrNotFound) {
		t.Fatal("旧版本资源仍可加载", err)
	}
}
func TestExplicitExtractionCacheFailureAndCancellation(t *testing.T) {
	store, _, id := fixture(t)
	ctx := context.Background()
	calls := 0
	article := `<html><head><title>可读正文</title></head><body><nav>导航</nav><article><h1>可读正文</h1><p>` + strings.Repeat("这是一段中文可读内容，保留代码和表格。", 60) + `</p><pre><code>const value = 1;</code></pre><table><tr><th>键</th><th>值</th></tr><tr><td>状态</td><td>通过</td></tr></table></article></body></html>`
	fetch := fetchFunc(func(ctx context.Context, url string, req netpolicy.Request) (netpolicy.Response, error) {
		calls++
		if req.MaxBytes != 5<<20 {
			t.Fatal("提取绕过限制")
		}
		return netpolicy.Response{URL: url, Status: http.StatusOK, Body: []byte(article)}, nil
	})
	service := NewService(store, fetch, func(context.Context, database.ReaderArticle) bool { return false }, nil)
	first := service.Get(ctx, Request{ArticleID: id})
	if !first.OK || calls != 0 || first.Data.HasCached {
		t.Fatal("普通阅读不应抓原网页", first, calls)
	}
	result := service.Extract(ctx, Request{ArticleID: id})
	if !result.OK || result.Data.Mode != "reader" || !strings.Contains(result.Data.Text, "可读内容") {
		t.Fatal(result)
	}
	if !strings.Contains(result.Data.HTML, "<pre>") || !strings.Contains(result.Data.HTML, "<table>") {
		t.Fatal("提取丢失代码或表格", result.Data.HTML)
	}
	cached := service.Get(ctx, Request{ArticleID: id, Mode: "reader"})
	if !cached.OK || calls != 1 {
		t.Fatal("缓存阅读发生网络", cached, calls)
	}
	broken := NewService(store, fetchFunc(func(context.Context, string, netpolicy.Request) (netpolicy.Response, error) {
		return netpolicy.Response{Status: 500}, nil
	}), nil, nil)
	failed := broken.Extract(ctx, Request{ArticleID: id})
	if failed.OK {
		t.Fatal(failed)
	}
	preserved := service.Get(ctx, Request{ArticleID: id})
	if !preserved.OK || preserved.Data.Text != "原订阅正文" || !preserved.Data.HasCached {
		t.Fatal("失败破坏原正文或缓存", preserved)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	cancelled := service.Extract(cancelCtx, Request{ArticleID: id})
	if cancelled.OK || calls != 1 {
		t.Fatal("取消后仍抓取", cancelled, calls)
	}
}
func TestNeverImagesAndHostileHTML(t *testing.T) {
	store, _, id := fixture(t)
	service := NewService(store, nil, func(context.Context, database.ReaderArticle) bool { return false }, nil)
	result := service.Get(context.Background(), Request{ArticleID: id})
	if !result.OK || strings.Contains(result.Data.HTML, "<img") {
		t.Fatal(result)
	}
	article, _ := store.ReaderArticle(context.Background(), id)
	body := `<svg><a href="javascript:attack()">x</a></svg><img src="data:image/svg+xml,attack()"><a href="file:///secret">秘密</a><a href="https://safe.test" onclick="attack()">正常链接</a><input autofocus onfocus="attack()">`
	rendered, _, err := prepareHTML(context.Background(), article, "feed", body, article.URL, true)
	if err != nil || strings.Contains(rendered, "javascript:") || strings.Contains(rendered, "file:") || strings.Contains(rendered, "data:") || strings.Contains(rendered, "onclick") || strings.Contains(rendered, "<input") {
		t.Fatal(rendered, err)
	}
	if _, err = boundedDocument(context.Background(), strings.Repeat("<div>", 130)+"x"+strings.Repeat("</div>", 130)); err == nil {
		t.Fatal("过深HTML未拒绝")
	}
}
