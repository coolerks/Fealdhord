package database_test

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/appagent"
	"github.com/coolerks/Fealdhord/internal/chat"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/library"
)

func TestAIRepairRestoresChatAndArticleEvaluationOnVersion21Database(t *testing.T) {
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "library.db")
	initial, err := database.Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	feed, err := initial.AddSubscription(ctx, database.SubscriptionInput{Title: "Feed", URL: "https://example.test/feed"}, nil, database.FeedCheck{At: time.Now().UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	article, err := initial.UpsertArticle(ctx, database.ArticleInput{FeedID: feed, GUID: "original", Title: "旧文章", FetchedAt: time.Now().UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}

	old, err := sql.Open("sqlite", filename)
	if err != nil {
		t.Fatal(err)
	}
	// Reproduce the schema observed in the dev database, including the recorded old migrations.
	_, err = old.ExecContext(ctx, `
		DELETE FROM goose_db_version WHERE version_id>21;
		ALTER TABLE messages DROP COLUMN reply_details;
		ALTER TABLE conversations DROP COLUMN auto_compact;
		ALTER TABLE article_ai_results DROP COLUMN profile_key;
		ALTER TABLE article_ai_results DROP COLUMN reason;
		DROP TABLE article_ai_deferrals;
		INSERT INTO conversations (id,scope_type,scope_id,title,created_at,updated_at,context_summary)
		VALUES ('old','global','','旧会话',1,1,'保留摘要');
		INSERT INTO messages (id,conversation_id,position,role,content,status)
		VALUES ('old-message','old',1,'assistant','保留旧历史','complete');
		INSERT INTO article_ai_results (article_id,task,input_key,content_hash,model_id,model_revision,provider_revision,
			settings_revision,importance,relevance,evaluated_at)
		VALUES (?,'featured','old-key','old-hash','old-model',1,1,1,0.9,0.8,1);
	`, article)
	closeErr := old.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("prepare v21 database: %v, %v", err, closeErr)
	}

	store, err := database.Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	backups, err := filepath.Glob(filepath.Join(filename+".backups", "before-v21-*.db"))
	if err != nil || len(backups) != 1 {
		t.Fatalf("pre-upgrade backup: %v, %v", backups, err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			t.Errorf("model request: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if !strings.Contains(string(body), `"role":"tool"`) {
			if !strings.Contains(string(body), `"name":"article_search"`) {
				t.Error("model request is missing the article search tool")
			}
			fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"search","type":"function","function":{"name":"article_search","arguments":"{\"search\":\"旧文章\"}"}}]}}]}

data: {"choices":[{"index":0,"finish_reason":"tool_calls"}]}

data: [DONE]

`)
			return
		}
		if !strings.Contains(string(body), article) {
			t.Error("tool result is missing the original article")
		}
		fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"回复已保存\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	config := aiconfig.New(store.AIProviderQueries(), nil)
	provider, err := config.SaveProvider(ctx, aiconfig.ProviderInput{Provider: aiconfig.Provider{
		Name: "测试服务", Protocol: "chat_completions", BaseURL: server.URL, Enabled: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	model, err := config.SaveModel(ctx, aiconfig.Model{ProviderID: provider.ID, Name: "test", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Assign(ctx, "featured", model.ID); err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{}, 1)
	service, err := chat.New(ctx, store, config, func(event chat.UpdateEvent) {
		if event.Done {
			done <- struct{}{}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	platform := appagent.NewPlatform(appagent.Builtins(appagent.Services{Store: store, Library: library.NewService(store)}), t.TempDir())
	defer platform.Shutdown()
	service.SetToolSource(platform.RunTools)
	api := chat.NewAPI(service)
	scope := chat.Scope{Kind: "global"}
	list, err := api.List(ctx, scope)
	if err != nil || len(list) != 1 || list[0].ID != "old" {
		t.Fatalf("legacy conversation list: %+v, %v", list, err)
	}
	history, err := api.Messages(ctx, "old")
	if err != nil || len(history) != 1 || history[0].Text != "保留旧历史" {
		t.Fatalf("legacy history: %+v, %v", history, err)
	}
	oldConversation, err := store.ChatQueries().GetConversation(ctx, "old")
	if err != nil || oldConversation.ContextSummary != "保留摘要" || oldConversation.AutoCompact != 0 {
		t.Fatalf("legacy context: %+v, %v", oldConversation, err)
	}

	if err := api.SetAutomaticCompaction(ctx, "old", true); err != nil {
		t.Fatal(err)
	}
	status, err := api.ContextStatus(ctx, "old", model.ID)
	if err != nil || !status.AutoCompact || status.Summary != "保留摘要" {
		t.Fatalf("updated context: %+v, %v", status, err)
	}

	newChat, err := api.Create(ctx, scope, "新会话")
	if err != nil {
		t.Fatal(err)
	}
	if err := api.Send(ctx, chat.SendInput{ConversationID: newChat.ID, ModelID: model.ID, Text: "测试发送"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("model stream did not finish")
	}
	history, err = api.Messages(ctx, newChat.ID)
	if err != nil || len(history) != 2 || history[1].Status != "complete" || history[1].Text != "回复已保存" {
		t.Fatalf("new conversation: %+v, %v", history, err)
	}

	snapshot, err := store.ArticleAISnapshot(ctx, article, "featured", true)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := store.ArticleAICached(ctx, snapshot)
	if err != nil || cached {
		t.Fatalf("legacy score refresh: cached %t, %v", cached, err)
	}
	settings, err := store.ArticleAISettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.SaveArticleAISettings(ctx, sqlc.SaveArticleAISettingsParams{
		Featured: 1, RequestsPerHour: settings.RequestsPerHour, TokensPerHour: settings.TokensPerHour,
		MinimumImportance: settings.MinimumImportance, MinimumRelevance: settings.MinimumRelevance, Revision: settings.Revision,
	}); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().Add(-time.Hour)
	if candidate, err := store.NextArticleAI(ctx, "featured", cutoff); err != nil || candidate != article {
		t.Fatalf("featured candidate: %q, %v", candidate, err)
	}
	if err := store.DeferArticleAI(ctx, article, "featured", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if candidate, err := store.NextArticleAI(ctx, "featured", cutoff); err != nil || candidate != "" {
		t.Fatalf("deferred candidate: %q, %v", candidate, err)
	}
	readOnly, err := sql.Open("sqlite", "file:"+filename+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer readOnly.Close()
	var score float64
	err = readOnly.QueryRowContext(ctx, "SELECT importance FROM article_ai_results WHERE article_id=? AND task='featured'", article).Scan(&score)
	if err != nil || score != 0.9 {
		t.Fatalf("legacy featured score: %v, %v", score, err)
	}
}
