package feed

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
)

type serviceFetcher struct {
	body  string
	calls int
}

func (f *serviceFetcher) Fetch(_ context.Context, url string, _ netpolicy.Request) (netpolicy.Response, error) {
	f.calls++
	return netpolicy.Response{URL: url, Status: 200, Header: http.Header{"Etag": []string{"version-one"}}, Body: []byte(f.body)}, nil
}
func TestSubscriptionServiceRealStorageLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "feed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	category, err := store.CreateCategory(ctx, "技术", "")
	if err != nil {
		t.Fatal(err)
	}
	fetcher := &serviceFetcher{body: `<rss version="2.0"><channel><title>订阅</title><link>https://example.test</link><item><guid>article</guid><title>文章</title><description>可读正文</description></item></channel></rss>`}
	changes, media := 0, 0
	service := NewService(store, fetcher, func() { changes++ }, func(ctx context.Context, id string, document *Document) {
		rows, err := store.ListArticles(ctx, 10)
		if err != nil || len(rows) != 1 || document.Items[0].Text != "可读正文" {
			t.Fatal("媒体钩子早于文章提交")
		}
		media++
	}, func(context.Context) bool { return false })
	result := service.Add(ctx, AddRequest{URL: "https://example.test/feed", Title: "自定义名称", CategoryIDs: []string{category}})
	if !result.OK {
		t.Fatal(result.Error)
	}
	added := *result.Data
	rows, err := store.ListArticles(ctx, 10)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	text, err := store.ArticleText(ctx, rows[0].ID)
	if err != nil || text != "可读正文" {
		t.Fatal(text, err)
	}
	if err := store.SetFavorite(ctx, rows[0].ID, true); err != nil {
		t.Fatal(err)
	}
	duplicate := service.Add(ctx, AddRequest{URL: added.URL, Title: "不能覆盖"})
	if !duplicate.OK || duplicate.Data.ID != added.ID || duplicate.Data.Title != "自定义名称" {
		t.Fatal(duplicate)
	}
	fetcher.body = "broken"
	failed := service.Update(ctx, UpdateRequest{ID: added.ID, Revision: added.Revision, URL: "https://example.test/bad", Title: "不应生效"})
	if failed.OK || failed.Error.Code != "FeedParseError" {
		t.Fatal(failed)
	}
	current := service.Get(ctx, IDRequest{ID: added.ID})
	if current.Data.URL != added.URL || current.Data.Title != added.Title {
		t.Fatal("坏地址破坏已有订阅")
	}
	calls := fetcher.calls
	renamed := service.Update(ctx, UpdateRequest{ID: added.ID, Revision: added.Revision, URL: added.URL, Title: "重命名", CategoryIDs: []string{category}})
	if !renamed.OK || fetcher.calls != calls {
		t.Fatal("仅改名不应请求网络", renamed)
	}
	conflict := service.Update(ctx, UpdateRequest{ID: added.ID, Revision: added.Revision, URL: "https://example.test/other", Title: "旧编辑"})
	if conflict.OK || conflict.Error.Code != "Conflict" || fetcher.calls != calls {
		t.Fatal(conflict)
	}
	archived := service.Remove(ctx, RevisionRequest{ID: added.ID, Revision: renamed.Data.Revision})
	if !archived.OK || !archived.Data.Archived {
		t.Fatal(archived)
	}
	if page := service.List(ctx, ListRequest{}); !page.OK || len(page.Data.Items) != 0 {
		t.Fatal(page)
	}
	restored := service.Restore(ctx, RevisionRequest{ID: added.ID, Revision: archived.Data.Revision})
	if !restored.OK || restored.Data.Archived {
		t.Fatal(restored)
	}
	rows, _ = store.ListArticles(ctx, 10)
	if len(rows) != 1 || rows[0].IsFavorite != 1 || rows[0].ID == "" || changes != 5 || media != 2 {
		t.Fatal("生命周期丢失状态或提交通知", rows, changes, media)
	}
}
func TestSubscriptionErrorDoesNotLeakURLCredentials(t *testing.T) {
	result := NewService(nil, nil, nil, nil, func(context.Context) bool { return false }).Discover(context.Background(), URLRequest{URL: "https://secret:password@example.test/feed?token=hidden"})
	if result.OK || result.Error == nil || strings.Contains(result.Error.Message, "secret") || strings.Contains(result.Error.Message, "hidden") {
		t.Fatal(result)
	}
}

type faviconFetcher struct {
	iconRequests int
	missing      bool
}

