package appagent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/library"
)

func TestArticleListToolAcceptsDefaultAndNewestSort(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "articles.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC().UnixMilli()
	feedID, err := store.AddSubscription(ctx, database.SubscriptionInput{
		Title: "V2EX - 最热",
		URL:   "https://example.test/v2ex.xml",
	}, nil, database.FeedCheck{At: now})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpsertArticle(ctx, database.ArticleInput{
		FeedID:    feedID,
		GUID:      "v2ex-1",
		Title:     "今天 V2EX 热帖",
		Summary:   "讨论内容",
		FetchedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	registry := Builtins(Services{
		Store:   store,
		Library: library.NewService(store),
	})
	var listTool Tool
	for _, tool := range registry.Tools() {
		if tool.Name == "article.list" {
			listTool = tool
			break
		}
	}
	if listTool.Name == "" {
		t.Fatal("article.list tool not registered")
	}

	for _, args := range []string{
		`{}`,
		`{"limit":20}`,
		`{"scope":{"kind":"feed","id":"` + feedID + `"},"sort":"newest","limit":10}`,
	} {
		out, err := registry.execute(ctx, "test", listTool, json.RawMessage(args))
		if err != nil {
			t.Fatalf("args %s failed: %v", args, err)
		}
		var result library.ListResult
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("args %s decode failed: %v (%s)", args, err, out)
		}
		if !result.OK || result.Data == nil || len(result.Data.Items) != 1 {
			t.Fatalf("args %s unexpected result: %+v", args, result)
		}
	}
}
