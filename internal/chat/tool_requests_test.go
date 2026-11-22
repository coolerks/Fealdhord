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

	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/appagent"
	"github.com/coolerks/Fealdhord/internal/database"
)

func TestGlobalChatSendsAuthorizedToolsToRoutedDeepSeek(t *testing.T) {
	ctx := t.Context()
	const modelID = "commandcode-plan/deepseek/deepseek-v4.1-flash"
	requests := make(chan string, 3)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		requests <- string(body)
		w.Header().Set("Content-Type", "text/event-stream")
		if strings.Contains(string(body), `"tools":[`) && !strings.Contains(string(body), `"role":"tool"`) {
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call-1\",\"type\":\"function\",\"function\":{\"name\":\"library_search\",\"arguments\":\"{}\"}}]}}]}\n\n")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
			return
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"完成\"}}]}\n\n")
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
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
	appagent.Register(registry, "library.search", "查找本地文章", false,
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

	input.ModelID = model.ID
	if err := service.Send(ctx, input); err != nil {
		t.Fatal(err)
	}
	reply := waitMessage(t, service, conversation.ID, 2, "complete")
	if audit := registry.RecentAudit(); !strings.Contains(reply.ModelName, "关闭思考") ||
		len(audit) != 1 || audit[0].Tool != "library.search" || audit[0].Status != "complete" {
		t.Fatalf("incorrect model mode or tool audit: reply=%+v audit=%+v", reply, audit)
	}
	first, next := <-requests, <-requests
	for _, payload := range []string{first, next} {
		for _, required := range []string{fmt.Sprintf(`"model":%q`, modelID), `"thinking":{"type":"disabled"}`, `"name":"library_search"`, `"parameters":{`, "以本轮实际提供的工具"} {
			if !strings.Contains(payload, required) {
				t.Fatalf("request missing %q: %s", required, payload)
			}
		}
		if strings.Contains(payload, "reasoning_effort") {
			t.Fatalf("non-thinking request still includes effort: %s", payload)
		}
	}
	if !strings.Contains(next, `"tool_call_id":"call-1"`) || !strings.Contains(next, "测试文章") {
		t.Fatalf("tool result not returned to the model: %s", next)
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
}
