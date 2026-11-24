package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/appagent"
	"github.com/coolerks/Fealdhord/internal/database"
)

func TestGlobalChatContinuesRoutedDeepSeekResponsesWithReasoning(t *testing.T) {
	ctx := t.Context()
	const modelID = "commandcode-plan/deepseek/deepseek-v4.1-flash"
	requests := make(chan string, 2)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path != "/v1/responses" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		payload := string(body)
		select {
		case requests <- payload:
		default:
			t.Error("unexpected extra request")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		events := []string{`{"type":"response.created"}`}
		if strings.Contains(payload, `"type":"function_call_output"`) {
			events = append(events,
				`{"type":"response.output_text.delta","delta":"找到测试文章"}`,
				`{"type":"response.completed","response":{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"找到测试文章"}]}]}}`,
			)
		} else {
			events = append(events,
				`{"type":"response.reasoning_text.delta","delta":"先查本地资料"}`,
				`{"type":"response.output_item.added","item":{"type":"function_call","id":"item-1","call_id":"call-1","name":"library_search","arguments":""}}`,
				`{"type":"response.function_call_arguments.delta","item_id":"item-1","delta":"{}"}`,
				`{"type":"response.completed","response":{"status":"completed","output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"先查本地资料"}]},{"type":"function_call","id":"item-1","call_id":"call-1","name":"library_search","arguments":"{}"}]}}`,
			)
		}
		for _, event := range events {
			fmt.Fprintf(w, "data: %s\n\n", event)
		}
	}))
	defer gateway.Close()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	config := aiconfig.New(db.AIProviderQueries(), nil)
	provider, err := config.SaveProvider(ctx, aiconfig.ProviderInput{Provider: aiconfig.Provider{
		Name: "Gateway", Protocol: "responses", BaseURL: gateway.URL + "/v1", Enabled: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	model, err := config.SaveModel(ctx, aiconfig.Model{
		ProviderID: provider.ID, Name: modelID, Enabled: true,
		ReasoningLevels: []aiconfig.ReasoningLevel{{Label: "中", Value: "medium"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(ctx, db, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	registry := appagent.NewRegistry()
	appagent.Register(registry, "library.search", "查找本地文章", false,
		func(context.Context, struct{}) []string { return []string{"测试文章"} })
	platform := appagent.NewPlatform(registry, t.TempDir())
	if err := platform.SaveToolPermission("builtin:library.search", "ask"); err != nil {
		t.Fatal(err)
	}
	service.SetToolSource(platform.RunTools)
	service.SetToolPermissions(platform)
	conversation, err := service.Create(ctx, Scope{Kind: "global"}, "Responses 工具回归")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Send(ctx, SendInput{ConversationID: conversation.ID, ModelID: model.ID,
		ReasoningEffort: "medium", ApprovalMode: "custom", Text: "看一下最近的 AI 文章"}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	pending := registry.Pending()
	for len(pending) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		pending = registry.Pending()
	}
	if len(pending) != 1 || pending[0].ConversationID != conversation.ID || !registry.Decide(pending[0].ID, true) {
		t.Fatalf("expected one approval for the conversation, got %v", pending)
	}
	reply := waitMessage(t, service, conversation.ID, 2, "complete")
	if reply.Text != "找到测试文章" || len(registry.RecentAudit()) != 1 || len(requests) != 2 {
		t.Fatalf("unexpected reply or tool audit: %+v, %+v", reply, registry.RecentAudit())
	}
	first, next := <-requests, <-requests
	for _, payload := range []string{first, next} {
		for _, required := range []string{`"model":"` + modelID + `"`, `"reasoning":{"effort":"medium"}`, `"tools":[`, `"name":"library_search"`} {
			if !strings.Contains(payload, required) {
				t.Fatalf("request missing %s: %s", required, payload)
			}
		}
	}
	reasoningAt := strings.Index(next, `{"type":"reasoning","content":[{"type":"reasoning_text","text":"先查本地资料"}]}`)
	callAt := strings.Index(next, `{"type":"function_call","call_id":"call-1"`)
	resultAt := strings.Index(next, `{"type":"function_call_output","call_id":"call-1"`)
	if reasoningAt == -1 || callAt <= reasoningAt || resultAt <= callAt || !strings.Contains(next, "测试文章") {
		t.Fatalf("incorrect tool continuation: %s", next)
	}
}

func TestGlobalChatSendsAuthorizedToolsToRoutedDeepSeek(t *testing.T) {
	ctx := t.Context()
	const modelID = "commandcode-plan/deepseek/deepseek-v4.1-flash"
	requests := make(chan string, 5)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		requests <- string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(string(body), `"tools":[`) && !strings.Contains(string(body), `"role":"tool"`) {
			if !strings.Contains(string(body), `"reasoning_effort":`) {
				fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"先查🙂"}}]}

data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"library_search","arguments":"{}"}}]}}]}

