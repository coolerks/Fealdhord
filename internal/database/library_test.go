package database

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

func seedArticle(t *testing.T, s *Store, input ArticleInput) string {
	t.Helper()
	id, err := s.UpsertArticle(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func seedFeed(t *testing.T, s *Store, url string) string {
	t.Helper()
	id, err := s.EnsureFeed(context.Background(), url, "测试订阅")
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func TestConcurrentIdentityAndPersistentStates(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	feed := seedFeed(t, s, "https://EXAMPLE.test:443/rss#fragment")
	same := seedFeed(t, s, "https://example.test/rss")
	if feed != same {
		t.Fatal("同订阅身份分叉")
	}
	input := ArticleInput{FeedID: feed, GUID: "g1", URL: "https://example.test/post", Title: "并发 Go", Text: "折栈阅读器", FetchedAt: 100}
	id := seedArticle(t, s, input)
	if err := s.SetRead(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFavorite(ctx, id, true); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	results := make(chan string, 16)
	failures := make(chan error, 16)
	for range 16 {
		group.Go(func() { got, err := s.UpsertArticle(ctx, input); results <- got; failures <- err })
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	for got := range results {
		if got != id {
			t.Fatal("并发入库产生不同 ID")
		}
	}
	input.GUID = "new-guid"
	input.Text = "新的正文"
	input.FetchedAt = 900
	if got := seedArticle(t, s, input); got != id {
		t.Fatal("URL 别名未合并")
	}
	rows, err := s.ListArticles(ctx, 100)
	if err != nil || len(rows) != 1 || rows[0].IsRead != 1 || rows[0].IsFavorite != 1 || rows[0].SortAt != 100 {
		t.Fatalf("刷新覆盖了状态或排序：%+v %v", rows, err)
	}
	input.GUID = "g1"
	input.URL = "https://example.test/unrelated"
	input.Text = "完全不同"
	if _, err := s.UpsertArticle(ctx, input); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("GUID 碰撞未隔离：%v", err)
	}
	old, err := s.queries.GetArticle(ctx, id)
	if err != nil || old.ContentText != "新的正文" {
		t.Fatal("身份冲突覆盖了旧正文")
	}
	otherFeed := seedFeed(t, s, "https://other.test/feed")
	input.FeedID = otherFeed
	if other := seedArticle(t, s, input); other == id {
		t.Fatal("跨 Feed 不应合并")
	}
	if err := s.CheckSearchIndex(ctx); err != nil {
		t.Fatal(err)
	}
}
func TestMembershipPriorityTreeAndDeletionRollback(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	feed := seedFeed(t, s, "https://example.test/rss")
	category, err := s.CreateCategory(ctx, "技术", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCategory(ctx, "技术", ""); err == nil {
		t.Fatal("根级重名未拒绝")
	}
	if err := s.SetFeedCategories(ctx, feed, []string{category}, true); err != nil {
		t.Fatal(err)
	}
	id := seedArticle(t, s, ArticleInput{FeedID: feed, GUID: "g", Title: "文章"})
	if members, _ := s.EffectiveMembers(ctx, category); len(members) != 1 {
		t.Fatal("未继承 Feed 分类")
	}
	if err := s.SetMembership(ctx, id, category, "manual", "excluded", nil); err != nil {
		t.Fatal(err)
	}
	score := .9
	if err := s.SetMembership(ctx, id, category, "ai", "included", &score); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFeedCategories(ctx, feed, []string{category}, true); err != nil {
		t.Fatal(err)
	}
	if members, _ := s.EffectiveMembers(ctx, category); len(members) != 0 {
		t.Fatal("自动来源覆盖了手动排除")
	}
	if err := s.ResetManualMembership(ctx, id, category); err != nil {
		t.Fatal(err)
	}
	if members, _ := s.EffectiveMembers(ctx, category); len(members) != 1 {
		t.Fatal("恢复自动判断无效")
	}
	low := .2
	if err := s.SetMembership(ctx, id, category, "ai", "included", &low); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFeedCategories(ctx, feed, nil, true); err != nil {
		t.Fatal(err)
	}
	if members, _ := s.EffectiveMembers(ctx, category); len(members) != 0 {
		t.Fatal("低置信 AI 不应有效")
	}
	if err := s.MoveCategory(ctx, category, "", 999); !errors.Is(err, ErrConflict) {
		t.Fatalf("过期 revision 未返回冲突：%v", err)
	}
	parent := category
	nodes := []string{category}
	for index := 1; index < 5; index++ {
		next, err := s.CreateCategory(ctx, "子级", parent)
		if err != nil {
			t.Fatal(err)
		}
		nodes = append(nodes, next)
		parent = next
	}
	if _, err := s.CreateCategory(ctx, "越深", parent); err == nil {
		t.Fatal("允许超过五级")
	}
	if err := s.MoveCategory(ctx, category, parent, 0); err == nil {
		t.Fatal("允许分类环")
	}
	if _, err := s.CreateCategory(ctx, "子级", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteCategory(ctx, category); err == nil {
		t.Fatal("删除造成根级重名却未回滚")
	}
	if _, err := s.queries.GetCategory(ctx, category); err != nil {
		t.Fatal("失败删除丢了父级")
	}
	if _, err := s.queries.GetArticle(ctx, id); err != nil {
		t.Fatal("分类操作删除了文章")
	}
	if err := s.DeleteCategory(ctx, nodes[len(nodes)-1]); err != nil {
		t.Fatal(err)
	}
}
func TestForeignKeysConstraintsAndFTSLifecycle(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	feed := seedFeed(t, s, "https://example.test/rss")
	if _, err := s.UpsertArticle(ctx, ArticleInput{FeedID: "missing", GUID: "bad"}); err == nil {
		t.Fatal("失效外键未拒绝")
	}
	id := seedArticle(t, s, ArticleInput{FeedID: feed, GUID: "g", Title: "Go 并发与折栈", Text: "学习数据库 中文搜索"})
	for _, query := range []string{"并发", "栈", "Go 并发", "go并发", "中文搜索"} {
		result, err := s.Search(ctx, query, 20)
		if err != nil || len(result.Items) != 1 || result.Items[0].ID != id {
			t.Fatalf("搜索 %q 错误：%+v %v", query, result, err)
		}
	}
	for _, query := range []string{`" OR * :`, "';DROP TABLE articles;--", "不存在", "！！！"} {
		if result, err := s.Search(ctx, query, 20); err != nil || len(result.Items) != 0 {
			t.Fatalf("字面标点导致 FTS 语法错误：%q %v", query, err)
		}
	}
	seedArticle(t, s, ArticleInput{FeedID: feed, GUID: "g", Title: "Rust 新内容", Text: "事务更新"})
	if result, err := s.Search(ctx, "并发", 20); err != nil || len(result.Items) != 0 {
		t.Fatalf("更新残留旧索引：%+v %v", result, err)
	}
	if err := s.CheckSearchIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.RebuildSearchIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if result, err := s.Search(ctx, "事务", 20); err != nil || len(result.Items) != 1 {
		t.Fatal("重建索引不一致", err)
	}
	if err := s.DeleteArticle(ctx, id); err != nil {
		t.Fatal(err)
	}
	if result, err := s.Search(ctx, "事务", 20); err != nil || len(result.Items) != 0 {
		t.Fatal("删除残留索引", err)
	}
	if err := s.CheckSearchIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFavorite(ctx, id, true); !errors.Is(err, ErrNotFound) {
		t.Fatal("失效目标仍可收藏")
	}
}

func TestFTSRollbackLiteralBoundsAndUnicode(t *testing.T) {
	ctx := context.Background()
	s := testStore(t)
	feed := seedFeed(t, s, "https://example.test/rss")
	if _, err := s.db.ExecContext(ctx, "CREATE TRIGGER reject_identity BEFORE INSERT ON article_identities BEGIN SELECT RAISE(ABORT,'injected identity failure'); END;"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpsertArticle(ctx, ArticleInput{FeedID: feed, GUID: "rollback", Title: "回滚正文"}); err == nil {
		t.Fatal("未触发故障")
	}
	if result, err := s.Search(ctx, "回滚", 20); err != nil || len(result.Items) != 0 {
		t.Fatal("回滚留下全文索引", err)
	}
	if err := s.CheckSearchIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "DROP TRIGGER reject_identity"); err != nil {
		t.Fatal(err)
	}
	seedArticle(t, s, ArticleInput{FeedID: feed, GUID: "unicode", Title: "𠮷野家 Go并发阅读", Text: "甲乙甲甲丙"})
	if result, err := s.Search(ctx, "𠮷野", 20); err != nil || len(result.Items) != 1 {
		t.Fatalf("Unicode 搜索错误：%+v %v", result, err)
	}
	if result, err := s.Search(ctx, "甲乙甲丙", 20); err != nil || len(result.Items) != 0 {
		t.Fatal("二字 token 集合假阳性未过滤", err)
	}
	seedArticle(t, s, ArticleInput{FeedID: feed, GUID: "second", Title: "Go并发"})
	if result, err := s.Search(ctx, "并发", 1); err != nil || len(result.Items) != 1 || !result.Truncated {
		t.Fatal("检索结果未有界", err)
	}
	if _, err := s.Search(ctx, strings.Repeat("栈", 513), 10); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("超长检索未拒绝")
	}
}
