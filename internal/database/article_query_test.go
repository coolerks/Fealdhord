package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestQueryPaginationScopeSearchAndSnapshot(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f, _ := s.EnsureFeed(ctx, "https://example.test/query", "查询")
	category, _ := s.CreateCategory(ctx, "分类", "")
	ids := []string{}
	for i := range 530 {
		text := "普通文章"
		if i == 0 {
			text = "并发 Go 栈"
		}
		id, err := s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: fmt.Sprint(i), Title: fmt.Sprintf("标题 %03d", i), Text: text, FetchedAt: int64(i + 1)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	query := ArticleQuery{Limit: 37}
	seen := map[string]bool{}
	pages := 0
	for {
		page, err := s.QueryArticles(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range page.Items {
			if seen[row.ID] {
				t.Fatal("分页重复")
			}
			seen[row.ID] = true
		}
		pages++
		if pages == 1 {
			if _, err = s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: "later", Title: "后来插入旧日期", FetchedAt: 2}); err != nil {
				t.Fatal(err)
			}
		}
		if page.NextCursor == "" {
			break
		}
		query.Cursor = page.NextCursor
	}
	if len(seen) != 530 || pages < 2 {
		t.Fatal("仍受500条上限或混入新文", len(seen), pages)
	}
	page, err := s.QueryArticles(ctx, ArticleQuery{Search: "并发 Go 栈"})
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != ids[0] {
		t.Fatal(page, err)
	}
	if err = s.SetFavorite(ctx, ids[0], true); err != nil {
		t.Fatal(err)
	}
	if err = s.SetRead(ctx, ids[0], true); err != nil {
		t.Fatal(err)
	}
	if err = s.SetMembership(ctx, ids[0], category, "manual", "included", nil); err != nil {
		t.Fatal(err)
	}
	page, err = s.QueryArticles(ctx, ArticleQuery{Scope: ArticleScope{Kind: "category", ID: category}, Read: "read"})
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	page, err = s.QueryArticles(ctx, ArticleQuery{Scope: ArticleScope{Kind: "favorites"}})
	if err != nil || len(page.Items) != 1 {
		t.Fatal(page, err)
	}
	page, err = s.QueryArticles(ctx, ArticleQuery{Search: `" : * OR 1=1 --`})
	if err != nil {
		t.Fatal("语法注入进入FTS", err)
	}
	if _, err = s.QueryArticles(ctx, ArticleQuery{Search: strings.Repeat("栈", 513)}); !errors.Is(err, ErrInvalidInput) {
		t.Fatal(err)
	}
}
func TestQueryCursorBindingAndRankGeneration(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f, _ := s.EnsureFeed(ctx, "https://example.test/fts", "搜索")
	for i := range 9 {
		if _, err := s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: fmt.Sprint(i), Title: "并发搜索", Text: "Go 并发", FetchedAt: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	query := ArticleQuery{Search: "并发", Limit: 2}
	seen := map[string]bool{}
	for {
		page, err := s.QueryArticles(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if seen[item.ID] {
				t.Fatal("排名分页重复")
			}
			seen[item.ID] = true
		}
		if page.NextCursor == "" {
			break
		}
		query.Cursor = page.NextCursor
	}
	if len(seen) != 9 {
		t.Fatal("排名分页漏页", len(seen))
	}
	first, err := s.QueryArticles(ctx, ArticleQuery{Search: "并发", Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.QueryArticles(ctx, ArticleQuery{Search: "Go", Cursor: first.NextCursor}); !errors.Is(err, ErrCursorExpired) {
		t.Fatal("跨查询游标被接受", err)
	}
	if _, err = s.QueryArticles(ctx, ArticleQuery{Search: "并发", Cursor: first.NextCursor + "x"}); !errors.Is(err, ErrCursorExpired) {
		t.Fatal("篡改游标被接受", err)
	}
	if err = s.RebuildSearchIndex(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err = s.QueryArticles(ctx, ArticleQuery{Search: "并发", Cursor: first.NextCursor}); !errors.Is(err, ErrCursorExpired) {
		t.Fatal("重建后旧排名仍有效", err)
	}
}
func TestSnapshotExcludesReusedRowID(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f, _ := s.EnsureFeed(ctx, "https://example.test/reuse", "查询")
	ids := []string{}
	for i := range 4 {
		id, err := s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: fmt.Sprint(i), Title: "文章", FetchedAt: int64(i + 1)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	first, err := s.QueryArticles(ctx, ArticleQuery{Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.DeleteArticle(ctx, ids[3]); err != nil {
		t.Fatal(err)
	}
	newID, err := s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: "new", Title: "复用rowid", FetchedAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.QueryArticles(ctx, ArticleQuery{Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 {
		t.Fatal(len(page.Items))
	}
	for _, row := range page.Items {
		if row.ID == newID {
			t.Fatal("新文章借复用rowid混入旧快照")
		}
	}
}

func TestSearchCandidateLimitDoesNotPretendExhaustion(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	f, _ := s.EnsureFeed(ctx, "https://example.test/candidates", "候选")
	for i := range 1002 {
		if _, err := s.UpsertArticle(ctx, ArticleInput{FeedID: f, GUID: fmt.Sprint(i), Title: "并发和发控与控制", FetchedAt: int64(i + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	query := ArticleQuery{Search: "并发控制", Limit: 2}
	first, err := s.QueryArticles(ctx, query)
	if err != nil || len(first.Items) != 0 || !first.SearchLimited || first.NextCursor == "" {
		t.Fatal("候选截断被误报为穷尽", first, err)
	}
	query.Cursor = first.NextCursor
	second, err := s.QueryArticles(ctx, query)
	if err != nil || second.NextCursor != "" || len(second.Items) != 0 {
		t.Fatal(second, err)
	}
}
