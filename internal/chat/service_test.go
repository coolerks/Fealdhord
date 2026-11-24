package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/appagent"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
)

func TestChatSendsStreamsCancelsAndRestores(t *testing.T) {
	ctx := context.Background()
	requests := make(chan string, 2)
	toolIDs := make(chan [2]string, 1)
	resume := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		requests <- string(payload)
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(string(payload), "测试授权工具") && strings.Contains(string(payload), `"tools":[`) &&
			!strings.Contains(string(payload), `"role":"tool"`) {
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
		if strings.Contains(string(payload), "取消这次") {
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"部分🙂\"}}]}\n\n")
			w.(http.Flusher).Flush()
			select {
			case <-resume:
			case <-r.Context().Done():
				return
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"回复\"}}]}\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"部分回复\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	config := aiconfig.New(db.AIProviderQueries(), nil)
	updates := make(chan UpdateEvent, 8)
	service, err := New(ctx, db, config, func(event UpdateEvent) { updates <- event })
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
	m, err := config.SaveModel(ctx, aiconfig.Model{ProviderID: p.ID, Name: "mock-model", Enabled: true,
		ReasoningLevels: []aiconfig.ReasoningLevel{{Label: "低", Value: "low"}}})
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
	if first, done := takeChatUpdate(t, updates), takeChatUpdate(t, updates); first.Delta != "部分回复" || !done.Done || done.MessageID != first.MessageID {
		t.Fatalf("complete stream events: %+v, %+v", first, done)
	}
	if request := <-requests; !strings.Contains(request, "仅本轮选段") {
		t.Fatalf("missing explicit selection: %s", request)
	} else if strings.Contains(request, "reasoning_effort") {
		t.Fatalf("default effort must be omitted: %s", request)
	}
	if err := service.Send(ctx, SendInput{ConversationID: conversation.ID, Text: "取消这次"}); err != nil {
		t.Fatal(err)
	}
	first := takeChatUpdate(t, updates)
	if first.ConversationID != conversation.ID || first.Offset != 0 || first.Delta != "部分🙂" || first.Done {
		t.Fatalf("first live delta = %+v", first)
	}
	close(resume)
	second := takeChatUpdate(t, updates)
	running := waitMessage(t, service, conversation.ID, 4, "running")
	if second.MessageID != running.ID || second.Offset != 4 || second.Delta != "回复" || running.Text != "部分🙂回复" {
		t.Fatalf("second live delta = %+v, snapshot = %+v", second, running)
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
	if done := takeChatUpdate(t, updates); !done.Done || done.MessageID != running.ID || cancelled.Text != "部分🙂回复" {
		t.Fatalf("lost partial output: %+v", cancelled)
	}
	service.Close()
	if _, err := db.ChatQueries().AppendChatMessage(ctx, sqlc.AppendChatMessageParams{ID: "stale", ConversationID: conversation.ID, Role: "assistant", Status: "running", Content: "重启前保存"}); err != nil {
		t.Fatal(err)
	}
	if err := db.ChatQueries().StartChatToolAudit(ctx, sqlc.StartChatToolAuditParams{MessageID: "stale", CallID: "stale-call", ToolName: "read_article", StartedAt: 1}); err != nil {
		t.Fatal(err)
	}
	reopened, err := New(ctx, db, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	messages, err := reopened.Messages(ctx, conversation.ID)
	if err != nil || len(messages) != 5 || !reflect.DeepEqual(messages[3], cancelled) || messages[4].Status != "interrupted" || messages[4].Text != "重启前保存" {
		t.Fatalf("restored messages = %+v, %v", messages, err)
	}
	if len(messages[4].Tools) != 1 || messages[4].Tools[0].Status != "interrupted" {
		t.Fatalf("audit recovery = %+v", messages[4].Tools)
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
	platform := appagent.NewPlatform(appagent.NewRegistry(), t.TempDir())
	reopened.SetToolPermissions(platform)
	if err := platform.SaveToolPermission("builtin:read_article", "bypass"); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Send(ctx, SendInput{ConversationID: global.ID, Text: "测试授权工具", ArticleIDs: []string{selected}, AllowArticleReads: true, ApprovalMode: "custom"}); err != nil {
		t.Fatal(err)
	}
	withTools := waitMessage(t, reopened, global.ID, 4, "complete")
	if len(withTools.Tools) != 2 || withTools.Tools[0].Status != "completed" || withTools.Tools[1].Status != "denied" {
		t.Fatalf("audit = %+v", withTools.Tools)
	}
	if withTools.Details == nil || withTools.Details.Work == nil {
		t.Fatalf("article activity missing: %+v", withTools.Details)
	}
	steps := withTools.Details.Work.Tools
	if len(steps) != 2 || steps[0].Status != "completed" || steps[1].Status != "denied" {
		t.Fatalf("article activity = %+v", steps)
	}
	initial, next := <-requests, <-requests
	if !strings.Contains(initial, `"name":"read_article"`) || strings.Contains(initial, "不应发送的全文") {
		t.Fatalf("bad initial tool request: %s", initial)
	}
	if !strings.Contains(next, "不应发送的全文") || !strings.Contains(next, "PermissionError") || strings.Contains(next, "不可外传的正文") {
		t.Fatalf("bad tool results: %s", next)
	}
	if err := platform.SaveToolPermission("builtin:read_article", "deny"); err != nil {
		t.Fatal(err)
	}
	deniedConversation, err := reopened.Create(ctx, Scope{Kind: "global"}, "拒绝正文")
	if err != nil {
		t.Fatal(err)
	}
	toolIDs <- [2]string{selected, foreign}
	if err := reopened.Send(ctx, SendInput{ConversationID: deniedConversation.ID, Text: "测试授权工具", ArticleIDs: []string{selected}, AllowArticleReads: true, ApprovalMode: "custom"}); err != nil {
		t.Fatal(err)
	}
	denied := waitMessage(t, reopened, deniedConversation.ID, 2, "complete")
	<-requests
	deniedPayload := <-requests
	if len(denied.Tools) != 2 || denied.Tools[0].ErrorCode != "PermissionDenied" || denied.Tools[1].ErrorCode != "PermissionError" ||
		strings.Contains(deniedPayload, "不应发送的全文") || strings.Contains(deniedPayload, "不可外传的正文") {
		t.Fatalf("custom denial leaked article content or lost its audit state: %+v", denied.Tools)
	}

	alternate, err := config.SaveModel(ctx, aiconfig.Model{
		ProviderID: p.ID, Name: "deepseek-reasoner", DisplayName: "备用模型", Enabled: true,
		ReasoningLevels: []aiconfig.ReasoningLevel{{Label: "极深", Value: "custom_max"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, choice := range []aiconfig.Model{alternate, m} {
		effort := choice.ReasoningLevels[0].Value
		err := reopened.Send(ctx, SendInput{
			ConversationID: global.ID, ModelID: choice.ID, ReasoningEffort: effort, Text: fmt.Sprintf("模型切换 %d", index),
		})
		if err != nil {
			t.Fatal(err)
		}
		reply := waitMessage(t, reopened, global.ID, 6+index*2, "complete")
		payload := <-requests
		if reply.ModelName != choice.DisplayName || !strings.Contains(payload, fmt.Sprintf(`"model":%q`, choice.Name)) ||
			!strings.Contains(payload, fmt.Sprintf(`"reasoning_effort":%q`, effort)) ||
			!strings.Contains(payload, "测试授权工具") || (index > 0 && !strings.Contains(payload, "模型切换 0")) {
			t.Fatalf("model switch lost model or history: reply=%+v payload=%s", reply, payload)
		}
	}
	if err := reopened.Send(ctx, SendInput{ConversationID: global.ID, ModelID: m.ID, ReasoningEffort: "custom_max", Text: "其他模型档位"}); err == nil || err.Error() != "ChatReasoningUnavailable" {
		t.Fatalf("foreign effort = %v", err)
	}
	if err := reopened.Send(ctx, SendInput{ConversationID: global.ID, ModelID: alternate.ID, ReasoningEffort: "custom_max",
		Text: "需要续传的工具组合", ArticleIDs: []string{selected}, AllowArticleReads: true}); err == nil || err.Error() != "ChatReasoningToolsUnavailable" {
		t.Fatalf("unsupported tool continuation = %v", err)
	}
	if assigned, err := reopened.Model(ctx); err != nil || assigned.ID != m.ID {
		t.Fatalf("model switch changed default: %+v, %v", assigned, err)
	}
	alternate.Enabled = false
	if _, err := config.SaveModel(ctx, alternate); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Send(ctx, SendInput{ConversationID: global.ID, ModelID: alternate.ID, Text: "停用模型"}); err == nil || err.Error() != "ChatModelUnavailable" {
		t.Fatalf("disabled model = %v", err)
	}
	if err := reopened.Send(ctx, SendInput{ConversationID: conversation.ID, ModelID: m.ID, Text: "非全局用途"}); err == nil || err.Error() != "ValidationError" {
		t.Fatalf("non-global override = %v", err)
	}
	rows, err := reopened.Messages(ctx, global.ID)
	if err != nil || len(rows) != 8 {
		t.Fatalf("rejected model persisted a message: %d, %v", len(rows), err)
	}
}

func takeChatUpdate(t *testing.T, updates <-chan UpdateEvent) UpdateEvent {
	t.Helper()
	select {
	case update := <-updates:
		return update
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for a chat update")
		return UpdateEvent{}
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
