package translation

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/reader"
)

func phase1Article(t *testing.T, html string) (string, *database.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "translation.db")
	ctx := context.Background()
	store, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	now := time.Now().UnixMilli()
	_, err = store.AddSubscription(ctx, database.SubscriptionInput{URL: "https://example.test/feed", Title: "test"}, []database.ArticleInput{
		{GUID: "1", Title: "An example", HTML: html, FetchedAt: now},
	}, database.FeedCheck{At: now, Next: now + 60000})
	if err != nil {
		t.Fatal(err)
	}
	articles, err := store.ListArticles(ctx, 1)
	if err != nil || len(articles) != 1 {
		t.Fatalf("articles: %v, error: %v", articles, err)
	}
	return path, store, articles[0].ID
}

func readPhase1Payload(t *testing.T, request *ai.ModelRequest) translationPayload {
	t.Helper()
	var payload translationPayload
	if err := json.Unmarshal([]byte(request.Messages[0].Content[0].Text), &payload); err != nil {
		t.Fatal(err)
	}
	if ai.EstimateRequest(*request) > request.Budget.InputLimit || *request.MaxOutputTokens > request.Budget.OutputReserve {
		t.Fatal("translation exceeded its model budget")
	}
	return payload
}

func phase1ModelReply(t *testing.T, payload translationPayload, finish string) *ai.ModelResponse {
	t.Helper()
	var output strings.Builder
	write := func(item translationItem) {
		line, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		output.Write(line)
		output.WriteByte('\n')
	}
	if payload.Title != "" {
		write(translationItem{ID: "title", Text: "示例标题"})
	}
	for _, item := range payload.Items {
		write(item)
	}
	if finish == "length" {
		output.WriteString(`{"id":"unfinished","text":"未完`)
	}
	return &ai.ModelResponse{FinishReason: finish, Message: ai.Message{Content: []ai.ContentBlock{{Type: "text", Text: output.String()}}}}
}

func TestLongArticleThatFitsUsesOneStream(t *testing.T) {
	paragraph := "This example paragraph contains about one hundred characters and stands in for a section of the article."
	_, store, id := phase1Article(t, strings.Repeat("<p>"+paragraph+"</p>", 429))
	var batches [][]string
	adapter := &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, _ int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		var ids []string
		for i, item := range payload.Items {
			ids = append(ids, item.ID)
			payload.Items[i].Text = "一段简短的测试译文。"
		}
		batches = append(batches, ids)
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	first := 0
	metadata := ai.Metadata{Context: 131072, OutputLimit: 32768}
	svc := New(store, fakeResolver{model: adapter, metadata: metadata}, reader.NewService(store, nil, nil, nil), func(event ProgressEvent) {
		if first == 0 && event.Record.TranslatedBlocks > 0 {
			first = event.Record.TranslatedBlocks
		}
	}, nil)
	out, err := svc.Translate(context.Background(), TranslateRequest{ArticleID: id, SourceMode: "feed", StartBlockID: "b30"})
	if err != nil || out.Status != "completed" || out.TranslatedBlocks != 429 || first != 1 {
		t.Fatalf("translation status=%s, blocks=%d, first=%d, err=%v", out.Status, out.TranslatedBlocks, first, err)
	}
	if len(batches) != 1 || len(batches[0]) != 429 || batches[0][0] != "b0" || batches[0][428] != "b428" {
		t.Fatalf("unexpected batch plan: %v", batches)
	}
}

