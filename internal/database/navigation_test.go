package database

import (
	"context"
	"errors"
	"testing"
)

func TestCategoryNavigationCountsAndManualPriority(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f, _ := s.EnsureFeed(ctx, "https://example.test/nav", "订阅")
	parent, err := s.SaveCategory(ctx, CategoryInput{Name: "父", Icon: "book", Color: "#123abc", Description: "说明"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.SaveCategory(ctx, CategoryInput{Name: "子", ParentID: parent, SortPosition: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetFeedCategories(ctx, f, []string{parent, child}, true); err != nil {
		t.Fatal(err)
	}
	id, err := s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: "one", Title: "文章"})
	if err != nil {
		t.Fatal(err)
	}
	nav, err := s.Navigation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if nav.Totals.Total != 1 || nav.Totals.Unread != 1 || len(nav.Categories) != 2 || nav.Categories[0].Total != 1 || nav.Feeds[0].Unread != 1 {
		t.Fatal(nav)
	}
	if err = s.SetMembership(ctx, id, parent, "manual", "excluded", nil); err != nil {
		t.Fatal(err)
	}
	if err = s.SetRead(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if err = s.SetFavorite(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	nav, _ = s.Navigation(ctx)
	if nav.Totals.Total != 1 || nav.Totals.Favorites != 1 || nav.Totals.Unread != 0 || nav.Categories[0].Total != 0 {
		t.Fatal(nav)
	}
	if err = s.ResetManualMembership(ctx, id, parent); err != nil {
		t.Fatal(err)
	}
	states, err := s.ArticleMemberships(ctx, id)
	if err != nil || !states[0].Effective || states[0].Manual != "auto" {
		t.Fatal(states, err)
	}
	info := nav.Categories[0]
	if _, err = s.SaveCategory(ctx, CategoryInput{ID: parent, Revision: info.Revision - 1, Name: "旧编辑"}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err = s.SaveCategory(ctx, CategoryInput{ID: parent, Revision: info.Revision, Name: info.Name, ParentID: child}); err == nil {
		t.Fatal("未阻止环")
	}
	preview, err := s.PreviewCategoryDeletion(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Children != 1 || preview.Articles != 1 || preview.Feeds != 1 {
		t.Fatal(preview)
	}
	if _, err = s.SaveCategory(ctx, CategoryInput{Name: "新子", ParentID: parent}); err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteCategoryChecked(ctx, preview); !errors.Is(err, ErrConflict) {
		t.Fatal("过期删除预览被执行", err)
	}
	preview, _ = s.PreviewCategoryDeletion(ctx, parent)
	if err = s.DeleteCategoryChecked(ctx, preview); err != nil {
		t.Fatal(err)
	}
	nav, _ = s.Navigation(ctx)
	if nav.Totals.Total != 1 || nav.Totals.Favorites != 1 {
		t.Fatal("删除分类误删文章", nav)
	}
	for _, cat := range nav.Categories {
		if cat.ParentID != "" {
			t.Fatal("子项未迁到父级", cat)
		}
	}
}
func TestDeleteCategoryConflictRollsBackChildren(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	parent, _ := s.CreateCategory(ctx, "父", "")
	s.CreateCategory(ctx, "重名", "")
	s.CreateCategory(ctx, "重名", parent)
	preview, err := s.PreviewCategoryDeletion(ctx, parent)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteCategoryChecked(ctx, preview); err == nil {
		t.Fatal("子项重名应原子失败")
	}
	nav, err := s.Navigation(ctx)
	if err != nil || len(nav.Categories) != 3 {
		t.Fatal(nav, err)
	}
}
func TestFeedCategoryFutureOnlySurvivesRefresh(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	a, _ := s.CreateCategory(ctx, "旧分类", "")
	b, _ := s.CreateCategory(ctx, "新分类", "")
	f, err := s.AddSubscription(ctx, SubscriptionInput{URL: "https://example.test/future", Title: "源", CategoryIDs: []string{a}}, []ArticleInput{{GUID: "old", Title: "旧文"}}, FeedCheck{At: 100})
	if err != nil {
		t.Fatal(err)
	}
	old, _ := s.GetSubscription(ctx, f)
	future := false
	if err = s.UpdateSubscription(ctx, f, old.Revision, SubscriptionInput{URL: old.URL, Title: old.Title, CategoryIDs: []string{b}, ApplyExisting: &future}); err != nil {
		t.Fatal(err)
	}
	current, _ := s.GetSubscription(ctx, f)
	if err = s.UpdateSubscription(ctx, f, current.Revision, SubscriptionInput{URL: current.URL, Title: "只改名", CategoryIDs: []string{b}}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: "old", Title: "旧文更新"}); err != nil {
		t.Fatal(err)
	}
	first, _ := s.EffectiveMembers(ctx, a)
	newMembers, _ := s.EffectiveMembers(ctx, b)
	if len(first) != 1 || len(newMembers) != 0 {
		t.Fatal("刷新重分配了仅未来生效的旧文章", first, newMembers)
	}
	if _, err = s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: "new", Title: "新文"}); err != nil {
		t.Fatal(err)
	}
	newMembers, _ = s.EffectiveMembers(ctx, b)
	if len(newMembers) != 1 {
		t.Fatal("新文未继承新分类")
	}
}
