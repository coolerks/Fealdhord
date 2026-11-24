package appagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/feed"
	"github.com/coolerks/Fealdhord/internal/library"
	"github.com/coolerks/Fealdhord/internal/netpolicy"
	"github.com/coolerks/Fealdhord/internal/notification"
	"github.com/coolerks/Fealdhord/internal/reader"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResultContinuationDoesNotRepeatAnApprovedWrite(t *testing.T) {
	registry := NewRegistry()
	calls := 0
	Register(registry, "mutation", "mutate", true, func(context.Context, empty) []string {
		calls++
		return []string{strings.Repeat("details🙂", 3000), "last"}
	})
	store := &agentrun.ResultStore{}
	tool := registry.Tools()[0]
	ctx := agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalBypass)
	first, err := registry.executePaged(ctx, "test", "chat", "chat", store, tool, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	ctx = agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalDeny)
	found := false
	for next := first; ; {
		var page agentrun.ResultPage
		if err := json.Unmarshal([]byte(next), &page); err != nil {
			t.Fatal(err)
		}
		for _, entry := range page.Fragments {
			if entry.Path == "/1" && string(entry.Value) == `"last"` {
				found = true
			}
		}
		if page.NextCursor == "" {
			break
		}
		args, _ := json.Marshal(map[string]string{"result_cursor": page.NextCursor})
		if _, err := registry.executePaged(ctx, "test", "different-chat", "different-chat", store, tool, args); !errors.Is(err, agentrun.ErrResultUnavailable) {
			t.Fatalf("foreign cursor: %v", err)
		}
		next, err = registry.executePaged(ctx, "test", "chat", "chat", store, tool, args)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !found || calls != 1 {
		t.Fatalf("last=%t writes=%d", found, calls)
	}
}

type pagesFetcher struct {
	body  string
	calls int
}

func (f *pagesFetcher) Fetch(_ context.Context, target string, _ netpolicy.Request) (netpolicy.Response, error) {
	f.calls++
	return netpolicy.Response{Body: []byte(f.body), URL: target, Status: http.StatusOK}, nil
}

func TestArticleToolsKeepAllPagesAndOnlyExtractOnce(t *testing.T) {
	ctx := agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalBypass)
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UnixMilli()
	feedID, err := store.AddSubscription(ctx, database.SubscriptionInput{Title: "test", URL: "https://example.org/feed"}, nil, database.FeedCheck{At: now})
	if err != nil {
		t.Fatal(err)
	}
	long := strings.Repeat("中文段落🙂和更多正文。", 4000) + "原文最后一句"
	articleID, err := store.UpsertArticle(ctx, database.ArticleInput{
		FeedID: feedID, GUID: "one", Title: "文章", URL: "https://example.org/article", Text: long, FetchedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	fetch := &pagesFetcher{body: "<html><head><title>文章</title></head><body><article><p>" + long + "</p></article></body></html>"}
	registry := Builtins(Services{Store: store, Library: library.NewService(store), Reader: reader.NewService(store, fetch, nil, nil)})
	resultStore := &agentrun.ResultStore{}
	for _, tool := range registry.Tools() {
		if tool.Name != "article.get" && tool.Name != "article.extract" {
			continue
		}
		args, _ := json.Marshal(map[string]string{"id": articleID})
		next, err := registry.executePaged(ctx, "test", "chat", "chat", resultStore, tool, args)
		var body strings.Builder
		for {
			if err != nil {
				t.Fatalf("%s: %v", tool.Name, err)
			}
			var page agentrun.ArticlePage
			if json.Unmarshal([]byte(next), &page) != nil || page.ID != articleID || page.Offset != body.Len() {
				t.Fatalf("%s invalid page at %d: %.200s", tool.Name, body.Len(), next)
			}
			body.WriteString(page.Text)
			if page.NextCursor == "" {
				break
			}
			args, _ = json.Marshal(map[string]string{"result_cursor": page.NextCursor})
			next, err = registry.executePaged(ctx, "test", "chat", "chat", resultStore, tool, args)
		}
		if strings.TrimSpace(body.String()) != long {
			t.Fatalf("%s: missing text, length %d want %d", tool.Name, body.Len(), len(long))
		}
	}
	if fetch.calls != 1 {
		t.Fatalf("%d extractions", fetch.calls)
	}
}

func TestFeedDiscoverHasNoThirtyTwoCandidateCutoff(t *testing.T) {
	var document strings.Builder
	document.WriteString("<html><head>")
	for i := range 75 {
		fmt.Fprintf(&document, `<link rel="alternate" type="application/rss+xml" href="/feed/%d" title="%s">`, i, strings.Repeat("订阅", 50))
	}
	document.WriteString("</head></html>")
	fetch := &pagesFetcher{body: document.String()}
	registry := Builtins(Services{Feed: feed.NewService(nil, fetch, nil, nil, nil)})
	ctx := agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalBypass)
	results := &agentrun.ResultStore{}
	var tool Tool
	for _, entry := range registry.Tools() {
		if entry.Name == "feed.discover" {
			tool = entry
		}
	}
	next, err := registry.executePaged(ctx, "test", "chat", "chat", results, tool, json.RawMessage(`{"url":"https://example.org"}`))
	count, found := 0, false
	for {
		var page agentrun.ResultPage
		if err != nil || json.Unmarshal([]byte(next), &page) != nil || !page.Paged {
			t.Fatalf("discovery page: %v, %.200s", err, next)
		}
		for _, part := range page.Fragments {
			if strings.HasPrefix(part.Path, "/data/candidates/") {
				count++
				found = found || strings.Contains(string(part.Value), "https://example.org/feed/74")
			}
		}
		if page.NextCursor == "" {
			break
		}
		args, _ := json.Marshal(map[string]string{"result_cursor": page.NextCursor})
		next, err = registry.executePaged(ctx, "test", "chat", "chat", results, tool, args)
	}
	if count != 75 || !found || fetch.calls != 1 {
		t.Fatalf("catalog size=%d tail=%t fetches=%d", count, found, fetch.calls)
	}
}