func TestTranslationRestartsAtMissingFragmentsAndPreservesMarkup(t *testing.T) {
	link := strings.Repeat("a", 64)
	metadata := ai.Metadata{Context: 4096, OutputLimit: 1024}
	original := `<a href="#reader-link" data-reader-link="` + link + `"><strong>` +
		strings.Repeat("This is a sentence with &amp; characters and several words. ", 300) +
		`</strong></a> <code>&lt;escaped&gt; &amp; marker()</code>`
	ctx := context.Background()
	path, store, id := phase1Article(t, "<p>"+original+"</p>")
	seen := make(map[string]bool)
	adapter := &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, call int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		if len(seen) > 0 {
			return nil, &ai.ModelError{Code: "RateLimitError"}
		}
		for i, item := range payload.Items {
			payload.Items[i].Text = strings.ReplaceAll(item.Text, "This is", "译 This is")
			seen[item.ID] = true
		}
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	req := TranslateRequest{ArticleID: id, SourceMode: "feed", TargetLang: "zh-CN"}
	svc := New(store, fakeResolver{model: adapter, metadata: metadata}, reader.NewService(store, nil, nil, nil), nil, nil)
	first, err := svc.Translate(ctx, req)
	if modelErrorCode(err) != "RateLimitError" || first.Status != "partial" || first.TotalBlocks != 1 || first.TranslatedBlocks != 0 || len(seen) == 0 {
		t.Fatalf("fragment checkpoint: %+v %v sent=%v", first, err, seen)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	adapter = &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, _ int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		for i, item := range payload.Items {
			payload.Items[i].Text = strings.ReplaceAll(item.Text, "This is", "译 This is")
			if seen[item.ID] {
				t.Errorf("previously committed fragment sent again: %s", item.ID)
			}
		}
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	svc = New(reopened, fakeResolver{model: adapter, metadata: metadata}, reader.NewService(reopened, nil, nil, nil), nil, nil)
	out, err := svc.Translate(ctx, req)
	if err != nil || out.Status != "completed" || out.TranslatedBlocks != 1 {
		t.Fatalf("resume status=%s, blocks=%d, error=%v", out.Status, out.TranslatedBlocks, err)
	}
	translated := out.Blocks[0].TranslatedHTML
	if strings.Count(translated, "<a ") != 1 || strings.Count(translated, "<strong>") != 1 ||
		strings.Count(translated, "This is a sentence") != 300 || !strings.Contains(translated, "<code>&lt;escaped&gt; &amp; marker()</code>") {
		t.Fatalf("unexpected merged structure: %.180s…%.90s", translated, translated[max(0, len(translated)-150):])
	}
}

func TestTranslationRefreshRejectsInFlightResultAndReusesMatchingBlocks(t *testing.T) {
	_, store, id := phase1Article(t, "<p>Subscription preview.</p>")
	ctx := context.Background()
	article, err := store.GetArticleForTranslation(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	save := func(html string) {
		t.Helper()
		err := store.SaveReaderDocument(ctx, id, database.ReaderDocument{HTML: html, Hash: article.ContentHash,
			Version: "go-readability/v2.1.3", URL: "https://example.test/article"})
		if err != nil {
			t.Fatal(err)
		}
	}
	save("<p>Unchanged original.</p><p>Old original.</p>")
	readerSvc := reader.NewService(store, nil, nil, nil)
	adapter := &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, call int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		if call > 1 && (len(payload.Items) != 1 || payload.Items[0].ID != "b1") {
			t.Fatalf("retry should send just the changed paragraph: %+v", payload.Items)
		}
		if call == 2 {
			save("<p>Unchanged original.</p><p>Latest original.</p>")
		}
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	svc := New(store, fakeResolver{model: adapter}, readerSvc, nil, nil)
	req := TranslateRequest{ArticleID: id, SourceMode: "reader", TargetLang: "zh-CN"}
	first, err := svc.Translate(ctx, req)
	if err != nil || first.SourceVersion == "" || first.TranslatedBlocks != 2 {
		t.Fatalf("first source: %+v %v", first, err)
	}
	save("<p>Unchanged original.</p><p>Intermediate original.</p>")
	if _, err := svc.Translate(ctx, req); modelErrorCode(err) != "Conflict" {
		t.Fatalf("in-flight change should conflict: %v", err)
	}
	current, err := svc.Get(ctx, id, req.TargetLang, req.SourceMode)
	if err != nil || current.SourceVersion == first.SourceVersion || current.TranslatedBlocks != 1 {
		t.Fatalf("current cached source: %+v, %v", current, err)
	}
	out, err := svc.Translate(ctx, req)
	if err != nil || out.TranslatedBlocks != 2 || out.Blocks[1].TranslatedHTML != "Latest original." {
		t.Fatalf("latest source: %+v, %v", out, err)
	}
}

func TestTranslationTruncatedResponseRetriesOnlyUnverifiedTail(t *testing.T) {
	_, store, id := phase1Article(t, "<p>One paragraph.</p><p>The next paragraph.</p>")
	var batches [][]string
	var firstOutputLimit int
	adapter := &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, call int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		ids := make([]string, len(payload.Items))
		for i, item := range payload.Items {
			ids[i] = item.ID
		}
		batches = append(batches, ids)
		if call == 1 {
			firstOutputLimit = *request.MaxOutputTokens
			payload.Items = payload.Items[:1]
			return phase1ModelReply(t, payload, "length"), nil
		}
		if *request.MaxOutputTokens <= firstOutputLimit {
			t.Fatal("repair should reserve more response tokens")
		}
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	svc := New(store, fakeResolver{model: adapter}, reader.NewService(store, nil, nil, nil), nil, nil)
	out, err := svc.Translate(context.Background(), TranslateRequest{ArticleID: id, SourceMode: "feed"})
	if err != nil || out.TranslatedBlocks != 2 || len(batches) != 2 || !slices.Equal(batches[1], []string{"b1"}) {
		t.Fatalf("truncated batch recovery: blocks=%d batches=%v err=%v", out.TranslatedBlocks, batches, err)
	}
}

func TestTranslationCancelDiscardsLateReply(t *testing.T) {
	_, store, id := phase1Article(t, strings.Repeat("<p>Original paragraph.</p>", 12))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	model := &fakeModel{stream: func(streamCtx context.Context, _ *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
		events := make(chan ai.ModelEvent, 2)
		events <- ai.ModelEvent{Type: "text_delta", Text: "{\"id\":\"b0\",\"text\":\"第一段\"}\n{\"id\":\"b1\",\"text\":\"第二段\"}\n{\"id\":\"b2\",\"text\":\"第三段\"}\n"}
		go func() {
			defer close(events)
			<-streamCtx.Done()
			events <- ai.ModelEvent{Type: "text_delta", Text: "{\"id\":\"b3\",\"text\":\"迟到\"}\n"}
		}()
		return events, nil
	}}
	svc := New(store, fakeResolver{model: model}, reader.NewService(store, nil, nil, nil), func(event ProgressEvent) {
		if event.Record.TranslatedBlocks == 3 {
			cancel()
		}
	}, nil)
	req := TranslateRequest{ArticleID: id, SourceMode: "feed", TargetLang: "zh-CN"}
	out, err := svc.Translate(ctx, req)
	if modelErrorCode(err) != "Cancelled" || out.Status != "partial" || out.TranslatedBlocks != 3 {
		t.Fatalf("cancellation should retain only committed progress: %+v %v", out, err)
	}
	persisted, err := svc.Get(context.Background(), id, req.TargetLang, req.SourceMode)
	if err != nil || persisted.Status != "partial" || persisted.TranslatedBlocks != 3 {
		t.Fatalf("late reply entered the cache: %+v %v", persisted, err)
	}
}

func TestTranslationSplitsToTheModelsOutputBudget(t *testing.T) {
	p := batchPlanner{model: "small", system: "Translate.", budget: ai.ContextBudget(ai.Metadata{
		Context: 2048, InputLimit: 400, OutputLimit: 128,
	})}
	block := SourceBlock{ID: "b0", SourceHash: "long", InnerHTML: strings.Repeat("A long sentence that has several words. ", 35)}
	plan, err := p.split(block)
	if err != nil || len(plan.units) < 2 {
		t.Fatalf("expected a split under a small output budget: units=%d err=%v", len(plan.units), err)
	}
	for _, unit := range plan.units {
		req, fits := p.request([]translationUnit{unit}, false, "")
		if !fits || *req.MaxOutputTokens > 128 || ai.EstimateRequest(*req) > 400 {
			t.Fatalf("fragment exceeds provider metadata: %+v", req)
		}
	}
}

func TestTranslationContinuesLaterBatchesAfterAnUnrecoverableBlock(t *testing.T) {
	_, store, id := phase1Article(t, strings.Repeat("<p>Original paragraph.</p>", 66))
	var faultyAttempts int
	model := &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, _ int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		for i, item := range payload.Items {
			if item.ID == "b1" {
				faultyAttempts++
				payload.Items = slices.Delete(payload.Items, i, i+1)
				break
			}
		}
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	svc := New(store, fakeResolver{model: model}, reader.NewService(store, nil, nil, nil), nil, nil)
	out, err := svc.Translate(context.Background(), TranslateRequest{ArticleID: id, SourceMode: "feed"})
	if modelErrorCode(err) != "InvalidModelOutput" || out.Status != "partial" || out.TranslatedBlocks != 65 || faultyAttempts != 2 {
		t.Fatalf("local failure stopped independent batches: %s %d/66 attempts=%d err=%v", out.Status, out.TranslatedBlocks, faultyAttempts, err)
	}
	if out.Blocks[64].ID != "b65" {
		t.Fatal("last paragraph did not reach the cache")
	}
}
