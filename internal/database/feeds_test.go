package database

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFeedCommitAtomicValidatorsAndStates(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	input := SubscriptionInput{URL: "https://example.test/feed", Title: "订阅"}
	id, err := s.AddSubscription(ctx, input, []ArticleInput{{GUID: "one", Title: "原文", Text: "原文"}}, FeedCheck{At: 100, Next: 200, ETag: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := s.ListArticles(ctx, 10)
	articleID := rows[0].ID
	s.SetRead(ctx, articleID, true)
	s.SetFavorite(ctx, articleID, true)
	_, err = s.CommitFeed(ctx, id, input.URL, []ArticleInput{{GUID: "new", Title: "不应留下"}, {GUID: "invalid", Title: strings.Repeat("x", 4097)}}, FeedCheck{At: 200, Next: 300, ETag: "bad"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("无效批次未失败 %v", err)
	}
	rows, _ = s.ListArticles(ctx, 10)
	sub, _ := s.GetSubscription(ctx, id)
	if len(rows) != 1 || sub.ETag != "v1" {
		t.Fatal("文章或 validator 部分提交")
	}
	_, err = s.CommitFeed(ctx, id, input.URL, []ArticleInput{{GUID: "one", Title: "更新", Text: "更新正文"}}, FeedCheck{At: 200, Next: 300, ETag: "v2"})
	if err != nil {
		t.Fatal(err)
	}
	rows, _ = s.ListArticles(ctx, 10)
	if rows[0].ID != articleID || rows[0].IsRead != 1 || rows[0].IsFavorite != 1 || rows[0].SortAt != 100 {
		t.Fatal("重复刷新重置身份或状态")
	}
	_, err = s.CommitFeed(ctx, id, input.URL, nil, FeedCheck{At: 300, Next: 400, NotModified: true})
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := s.queries.GetArticle(ctx, articleID)
	sub, _ = s.GetSubscription(ctx, id)
	if stored.FetchedAt != 100 || sub.ETag != "v2" || *sub.SuccessAt != 300 {
		t.Fatal("304 改了文章或清空 validator")
	}
}
func TestSubscriptionRevisionArchiveRestoreAndStaleRefresh(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	input := SubscriptionInput{URL: "https://example.test/feed", Title: "原名"}
	id, err := s.AddSubscription(ctx, input, nil, FeedCheck{At: 100, Next: 200, ETag: "v1"})
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := s.AddSubscription(ctx, SubscriptionInput{URL: input.URL, Title: "不覆盖"}, nil, FeedCheck{At: 101})
	if err != nil || duplicate != id {
		t.Fatal("重复订阅身份分叉")
	}
	current, _ := s.GetSubscription(ctx, id)
	if current.Title != "原名" {
		t.Fatal("重复添加覆盖名称")
	}
	changed := input
	changed.URL = "https://example.test/new"
	changed.Title = "新名"
	if err = s.UpdateSubscription(ctx, id, current.Revision-1, changed); !errors.Is(err, ErrConflict) {
		t.Fatal("过期编辑成功")
	}
	if err = s.UpdateSubscription(ctx, id, current.Revision, changed); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CommitFeed(ctx, id, input.URL, nil, FeedCheck{At: 200, ETag: "stale"}); !errors.Is(err, ErrConflict) {
		t.Fatal("旧 URL 刷新覆盖新订阅")
	}
	current, _ = s.GetSubscription(ctx, id)
	if current.ETag != "" {
		t.Fatal("URL 改动未清旧 validator")
	}
	if err = s.ArchiveFeed(ctx, id, current.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CommitFeed(ctx, id, changed.URL, nil, FeedCheck{At: 200}); !errors.Is(err, ErrConflict) {
		t.Fatal("归档源仍提交")
	}
	current, _ = s.GetSubscription(ctx, id)
	if err = s.RestoreSubscription(ctx, id, current.Revision); err != nil {
		t.Fatal(err)
	}
	restored, _ := s.GetSubscription(ctx, id)
	if restored.Archived || restored.ID != id {
		t.Fatal("恢复没有复用身份")
	}
}
