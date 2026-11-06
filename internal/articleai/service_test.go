package articleai

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database"
)

type mockResolver struct{ model ai.Model }

func (m mockResolver) ResolveArticleModel(_ context.Context, _ string, id string, revision, _ int64) (aiconfig.Model, ai.Model, error) {
	return aiconfig.Model{ID: id, Name: "mock", Revision: revision}, m.model, nil
}

type mockModel func(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error)

func (m mockModel) Generate(ctx context.Context, request *ai.ModelRequest) (*ai.ModelResponse, error) {
	return m(ctx, request)
}

func (m mockModel) Stream(context.Context, *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
	panic("article evaluation must use one non-streaming request")
}

func textResponse(text string) *ai.ModelResponse {
	return &ai.ModelResponse{FinishReason: "stop", Message: ai.Message{Content: []ai.ContentBlock{{Type: "text", Text: text}}}}
}

func TestArticleAIWorkflow(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "local.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	config := aiconfig.New(store.AIProviderQueries(), nil)
	provider, err := config.SaveProvider(ctx, aiconfig.ProviderInput{Provider: aiconfig.Provider{Name: "local", Protocol: "responses", BaseURL: "https://example.test", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	model, err := config.SaveModel(ctx, aiconfig.Model{ProviderID: provider.ID, Name: "mock", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range []string{"classification", "featured"} {
		if err := config.Assign(ctx, task, model.ID); err != nil {
			t.Fatal(err)
		}
	}
	feed, err := store.EnsureFeed(ctx, "https://example.test/feed", "source")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	first := database.ArticleInput{FeedID: feed, GUID: "a", Title: "First", Summary: "summary", Text: "local article", FetchedAt: now.UnixMilli()}
	id, err := store.UpsertArticle(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: feed, GUID: "b", Title: "Second", FetchedAt: now.Add(time.Minute).UnixMilli()})
	if err != nil {
		t.Fatal(err)
	}
	category, err := store.SaveCategory(ctx, database.CategoryInput{Name: "Reading"})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	mutate, invalid := false, false
	service := New(store, mockResolver{model: mockModel(func(ctx context.Context, request *ai.ModelRequest) (*ai.ModelResponse, error) {
		calls++
		if mutate {
			mutate = false
			first.Text = "changed during request"
			if _, err := store.UpsertArticle(ctx, first); err != nil {
				t.Fatal(err)
			}
		}
		if invalid {
			return textResponse(`{"categories":[{"id":"foreign","confidence":0.9}]}`), nil
		}
		var payload struct {
			Title      string
			Categories []any
		}
		if err := json.Unmarshal([]byte(request.Messages[0].Content[0].Text), &payload); err != nil {
			t.Fatal(err)
		}
		if payload.Categories == nil {
			if payload.Title == "First" {
				return textResponse(`{"importance":0.9,"relevance":0.9,"reason":"high"}`), nil
			}
			return textResponse(`{"importance":0.7,"relevance":0.6,"reason":"threshold"}`), nil
		}
		return textResponse(`{"categories":[{"id":"` + category + `","confidence":0.9,"reason":"match"}]}`), nil
	})}, nil)
	input := Request{ArticleID: id, Task: "classification"}
	if _, err := service.Evaluate(ctx, input); err == nil || err.Error() != "Disabled" || calls != 0 {
		t.Fatalf("disabled: %v, %d", err, calls)
	}
	input.Preview = true
	result, err := service.Evaluate(ctx, input)
	if err != nil || len(result.Matches) != 1 {
		t.Fatalf("preview: %+v %v", result, err)
	}
	if rows, err := store.ArticleMemberships(ctx, id); err != nil || len(rows) != 1 || rows[0].Effective {
		t.Fatalf("preview wrote membership: %+v %v", rows, err)
	}
	settings, err := service.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Classification, settings.Featured = true, true
	if _, err := service.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	input.Preview = false
	if err := store.SetMembership(ctx, id, category, "manual", "excluded", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Evaluate(ctx, input); err != nil {
		t.Fatal(err)
	}
	if rows, err := store.ArticleMemberships(ctx, id); err != nil || len(rows) != 1 || rows[0].Effective {
		t.Fatalf("AI overrode manual exclusion: %+v %v", rows, err)
	}
	if result, err := service.Evaluate(ctx, input); err != nil || !result.Cached || calls != 2 {
		t.Fatalf("cache: %+v %v calls=%d", result, err, calls)
	}
	first.Text = "invalidates cache"
	if _, err := store.UpsertArticle(ctx, first); err != nil {
		t.Fatal(err)
	}
	mutate = true
	if _, err := service.Evaluate(ctx, input); err == nil || err.Error() != "Conflict" {
		t.Fatalf("stale: %v", err)
	}
	invalid = true
	if _, err := service.Evaluate(ctx, input); err == nil || err.Error() != "InvalidModelOutput" || calls != 5 {
		t.Fatalf("invalid: %v calls=%d", err, calls)
	}
	invalid = false
	for _, article := range []string{id, second} {
		if _, err := service.Evaluate(ctx, Request{ArticleID: article, Task: "featured"}); err != nil {
			t.Fatal(err)
		}
	}
	query := database.ArticleQuery{Scope: database.ArticleScope{Kind: "featured"}, Limit: 1}
	page, err := store.QueryArticles(ctx, query)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != id || page.NextCursor == "" {
		t.Fatalf("ranked page: %+v %v", page, err)
	}
	query.Cursor = page.NextCursor
	page, err = store.QueryArticles(ctx, query)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != second {
		t.Fatalf("next ranked page: %+v %v", page, err)
	}
	if candidate, err := store.NextArticleAI(ctx, "featured", now.Add(-time.Hour)); err != nil || candidate != "" {
		t.Fatalf("completed candidate: %q %v", candidate, err)
	}
	settings, err = service.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.MinimumImportance = 0.8
	settings.TokensPerHour = 100000
	if _, err := service.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	if _, err := store.QueryArticles(ctx, query); err != database.ErrCursorExpired {
		t.Fatalf("stale threshold cursor: %v", err)
	}
	query.Cursor = ""
	page, err = store.QueryArticles(ctx, query)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != id || page.NextCursor != "" {
		t.Fatalf("threshold: %+v %v", page, err)
	}
	before := calls
	if result, err := service.Evaluate(ctx, Request{ArticleID: second, Task: "featured"}); err != nil || !result.Cached || calls != before {
		t.Fatalf("threshold invalidated cache: %+v %v", result, err)
	}

	snapshot, err := store.ArticleAISnapshot(ctx, id, "featured", false)
	if err != nil {
		t.Fatal(err)
	}
	later := now.Add(2 * time.Hour)
	if err := store.ReserveArticleAI(ctx, snapshot, false, 100000, later); err != nil {
		t.Fatal(err)
	}
	if err := store.ReserveArticleAI(ctx, snapshot, false, 1, later); err != database.ErrArticleAIBudget {
		t.Fatalf("over budget: %v", err)
	}
	if err := store.ReserveArticleAI(ctx, snapshot, false, 1, later.Add(time.Hour)); err != nil {
		t.Fatalf("expired budget: %v", err)
	}

}
