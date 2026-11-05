package navigation

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/coolerks/Fealdhord/internal/database"
)

func TestServiceCASAndManualOnlyMembership(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "nav.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	changes := 0
	service := NewService(store, func() { changes++ })
	saved := service.Save(ctx, database.CategoryInput{Name: "技术", Icon: "folder"})
	if !saved.OK {
		t.Fatal(saved)
	}
	nav := service.List(ctx)
	if !nav.OK || len(nav.Data.Categories) != 1 {
		t.Fatal(nav)
	}
	category := nav.Data.Categories[0]
	conflict := service.Save(ctx, database.CategoryInput{ID: saved.Data.ID, Revision: category.Revision - 1, Name: "旧编辑"})
	if conflict.OK || conflict.Error.Code != "Conflict" {
		t.Fatal(conflict)
	}
	bad := service.Save(ctx, database.CategoryInput{ID: saved.Data.ID, Revision: category.Revision, Name: category.Name, ParentID: category.ID})
	if bad.OK || bad.Error.Code != "ValidationError" {
		t.Fatal(bad)
	}
	f, _ := store.EnsureFeed(ctx, "https://example.test/feed", "源")
	id, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: f, GUID: "one", Title: "文"})
	if err != nil {
		t.Fatal(err)
	}
	if result := service.SetMembership(ctx, MembershipRequest{ArticleID: id, CategoryID: category.ID, State: "feed"}); result.OK {
		t.Fatal("界面不能写入其他来源")
	}
	if result := service.SetMembership(ctx, MembershipRequest{ArticleID: id, CategoryID: category.ID, State: "included"}); !result.OK {
		t.Fatal(result)
	}
	members := service.Memberships(ctx, IDRequest{ID: id})
	if !members.OK || !members.Data.Items[0].Effective || members.Data.Items[0].Manual != "included" {
		t.Fatal(members)
	}
	if result := service.SetMembership(ctx, MembershipRequest{ArticleID: id, CategoryID: category.ID, State: "auto"}); !result.OK {
		t.Fatal(result)
	}
	if changes != 3 {
		t.Fatal("失败操作不应发变更事件", changes)
	}
}
