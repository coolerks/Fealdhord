package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func TestChatSendsStreamsCancelsAndRestores(t *testing.T) {
	ctx := context.Background()
	requests := make(chan string, 2)
	toolIDs := make(chan [2]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		requests <- string(payload)
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(string(payload), "测试授权工具") && !strings.Contains(string(payload), `"role":"tool"`) {
			ids := <-toolIDs
			calls := make([]map[string]any, 0, len(ids))
			for index, id := range ids {
				arguments, _ := json.Marshal(map[string]string{"id": id})
				calls = append(calls, map[string]any{"index": index, "id": fmt.Sprintf("read-%d", index), "type": "function",
					"function": map[string]string{"name": "read_article", "arguments": string(arguments)}})
			}
			delta, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": calls}}}})
			fmt.Fprintf(w, "data: %s\n\ndata: {\"choices\":[{\"index\":0,\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n", delta)
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"部分回复\"}}]}\n\n")
		w.(http.Flusher).Flush()
		if strings.Contains(string(payload), "取消这次") {
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	config := aiconfig.New(db.AIProviderQueries(), nil)
	service, err := New(ctx, db, config)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	scope := Scope{Kind: "article", ID: "local-article"}
	conversation, err := service.Create(ctx, scope, "测试历史")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Send(ctx, SendInput{ConversationID: conversation.ID, Text: "未配置"}); err == nil || err.Error() != "ConfigurationError" {
		t.Fatalf("missing model error: %v", err)
	}
	p, err := config.SaveProvider(ctx, aiconfig.ProviderInput{Provider: aiconfig.Provider{Name: "Mock", Protocol: "chat_completions", BaseURL: server.URL, Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := config.SaveModel(ctx, aiconfig.Model{ProviderID: p.ID, Name: "mock-model", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Assign(ctx, "chat", m.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Send(ctx, SendInput{ConversationID: conversation.ID, Text: "第一个问题", Selection: "仅本轮选段"}); err != nil {
		t.Fatal(err)
	}
	waitMessage(t, service, conversation.ID, 2, "complete")
	if request := <-requests; !strings.Contains(request, "仅本轮选段") {
		t.Fatalf("missing explicit selection: %s", request)
	}
	if err := service.Send(ctx, SendInput{ConversationID: conversation.ID, Text: "取消这次"}); err != nil {
		t.Fatal(err)
	}
	running := waitMessage(t, service, conversation.ID, 4, "running")
	if running.Text != "部分回复" {
		t.Fatalf("live reply = %+v", running)
	}
	if request := <-requests; strings.Contains(request, "仅本轮选段") || !strings.Contains(request, "第一个问题") {
		t.Fatalf("unexpected history: %s", request)
	}
	if err := service.Send(ctx, SendInput{ConversationID: conversation.ID, Text: "重复发送"}); err == nil || err.Error() != "Conflict" {
		t.Fatalf("reentry = %v", err)
	}
	if err := service.Cancel(ctx, conversation.ID); err != nil {
		t.Fatal(err)
	}
	cancelled := waitMessage(t, service, conversation.ID, 4, "cancelled")
	if cancelled.Text != "部分回复" {
		t.Fatalf("lost partial output: %+v", cancelled)
	}
	service.Close()
	if _, err := db.ChatQueries().AppendChatMessage(ctx, sqlc.AppendChatMessageParams{ID: "stale", ConversationID: conversation.ID, Role: "assistant", Status: "running", Content: "重启前保存"}); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(ctx, db, config)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	messages, err := reopened.Messages(ctx, conversation.ID)
	if err != nil || len(messages) != 5 || messages[3] != cancelled || messages[4].Status != "interrupted" || messages[4].Text != "重启前保存" {
		t.Fatalf("restored messages = %+v, %v", messages, err)
	}
	if list, err := reopened.List(ctx, Scope{Kind: "global"}); err != nil || len(list) != 0 {
		t.Fatalf("scope leaked: %+v, %v", list, err)
	}
	if list, err := reopened.List(ctx, scope); err != nil || len(list) != 1 || list[0].ID != conversation.ID {
		t.Fatalf("missing history: %+v, %v", list, err)
	}
	global, err := reopened.Create(ctx, Scope{Kind: "global"}, "来源选择")
	if err != nil {
		t.Fatal(err)
	}
	feed, err := db.EnsureFeed(ctx, "https://example.test/feed", "来源")
	if err != nil {
		t.Fatal(err)
	}
	selected, err := db.UpsertArticle(ctx, database.ArticleInput{FeedID: feed, GUID: "selected", Title: "已选文章", Summary: "公开摘要", Text: "不应发送的全文", FetchedAt: 1})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.UpsertArticle(ctx, database.ArticleInput{FeedID: feed, GUID: "other", Title: "未选文章", Summary: "不应发送的其他摘要", Text: "不可外传的正文", FetchedAt: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Send(ctx, SendInput{ConversationID: global.ID, Text: "先验证无效引用", ArticleIDs: []string{"missing"}}); err == nil || err.Error() != "ContextUnavailable" {
		t.Fatalf("missing context: %v", err)
	}
	if err := reopened.Send(ctx, SendInput{ConversationID: global.ID, Text: "请比较来源", ArticleIDs: []string{selected}}); err != nil {
		t.Fatal(err)
	}
	waitMessage(t, reopened, global.ID, 2, "complete")
	if request := <-requests; !strings.Contains(request, "公开摘要") || strings.Contains(request, "不应发送") {
		t.Fatalf("wrong source scope: %s", request)
	}
	toolIDs <- [2]string{selected, foreign}
	if err := reopened.Send(ctx, SendInput{ConversationID: global.ID, Text: "测试授权工具", ArticleIDs: []string{selected}, AllowArticleReads: true}); err != nil {
		t.Fatal(err)
	}
	waitMessage(t, reopened, global.ID, 4, "complete")
	initial, next := <-requests, <-requests
	if !strings.Contains(initial, `"name":"read_article"`) || strings.Contains(initial, "不应发送的全文") {
		t.Fatalf("bad initial tool request: %s", initial)
	}
	if !strings.Contains(next, "不应发送的全文") || !strings.Contains(next, "PermissionError") || strings.Contains(next, "不可外传的正文") {
		t.Fatalf("bad tool results: %s", next)
	}

}

func waitMessage(t *testing.T, service *Service, id string, count int, status string) Message {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		rows, err := service.Messages(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == count && rows[count-1].Status == status && rows[count-1].Text != "" {
			return rows[count-1]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", status)
	return Message{}
}
