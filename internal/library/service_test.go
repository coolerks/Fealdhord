package library

import (
	"context"
	"encoding/json"
	"github.com/coolerks/Fealdhord/internal/database"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundedMetadataDetailAndPersistentState(t *testing.T) {
	ctx := context.Background()
	file := filepath.Join(t.TempDir(), "library.db")
	store, err := database.Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	feed, err := store.EnsureFeed(ctx, "https://example.test/rss", "测试订阅")
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Repeat("完整正文不可截断。", 800)
	id, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: feed, GUID: "1", Title: "第一篇", Text: body, FetchedAt: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: feed, GUID: "2", Title: "第二篇", Text: body, FetchedAt: 200}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	page := service.List(ctx, ListRequest{Limit: 1})
	if !page.OK || len(page.Data.Items) != 1 || !page.Data.HasMore || page.RequestID == "" {
		t.Fatalf("有界读取错误：%+v", page)
	}
	encoded, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "完整正文") {
		t.Fatal("列表夹带完整正文")
	}
	detail := service.Get(ctx, IDRequest{ArticleID: id})
	if !detail.OK || detail.Data.ContentText != body {
		t.Fatal("按需正文丢失")
	}
	if got := service.SetRead(ctx, StateRequest{ArticleID: id, Value: true}); !got.OK {
		t.Fatal(got)
	}
	if got := service.SetFavorite(ctx, StateRequest{ArticleID: id, Value: true}); !got.OK {
		t.Fatal(got)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(ctx, file)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restored := NewService(reopened).List(ctx, ListRequest{})
	if !restored.OK {
		t.Fatal(restored)
	}
	for _, article := range restored.Data.Items {
		if article.ID == id && (!article.IsRead || !article.IsFavorite) {
			t.Fatal("重开丢失领域状态")
		}
	}
}
func TestBoundaryErrorsAreSafe(t *testing.T) {
	ctx := context.Background()
	if got := NewService(nil).List(ctx, ListRequest{}); got.OK || got.Data != nil || got.Error.Code != "DatabaseError" {
		t.Fatal("不可用仓储被当空库")
	}
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "library.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := NewService(store)
	if got := service.List(ctx, ListRequest{Limit: 501}); got.OK || got.Error.Code != "ValidationError" {
		t.Fatal("超限读取未拒绝")
	}
	if got := service.Get(ctx, IDRequest{ArticleID: "missing"}); got.OK || got.Error.Code != "NotFound" {
		t.Fatal("缺失文章错误")
	}
	if got := service.SetFavorite(ctx, StateRequest{ArticleID: "missing", Value: true}); got.OK || got.Error.Code != "NotFound" {
		t.Fatal("无效状态目标未拒绝")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got := service.List(cancelled, ListRequest{}); got.OK || got.Error.Code != "Cancelled" {
		t.Fatalf("取消语义丢失：%+v", got)
	}
}