data: {"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":20,"completion_tokens":3}}

data: [DONE]

`)
				return
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"provider reasoning\",\"content\":\"先查🙂\",\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"library_search\",\"arguments\":\"{}\"}}]}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"finish_reason\":\"tool_calls\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":20,\"prompt_cache_hit_tokens\":5,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"完成\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":30,\"prompt_tokens_details\":{\"cached_tokens\":7},\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")
	}))
	defer gateway.Close()

	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	config := aiconfig.New(db.AIProviderQueries(), nil)
	provider, err := config.SaveProvider(ctx, aiconfig.ProviderInput{Provider: aiconfig.Provider{
		Name: "Gateway", Protocol: "chat_completions", BaseURL: gateway.URL, Enabled: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	model, err := config.SaveModel(ctx, aiconfig.Model{
		ProviderID: provider.ID, Name: modelID, DisplayName: "DeepSeek V4.1 Flash", Enabled: true,
		ReasoningLevels: []aiconfig.ReasoningLevel{{Label: "低", Value: "low"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := config.SaveModel(ctx, aiconfig.Model{
		ProviderID: provider.ID, Name: "deepseek-reasoner", Enabled: true, ReasoningLevels: model.ReasoningLevels,
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(ctx, db, config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	registry := appagent.NewRegistry()
	appagent.Register(registry, "library.search", "查找本地文章", true,
		func(context.Context, struct{}) []string { return []string{"测试文章"} })
	platform := appagent.NewPlatform(registry, t.TempDir())
	service.SetToolSource(platform.RunTools)
	conversation, err := service.Create(ctx, Scope{Kind: "global"}, "工具回归")
	if err != nil {
		t.Fatal(err)
	}

	input := SendInput{ConversationID: conversation.ID, ModelID: legacy.ID, ReasoningEffort: "low", Text: "查找本地文章"}
	if err := service.Send(ctx, input); err == nil || err.Error() != "ChatReasoningToolsUnavailable" {
		t.Fatalf("unsupported tool combination must report an explicit error: %v", err)
	}

	input.ModelID, input.ApprovalMode = model.ID, "bypass"
	if err := service.Send(ctx, input); err != nil {
		t.Fatal(err)
	}
	reply := waitMessage(t, service, conversation.ID, 2, "complete")
	if audit := registry.RecentAudit(); reply.ModelName != model.DisplayName ||
		len(audit) != 1 || audit[0].Tool != "library.search" || audit[0].Status != "complete" {
		t.Fatalf("incorrect model mode or tool audit: reply=%+v audit=%+v", reply, audit)
	}
	first, next := <-requests, <-requests
	for _, payload := range []string{first, next} {
		for _, required := range []string{fmt.Sprintf(`"model":%q`, modelID), `"thinking":{"type":"enabled"}`, `"reasoning_effort":"low"`, `"name":"library_search"`, `"parameters":{`, "以本轮实际提供的工具"} {
			if !strings.Contains(payload, required) {
				t.Fatalf("request missing %q: %s", required, payload)
			}
		}
	}
	if !strings.Contains(next, `"tool_call_id":"call-1"`) || !strings.Contains(next, "测试文章") || !strings.Contains(next, `"reasoning_content":"provider reasoning"`) {
		t.Fatalf("tool result not returned to the model: %s", next)
	}

	if d := reply.Details; d == nil || d.Rounds != 2 || d.ReasoningEffort != "低" ||
		d.InputTokens == nil || *d.InputTokens != 50 || d.CachedInputTokens == nil || *d.CachedInputTokens != 12 ||
		d.OutputTokens == nil || *d.OutputTokens != 8 || d.FirstTokenMS == nil {
		t.Fatalf("incorrect aggregate reply usage: %+v", d)
	}
	work := reply.Details.Work
	if work == nil || !work.ReasoningObserved || work.ReasoningText != "provider reasoning" || work.ReasoningSummary != "" || len(work.Tools) != 1 {
		t.Fatalf("incorrect recorded work: %+v", work)
	}
	if len(work.Positions) != 2 || work.Positions[0].Offset != 0 || work.Positions[0].ToolEnd != 0 ||
		work.Positions[1].Offset != 4 || work.Positions[1].ToolEnd != 1 || reply.Text != "先查🙂完成" {
		t.Fatalf("unexpected work positions for %q: %+v", reply.Text, work.Positions)
	}
	if step := work.Tools[0]; step.Name != "library_search" || step.Status != "completed" ||
		step.Arguments == nil || *step.Arguments != "{}" || step.Result == nil || !strings.Contains(*step.Result, "测试文章") {
		t.Fatalf("incorrect tool record: %+v", step)
	}

	input.DisableTools = true
	input.Text = "列出你的工具列表"
	if err := service.Send(ctx, input); err != nil {
		t.Fatal(err)
	}
	waitMessage(t, service, conversation.ID, 4, "complete")
	plain := <-requests
	if !strings.Contains(plain, `"reasoning_effort":"low"`) || !strings.Contains(plain, "本轮没有提供工具") ||
		strings.Contains(plain, `"tools":`) || strings.Contains(plain, `"thinking":`) {
		t.Fatalf("tool-free chat did not preserve selected effort or report its runtime capabilities: %s", plain)
	}

	branch, err := service.Branch(ctx, conversation.ID, reply.ID)
	if err != nil || branch.ID == conversation.ID {
		t.Fatalf("branch was not created: %+v %v", branch, err)
	}
	branchMessages, err := service.Messages(ctx, branch.ID)
	if err != nil || len(branchMessages) != 2 || branchMessages[1].ID == reply.ID ||
		branchMessages[1].Text != reply.Text || branchMessages[1].Details == nil ||
		branchMessages[1].Details.Work == nil || len(branchMessages[1].Details.Work.Tools) != 1 ||
		branchMessages[1].Details.Work.ReasoningText != work.ReasoningText ||
		branchMessages[1].Details.Work.Tools[0].Result == nil || *branchMessages[1].Details.Work.Tools[0].Result != *work.Tools[0].Result {
		t.Fatalf("branch did not copy the selected prefix: %+v %v", branchMessages, err)
	}
	if _, err := service.Branch(ctx, branch.ID, reply.ID); err == nil || err.Error() != "ChatBranchUnavailable" {
		t.Fatalf("foreign branch point accepted: %v", err)
	}
	input.ConversationID, input.DisableTools, input.Text = branch.ID, false, "继续查找"
	if err := service.Send(ctx, input); err != nil {
		t.Fatal(err)
	}
	waitMessage(t, service, branch.ID, 4, "complete")
	resumed := <-requests
	if strings.Contains(resumed, `"role":"assistant"`) || strings.Contains(resumed, "provider reasoning") ||
		!strings.Contains(resumed, "先前可见对话") || !strings.Contains(resumed, "继续查找") {
		t.Fatalf("persisted visible history was not passed as context: %s", resumed)
	}
	<-requests
	// Some gateways omit reasoning entirely even when thinking was requested.
	input.Text, input.ReasoningEffort = "再次搜索本地文章", ""
	if err := service.Send(ctx, input); err != nil {
		t.Fatal(err)
	}
	withoutReasoning := waitMessage(t, service, branch.ID, 6, "complete")
	if withoutReasoning.Text != "先查🙂完成" || withoutReasoning.Details == nil || withoutReasoning.Details.Rounds != 2 ||
		withoutReasoning.Details.Work == nil || len(withoutReasoning.Details.Work.Tools) != 1 ||
		withoutReasoning.Details.Work.Tools[0].Status != "completed" {
		t.Fatalf("valid tool turn did not complete normally: %+v", withoutReasoning)
	}
	initial, continued := <-requests, <-requests
	if !strings.Contains(initial, `"thinking":{"type":"enabled"}`) || !strings.Contains(continued, `"tool_call_id":"call-1"`) ||
		!strings.Contains(continued, "测试文章") || strings.Contains(continued, `"reasoning_content"`) || len(requests) != 0 {
		t.Fatalf("gateway requests changed or repeated: initial=%s continued=%s outstanding=%d", initial, continued, len(requests))
	}
}

func TestChatShowsArticleCompactionRetryAndPersistsRetryCount(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 8*time.Second)
	defer cancel()
	var compression, final atomic.Int32
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		switch {
		case strings.Contains(string(body), `"role":"tool"`):
			final.Add(1)
			// The closing stream marker is deliberately absent after the article tool.
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"未完成的后续回复\"}}]}\n\n")
		case strings.Contains(string(body), "本批原文"):
			if compression.Add(1) > 1 {
				fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"已核对该段原文\"}}]}\n\n")
			}
			fmt.Fprint(w, "data: {\"choices\":[{\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
		default:
			fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"先查正文。","tool_calls":[{"index":0,"id":"read-1","type":"function","function":{"name":"article_get","arguments":"{}"}}]}}]}

data: {"choices":[{"finish_reason":"tool_calls"}]}

data: [DONE]

`)
		}
	}))
	defer gateway.Close()
	db, err := database.Open(ctx, filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if updated, err := db.SavePreferences(ctx, `{"chat_max_retries":1}`, 0, time.Now().UnixMilli()); err != nil || !updated {
		t.Fatalf("retry preference: %v %v", updated, err)
	}
	config := aiconfig.New(db.AIProviderQueries(), nil)
	provider, err := config.SaveProvider(ctx, aiconfig.ProviderInput{Provider: aiconfig.Provider{
		Name: "Gateway", Protocol: "chat_completions", BaseURL: gateway.URL, Enabled: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	model, err := config.SaveModel(ctx, aiconfig.Model{
		ProviderID: provider.ID, Name: "fixture", Enabled: true, Metadata: ai.Metadata{Context: 8192},
	})
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan UpdateEvent, 40)
	service, err := New(ctx, db, config, func(event UpdateEvent) { updates <- event })
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	var reads atomic.Int32
	article := agentrun.NewArticleSource("one", "feed", strings.Repeat("中文正文内容。", 600))
	service.SetToolSource(func(id string, _ []string, _ bool) ([]agentrun.ReadTool, string, error) {
		return []agentrun.ReadTool{{
			Definition: ai.ToolDefinition{Name: "article_get", InputSchema: json.RawMessage(`{"type":"object"}`)},
			Read: func(ctx context.Context, call ai.ToolCall) (string, error) {
				reads.Add(1)
				return agentrun.ResultStoreFrom(ctx).PublishArticle(id, call.Name, call.ArgumentsJSON, article)
			},
			ArticleNext: func(ctx context.Context, call ai.ToolCall, cursor string) (string, error) {
				return agentrun.ResultStoreFrom(ctx).Continue(id, call.Name, cursor, nil)
			},
		}}, "", nil
	})
	conversation, err := service.Create(ctx, Scope{Kind: "global"}, "重试回归")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Send(ctx, SendInput{ConversationID: conversation.ID, ModelID: model.ID, Text: "阅读全文"}); err != nil {
		t.Fatal(err)
	}

	var sawCompactionRetry bool
	for {
		select {
		case <-ctx.Done():
			t.Fatal("waiting for chat completion")
		case event := <-updates:
			if !event.WorkChanged && !event.Done {
				continue
			}
			rows, err := service.Messages(ctx, conversation.ID)
			if err != nil || len(rows) != 2 {
				t.Fatalf("messages: %+v, %v", rows, err)
			}
			reply := rows[1]
			if reply.Details == nil || reply.Details.Work == nil {
				continue
			}
			work := reply.Details.Work
			if work.RetryAttempt == 1 && work.RetryMax == 1 && len(work.Tools) == 1 && work.Tools[0].Status == "running" {
				sawCompactionRetry = reply.Text == "先查正文。" && !event.ResetText
			}
			if event.Done {
				if !sawCompactionRetry || reply.Status != "failed" || reply.ErrorCode != "ProviderProtocolError" ||
					work.RetryCount != 2 || work.RetryAttempt != 0 || reads.Load() != 1 || final.Load() != 2 ||
					len(work.Tools) != 1 || work.Tools[0].Status != "completed" {
					t.Fatalf("retry not preserved: saw=%t reads=%d final=%d reply=%+v work=%+v",
						sawCompactionRetry, reads.Load(), final.Load(), reply, work)
				}
				return
			}
		}
	}
}