func TestEveryBuiltinHasAReviewedResultPath(t *testing.T) {
	reviewed := strings.Fields(`
		article.get article.extract article.list article.search article.get_metadata
		article.mark_read article.mark_unread article.favorite article.unfavorite article.add_category article.remove_category
		feed.list feed.get feed.discover feed.add feed.update feed.remove feed.restore
		feed.refresh feed.refresh_all feed.refresh_status feed.cancel_refresh
		category.list category.get category.create category.update category.preview_remove category.remove
		category.article_memberships category.add_feed category.remove_feed
		view.list view.get_counts
		app.get_settings app.update_reading_settings app.get_refresh_settings app.update_refresh_settings app.get_version app.get_diagnostics
		article_ai.preview article_ai.evaluate opml.preview_import opml.import opml.export
		notification.channel.list notification.rule.list notification.send
		schedule.list schedule.get schedule.create schedule.update schedule.pause schedule.resume schedule.delete AskUserQuestion`)
	remaining := make(map[string]bool, len(reviewed))
	for _, name := range reviewed {
		remaining[name] = true
	}
	registry := Builtins(Services{Notifications: &notification.Service{}})
	for _, tool := range registry.Tools() {
		if !remaining[tool.Name] {
			t.Errorf("新增或重复工具 %s，检查并记录其结果分页行为", tool.Name)
		}
		delete(remaining, tool.Name)
	}
	if len(remaining) != 0 {
		t.Fatalf("工具已移除，更新审计清单: %v", remaining)
	}
}

func TestMCPResultCursorBoundToAuthorizedClient(t *testing.T) {
	ctx := t.Context()
	registry := NewRegistry()
	Register(registry, "items.list", "list", false, func(context.Context, id) []string {
		return []string{strings.Repeat("已授权的列表内容", 1000), "末项"}
	})
	platform := &Platform{registry: registry}
	tools := []string{"items.list"}
	results := &agentrun.ResultStore{}
	connect := func(principal string) *mcp.ClientSession {
		server := platform.newMCPServer(ServerConfig{Tools: tools}, tools, MCPScope{}, principal, results)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		serverSession, err := server.Connect(ctx, serverTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = serverSession.Close() })
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close() })
		return session
	}
	first, other := connect("first"), connect("other")
	pages, err := first.CallTool(ctx, &mcp.CallToolParams{Name: "items_list", Arguments: map[string]string{"id": "test"}})
	if err != nil || pages.IsError {
		t.Fatalf("initial call: %v %v", pages, err)
	}
	var page agentrun.ResultPage
	if err := json.Unmarshal([]byte(pages.Content[0].(*mcp.TextContent).Text), &page); err != nil || page.NextCursor == "" {
		t.Fatalf("first page: %v", err)
	}
	args := &mcp.CallToolParams{Name: "items_list", Arguments: map[string]string{"result_cursor": page.NextCursor}}
	foreign, err := other.CallTool(ctx, args)
	if err != nil || !foreign.IsError || foreign.Content[0].(*mcp.TextContent).Text != "ToolResultUnavailable" {
		t.Fatalf("foreign client access: %v %v", foreign, err)
	}
	continued, err := first.CallTool(ctx, args)
	if err != nil || continued.IsError {
		t.Fatalf("authorized continuation failed: %v %v", continued, err)
	}
}