func (f *faviconFetcher) Fetch(_ context.Context, url string, options netpolicy.Request) (netpolicy.Response, error) {
	response := netpolicy.Response{URL: url, Status: http.StatusOK}
	if strings.HasSuffix(url, ".ico") {
		f.iconRequests++
		if (url != "https://site.test/favicon.ico" && url != "https://cdn.test/brand.ico") || options.MaxBytes != faviconLimit {
			return netpolicy.Response{}, netpolicy.ErrBlocked
		}
		if f.missing {
			response.Status = http.StatusNotFound
			return response, nil
		}
		// ICO with one bounded directory entry.
		response.Body = []byte{0, 0, 1, 0, 1, 0, 16, 16, 0, 0, 1, 0, 32, 0, 1, 0, 0, 0, 22, 0, 0, 0, 1}
		return response, nil
	}
	response.Body = []byte(`<rss version="2.0"><channel><title>订阅</title><link>https://site.test/blog</link></channel></rss>`)
	return response, nil
}

func TestSubscriptionFaviconChoice(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "icon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fetcher := &faviconFetcher{}
	allowed := true
	service := NewService(store, fetcher, nil, nil, func(context.Context) bool { return allowed })
	feedURL := "https://feeds.test/rss"
	preview := service.PreviewIcon(ctx, URLRequest{URL: feedURL})
	if !strings.HasPrefix(preview, "data:image/x-icon;base64,") {
		t.Fatalf("网站图标预览：%q", preview)
	}
	added := service.Add(ctx, AddRequest{URL: feedURL})
	if !added.OK || added.Data.IconData != preview {
		t.Fatalf("添加未保存预览图标：%+v", added)
	}
	nav, err := store.Navigation(ctx)
	if err != nil || len(nav.Feeds) != 1 || nav.Feeds[0].IconData != preview {
		t.Fatalf("导航未返回保存的图标：%+v，%v", nav, err)
	}
	useDefault := true
	updated := service.Update(ctx, UpdateRequest{ID: added.Data.ID, Revision: added.Data.Revision, Title: added.Data.Title, UseDefaultIcon: &useDefault})
	if !updated.OK || updated.Data.IconData != "" || fetcher.iconRequests != 2 {
		t.Fatalf("切换默认图标未清除数据或触发了额外请求：%+v，请求数 %d", updated, fetcher.iconRequests)
	}
	explicit := service.Add(ctx, AddRequest{URL: "https://feeds.test/explicit", UseDefaultIcon: true})
	if !explicit.OK || explicit.Data.IconData != "" || fetcher.iconRequests != 2 {
		t.Fatalf("手动选择默认图标仍请求网站图标：%+v，请求数 %d", explicit, fetcher.iconRequests)
	}
	fetcher.missing = true
	missing := service.Add(ctx, AddRequest{URL: "https://feeds.test/missing"})
	if !missing.OK || missing.Data.IconData != "" || fetcher.iconRequests != 3 {
		t.Fatalf("网站图标缺失影响添加：%+v，请求数 %d", missing, fetcher.iconRequests)
	}
	allowed = false
	if value := service.PreviewIcon(ctx, URLRequest{URL: feedURL}); value != "" || fetcher.iconRequests != 3 {
		t.Fatal("禁止远程图片时仍请求网站图标")
	}
}

func TestSubscriptionCustomIcon(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "icon.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fetcher := &faviconFetcher{}
	service := NewService(store, fetcher, nil, nil, func(context.Context) bool { return true })
	const custom = "https://cdn.test/brand.ico"
	added := service.Add(ctx, AddRequest{URL: "https://feeds.test/rss", CustomIconURL: custom})
	if !added.OK || added.Data.CustomIconURL != custom || added.Data.IconData == "" || fetcher.iconRequests != 1 {
		t.Fatalf("添加没有保存自定义地址和图片：%+v，请求数 %d", added, fetcher.iconRequests)
	}

	edit := UpdateRequest{ID: added.Data.ID, Revision: added.Data.Revision, Title: "新名称", URL: "https://feeds.test/new", CustomIconURL: custom}
	updated := service.Update(ctx, edit)
	if !updated.OK || updated.Data.CustomIconURL != custom || updated.Data.IconData != added.Data.IconData || fetcher.iconRequests != 1 {
		t.Fatalf("修改订阅地址时丢失或重复下载自定义图标：%+v", updated)
	}
	edit.Revision, edit.Title, edit.CustomIconURL = updated.Data.Revision, "不应保存", "https://cdn.test/missing.ico"
	if failed := service.Update(ctx, edit); failed.OK || failed.Error.Code != "IconLoadError" {
		t.Fatalf("无法加载的图标未拒绝：%+v", failed)
	}
	unchanged := service.Get(ctx, IDRequest{ID: edit.ID})
	if unchanged.Data.Revision != edit.Revision || unchanged.Data.Title != "新名称" || unchanged.Data.CustomIconURL != custom {
		t.Fatalf("失败修改影响了已保存数据：%+v", unchanged.Data)
	}

	useDefault := true
	edit.Title, edit.CustomIconURL, edit.UseDefaultIcon = "新名称", "", &useDefault
	defaultIcon := service.Update(ctx, edit)
	if !defaultIcon.OK || defaultIcon.Data.CustomIconURL != "" || defaultIcon.Data.IconData != "" {
		t.Fatalf("切换默认图标未清除自定义地址和图片：%+v", defaultIcon)
	}
}
