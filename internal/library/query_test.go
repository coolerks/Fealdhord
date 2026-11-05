package library

import (
	"context"
	"github.com/coolerks/Fealdhord/internal/database"
	"path/filepath"
	"testing"
)

func TestQueryDTOHighlightsAndSafeCursorError(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "query.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	f, _ := store.EnsureFeed(ctx, "https://example.test/feed", "订阅")
	if _, err = store.UpsertArticle(ctx, database.ArticleInput{FeedID: f, GUID: "one", Title: "😀并发 Go", Text: "正文"}); err != nil {
		t.Fatal(err)
	}
	service := NewService(store)
	result := service.Query(ctx, QueryRequest{Search: "并发"})
	if !result.OK || result.Data.SnapshotID == "" || len(result.Data.Items) != 1 {
		t.Fatal(result)
	}
	article := result.Data.Items[0]
	if article.FeedID != f || len(article.TitleMatches) != 1 || article.TitleMatches[0].Start != 2 || article.TitleMatches[0].End != 4 {
		t.Fatal(article)
	}
	failed := service.Query(ctx, QueryRequest{Cursor: "tampered"})
	if failed.OK || failed.Error == nil || failed.Error.Code != "CursorExpired" {
		t.Fatal(failed)
	}
}
