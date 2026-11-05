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
	})
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
	result := NewService(nil, nil, nil, nil).Discover(context.Background(), URLRequest{URL: "https://secret:password@example.test/feed?token=hidden"})
	if result.OK || result.Error == nil || strings.Contains(result.Error.Message, "secret") || strings.Contains(result.Error.Message, "hidden") {
		t.Fatal(result)
	}
}
