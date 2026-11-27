package translation

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/articleai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/reader"
)

type testMTEngine struct {
	caps EngineCapabilities
	run  func(context.Context, BatchRequest) (BatchResult, error)
}

func (e testMTEngine) ID() EngineID                             { return EngineBaidu }
func (e testMTEngine) Capabilities() EngineCapabilities         { return e.caps }
func (e testMTEngine) SupportsLanguagePair(string, string) bool { return true }
func (e testMTEngine) TranslateBatch(ctx context.Context, req BatchRequest) (BatchResult, error) {
	return e.run(ctx, req)
}

func selectMTProvider(t *testing.T, store *database.Store, id EngineID) {
	t.Helper()
	ctx := context.Background()
	settingsService := articleai.New(store, nil, nil)
	settings, err := settingsService.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.TranslationEngine = string(id)
	if _, err := settingsService.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
}

func testMTService(store *database.Store, engine Engine, emit func(ProgressEvent)) *Service {
	s := New(store, nil, reader.NewService(store, nil, nil, nil), emit, nil)
	s.engineFactory = func(context.Context, EngineID) (Engine, error) { return engine, nil }
	return s
}

func TestIncompleteMTSummaryRemainsRetryable(t *testing.T) {
	ctx := context.Background()
	_, store, _ := phase1Article(t, "<p>Filler.</p>")
	selectMTProvider(t, store, EngineBaidu)
	now := time.Now().UnixMilli()
	_, err := store.AddSubscription(ctx, database.SubscriptionInput{
		URL: "https://example.test/review", Title: "Review", AutoTranslateMode: "on",
	}, []database.ArticleInput{{
		GUID: "summary", Title: "An article with a summary", Summary: "This is the summary with the details.",
		HTML: "<p>This is the article body.</p>", FetchedAt: now,
	}}, database.FeedCheck{At: now, Next: now + 60000})
	if err != nil {
		t.Fatal(err)
	}
	id, _, _, err := store.NextArticleForTranslation(ctx, time.Now().Add(-time.Hour))
	if err != nil || id == "" {
		t.Fatalf("candidate %q: %v", id, err)
	}
	article, err := store.GetArticleForTranslation(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	bodySvc := testMTService(store, testMTEngine{run: func(context.Context, BatchRequest) (BatchResult, error) {
		return BatchResult{Translations: []string{"译文"}}, nil
	}}, nil)
	if _, err := bodySvc.Translate(ctx, TranslateRequest{ArticleID: id, TargetLang: "zh-CN", SourceMode: "feed"}); err != nil {
		t.Fatal(err)
	}
	if cached, err := bodySvc.Get(ctx, id, "zh-CN", "summary"); err != nil || cached.Status != "pending" || cached.TranslatedTitle != "译文" {
		t.Fatalf("title-only cache must remain pending: %+v, %v", cached, err)
	}
	if listed, err := store.ArticleTranslationSummaries(ctx, []string{id}); err != nil || listed[id].Title != "译文" {
		t.Fatalf("pending summary hid the available title: %+v, %v", listed, err)
	}
	svc := New(store, nil, nil, nil, nil)
	rec := Record{ArticleID: id, SourceMode: "summary", TargetLang: "zh-CN", ContentHash: article.ContentHash,
		TranslatedTitle: "标题", Status: "completed", ModelID: "mt:baidu", Blocks: []BlockTranslation{}, UpdatedAt: now}
	if err := svc.saveRecord(ctx, rec, sql.NullInt64{}, sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	if got, err := svc.Get(ctx, id, rec.TargetLang, rec.SourceMode); err != nil || got.Status != "partial" {
		t.Fatalf("legacy incomplete summary should be retryable: %+v, %v", got, err)
	}
	if candidate, _, _, err := store.NextArticleForTranslation(ctx, time.Now().Add(-time.Hour)); err != nil || candidate != id {
		t.Fatalf("legacy incomplete summary missing from queue: %q, %v", candidate, err)
	}

	tests := []struct {
		name  string
		reply BatchResult
		want  string
	}{
		{"provider failure", BatchResult{Translations: []string{"标题", ""}, FailedIndices: map[int]error{1: &ai.ModelError{Code: "RateLimitError"}}}, "RateLimitError"},
		{"blank output", BatchResult{Translations: []string{"标题", " "}}, "InvalidModelOutput"},
		{"missing output", BatchResult{Translations: []string{"标题"}}, "InvalidModelOutput"},
		{"complete output", BatchResult{Translations: []string{"标题", "摘要"}}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := testMTService(store, testMTEngine{run: func(context.Context, BatchRequest) (BatchResult, error) {
				return tc.reply, nil
			}}, nil)
			got, err := svc.Translate(ctx, TranslateRequest{ArticleID: id, TargetLang: rec.TargetLang, SourceMode: "summary"})
			if tc.want != "" {
				if err == nil || modelErrorCode(err) != tc.want {
					t.Fatalf("wanted %s, got %+v, %v", tc.want, got, err)
				}
				if cached, err := svc.Get(ctx, id, rec.TargetLang, rec.SourceMode); err != nil || cached.Status != "partial" {
					t.Fatalf("failure should leave cached title retryable: %+v, %v", cached, err)
				}
			} else if err != nil || got.Status != "completed" || got.TranslatedSummary != "摘要" {
				t.Fatalf("wanted completed summary, got %+v, %v", got, err)
			}
		})
	}
}

func TestMTBodyRestartsAtMissingFragmentWithMarkup(t *testing.T) {
	ctx := context.Background()
	original := `<a href="https://example.test/manual"><strong>` +
		strings.Repeat("Read the documentation to configure the application. ", 10) +
		`</strong></a> Run <code>go test ./...</code> before you finish. Tail sentence should initially fail.`
	path, store, id := phase1Article(t, "<p>"+original+"</p>")
	selectMTProvider(t, store, EngineBaidu)
	firstRun := true
	sent := []string{}
	engine := testMTEngine{
		caps: EngineCapabilities{MaxBatchItems: 1, MaxBatchChars: 115, MaxSingleItemChars: 115},
		run: func(_ context.Context, req BatchRequest) (BatchResult, error) {
			text := req.Texts[0]
			sent = append(sent, text)
			if strings.Contains(text, "go test") || strings.Contains(text, "example.test") {
				t.Fatalf("protected inline content sent upstream: %q", text)
			}
			if firstRun && strings.Contains(text, "Tail sentence") {
				return BatchResult{}, &ai.ModelError{Code: "ProviderConnectionError"}
			}
			return BatchResult{Translations: []string{strings.ReplaceAll(text, "documentation", "文档")}}, nil
		},
	}
	req := TranslateRequest{ArticleID: id, TargetLang: "zh-CN", SourceMode: "feed"}
	svc := testMTService(store, engine, nil)
	partial, err := svc.Translate(ctx, req)
	if modelErrorCode(err) != "ProviderConnectionError" || partial.Status != "partial" || partial.TranslatedBlocks != 0 {
		t.Fatalf("expected checkpoint-only partial translation, got %+v, %v (%v)", partial, err, sent)
	}
	firstCalls := slices.Clone(sent)
	if len(firstCalls) < 3 {
		t.Fatalf("long block did not split: %v", firstCalls)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	firstRun, sent = false, nil
	svc = testMTService(reopened, engine, nil)
	complete, err := svc.Translate(ctx, req)
	if err != nil || complete.Status != "completed" || len(complete.Blocks) != 1 {
		t.Fatalf("resume failed: %+v, %v (%v)", complete, err, sent)
	}
	if !slices.Equal(sent, firstCalls[len(firstCalls)-1:]) {
		t.Fatalf("resume should send only the unsuccessful fragment, was %v", sent)
	}
	got := complete.Blocks[0].TranslatedHTML
	if strings.Count(got, "<a ") != 1 || !strings.Contains(got, "data-reader-link=") ||
		!strings.Contains(got, "<code>go test ./...</code>") || strings.Count(got, "文档") != 10 {
		t.Fatalf("merged output did not preserve all content and markup: %s", got)
	}
	cached, err := svc.Get(ctx, id, req.TargetLang, req.SourceMode)
	if err != nil || cached.Status != "completed" || len(cached.Blocks) != 1 || cached.Blocks[0].TranslatedHTML != got {
		t.Fatalf("completed cache did not survive validation: %+v, %v", cached, err)
	}
}

func TestMTBodyRejectsDroppedMarkersAndRepairsLegacyCache(t *testing.T) {
	ctx := context.Background()
	_, store, id := phase1Article(t, "<p>Use <code>npm run build</code> to build.</p><p>Another paragraph.</p>")
	selectMTProvider(t, store, EngineBaidu)
	invalid := true
	sent := []string{}
	engine := testMTEngine{run: func(_ context.Context, req BatchRequest) (BatchResult, error) {
		sent = append(sent, req.Texts...)
		result := BatchResult{Translations: make([]string, len(req.Texts))}
		for i, input := range req.Texts {
			if invalid && strings.Contains(input, "to build") {
				result.Translations[i] = "译文缺少代码"
			} else {
				result.Translations[i] = "译文 " + input
			}
		}
		return result, nil
	}}
	svc := testMTService(store, engine, nil)
	req := TranslateRequest{ArticleID: id, TargetLang: "zh-CN", SourceMode: "feed"}
	partial, err := svc.Translate(ctx, req)
	if modelErrorCode(err) != "InvalidModelOutput" || partial.Status != "partial" || len(partial.Blocks) != 1 {
		t.Fatalf("unverified code markers should not complete: %+v, %v", partial, err)
	}
	detail := svc.reader.Get(ctx, reader.Request{ArticleID: id, Mode: req.SourceMode})
	sources, err := ExtractBlocks(detail.Data.HTML)
	if err != nil || len(sources) != 2 {
		t.Fatalf("extract: %v, %v", sources, err)
	}
	legacy := partial
	legacy.Blocks = append(legacy.Blocks, BlockTranslation{ID: sources[0].ID, SourceHash: sources[0].SourceHash, TranslatedHTML: "旧译文缺少代码"})
	legacy.Status, legacy.ErrorCode, legacy.TranslatedBlocks = "completed", "", 2
	if err := svc.saveRecord(ctx, legacy, sql.NullInt64{}, sql.NullInt64{}); err != nil {
		t.Fatal(err)
	}
	if cached, err := svc.Get(ctx, id, req.TargetLang, req.SourceMode); err != nil || cached.Status != "partial" || len(cached.Blocks) != 1 {
		t.Fatalf("legacy cache should expose only verified paragraphs: %+v, %v", cached, err)
	}
	invalid, sent = false, nil
	repaired, err := svc.Translate(ctx, req)
	if err != nil || repaired.Status != "completed" || len(repaired.Blocks) != 2 ||
		!strings.Contains(repaired.Blocks[0].TranslatedHTML, "<code>npm run build</code>") || len(sent) != 1 {
		t.Fatalf("failed to repair exactly the affected block: %+v, %v (%v)", repaired, err, sent)
	}
}

func TestMTBodyDoesNotRestoreTranslationAfterClear(t *testing.T) {
	ctx := context.Background()
	_, store, id := phase1Article(t, "<p>This is a sample article.</p>")
	selectMTProvider(t, store, EngineBaidu)
	req := TranslateRequest{ArticleID: id, TargetLang: "ja", SourceMode: "feed"}
	var svc *Service
	events := []ProgressEvent{}
	engine := testMTEngine{run: func(callCtx context.Context, req BatchRequest) (BatchResult, error) {
		if err := svc.Clear(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
		if !errors.Is(callCtx.Err(), context.Canceled) {
			t.Fatal("clearing all languages did not cancel the active Japanese translation")
		}
		return BatchResult{Translations: []string{"遅い応答"}}, nil
	}}
	svc = testMTService(store, engine, func(event ProgressEvent) { events = append(events, event) })
	if _, err := svc.Translate(ctx, req); modelErrorCode(err) != "Cancelled" {
		t.Fatalf("expected cancelled call, got %v", err)
	}
	if _, err := store.GetArticleTranslation(ctx, id, req.TargetLang, req.SourceMode); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("cleared row was restored: %v", err)
	}
	if len(events) != 1 || events[0].Record.Status != "translating" {
		t.Fatalf("cleared task emitted stale progress: %+v", events)
	}
}

func TestBaiduSingleRetriesShareTheServiceRateLimit(t *testing.T) {
	for _, cancelBeforeRetry := range []bool{false, true} {
		name := "throttles retries"
		if cancelBeforeRetry {
			name = "does not send after cancellation"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls []time.Time
			client := mockHTTPClient(func(req *http.Request) (*http.Response, error) {
				calls = append(calls, time.Now())
				if err := req.ParseForm(); err != nil {
					t.Fatal(err)
				}
				src := req.Form.Get("q")
				if len(calls) == 1 {
					src = "only an unmatched source"
					if cancelBeforeRetry {
						cancel()
					}
				}
				return jsonResponse(http.StatusOK, map[string]any{
					"trans_result": []map[string]string{{"src": src, "dst": "译文"}},
				}), nil
			})
			engine, err := newBaiduEngine(resolvedProviderConfig{AppID: "dummy", Secret: "dummy", Edition: "standard"}, client)
			if err != nil {
				t.Fatal(err)
			}
			svc := New(nil, nil, nil, nil, nil)
			res, err := svc.callEngineWithLimit(ctx, engine, BatchRequest{Texts: []string{"First paragraph.", "Second paragraph."}, TargetLang: "zh-CN"}, false)
			if cancelBeforeRetry {
				if modelErrorCode(err) != "Cancelled" || len(calls) != 1 {
					t.Fatalf("cancel sent an additional request: %d calls, %v", len(calls), err)
				}
				return
			}
			if err != nil || len(calls) != 3 || !slices.Equal(res.Translations, []string{"译文", "译文"}) {
				t.Fatalf("unexpected retries: %d calls, %+v, %v", len(calls), res, err)
			}
			for i := 1; i < len(calls); i++ {
				if elapsed := calls[i].Sub(calls[i-1]); elapsed < time.Second {
					t.Fatalf("retry bypassed standard rate limit, gap %v", elapsed)
				}
			}
		})
	}
}

func TestWorkerContinuesAfterUnsupportedArticle(t *testing.T) {
	ctx := context.Background()
	_, store, _ := phase1Article(t, "<p>Filler.</p>")
	selectMTProvider(t, store, EngineBaidu)
	now := time.Now().UTC().UnixMilli()
	older := now - 60_000
	_, err := store.AddSubscription(ctx, database.SubscriptionInput{
		URL: "https://example.test/worker", Title: "Worker", AutoTranslateMode: "on",
	}, []database.ArticleInput{
		{GUID: "older", Title: "Supported article", PublishedAt: &older, FetchedAt: now},
		{GUID: "newer", Title: "Unsupported article", PublishedAt: &now, FetchedAt: now},
	}, database.FeedCheck{At: now, Next: now + 60000})
	if err != nil {
		t.Fatal(err)
	}
	var translated []string
	svc := testMTService(store, testMTEngine{run: func(_ context.Context, req BatchRequest) (BatchResult, error) {
		translated = append(translated, req.Texts...)
		if req.Texts[0] == "Unsupported article" {
			return BatchResult{}, &ai.ModelError{Code: "UnsupportedLanguage"}
		}
		return BatchResult{Translations: []string{"标题"}}, nil
	}}, nil)
	if backoff := svc.translateNext(ctx); backoff != 0 {
		t.Fatalf("unsupported article backed off the whole worker for %v", backoff)
	}
	if backoff := svc.translateNext(ctx); backoff != 0 {
		t.Fatalf("unexpected second backoff %v", backoff)
	}
	if !slices.Equal(translated, []string{"Unsupported article", "Supported article"}) {
		t.Fatalf("worker did not advance to the older article: %v", translated)
	}
}

type cancellingCredentials struct {
	*memoryCreds
	cancel context.CancelFunc
}

func (c cancellingCredentials) Set(ref, value string) error {
	err := c.memoryCreds.Set(ref, value)
	c.cancel()
	return err
}

func TestProviderCredentialsFollowHTTPSOriginAndCommit(t *testing.T) {
	ctx := context.Background()
	_, store, _ := phase1Article(t, "<p>Example.</p>")
	svc := New(store, nil, nil, nil, nil)
	creds := newMemoryCreds()
	svc.credentials = creds
	id := string(EngineGoogleBYOK)
	var requests []string
	svc.httpClient = mockHTTPClient(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.URL.Host+"/"+req.Header.Get("X-Goog-Api-Key"))
		return jsonResponse(http.StatusOK, map[string]any{"data": map[string]any{
			"translations": []map[string]string{{"translatedText": "译文"}},
		}}), nil
	})

	if _, err := svc.SaveProviderConfig(ctx, SaveProviderInput{ID: id, Secret: "old-dummy-key"}); err != nil {
		t.Fatal(err)
	}
	old, err := store.GetTranslationProvider(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ endpoint, code string }{
		{"http://translation.googleapis.com", "InvalidEndpoint"},
		{"https://new.example.test/v2", "CredentialRequired"},
		{"https://translation.googleapis.com:8443/v2", "CredentialRequired"},
	} {
		draft := SaveProviderInput{ID: id, Endpoint: tc.endpoint}
		if _, err := svc.SaveProviderConfig(ctx, draft); modelErrorCode(err) != tc.code {
			t.Fatalf("save %q: got %v, want %s", tc.endpoint, err, tc.code)
		}
		if result := svc.TestProvider(ctx, id, &draft); result.OK || result.ErrorCode != tc.code {
			t.Fatalf("test %q: got %+v, want %s", tc.endpoint, result, tc.code)
		}
	}
	if len(requests) != 0 {
		t.Fatalf("unapproved origin received a request: %v", requests)
	}
	equivalent := SaveProviderInput{ID: id, Endpoint: "https://translation.googleapis.com:443/v2"}
	if result := svc.TestProvider(ctx, id, &equivalent); !result.OK {
		t.Fatalf("equivalent HTTPS origin should reuse credential: %+v", result)
	}
	saveCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	svc.credentials = cancellingCredentials{memoryCreds: creds, cancel: cancel}
	draft := SaveProviderInput{ID: id, Endpoint: "https://new.example.test/v2", Secret: "new-dummy-key"}
	if _, err := svc.SaveProviderConfig(saveCtx, draft); modelErrorCode(err) != "StorageError" {
		t.Fatalf("cancelled save should report storage failure: %v", err)
	}
	svc.credentials = creds
	restored, err := store.GetTranslationProvider(ctx, id)
	if err != nil || restored.CredentialRef != old.CredentialRef || restored.Endpoint != old.Endpoint ||
		creds.data[old.CredentialRef] != "old-dummy-key" || len(creds.data) != 1 {
		t.Fatalf("save failure damaged the old configuration: %+v, %v", restored, err)
	}
	if _, err := svc.SaveProviderConfig(ctx, draft); err != nil {
		t.Fatal(err)
	}
	if result := svc.TestProvider(ctx, id, nil); !result.OK {
		t.Fatalf("new origin with a fresh key should work: %+v", result)
	}
	want := []string{"translation.googleapis.com:443/old-dummy-key", "new.example.test/new-dummy-key"}
	if !slices.Equal(requests, want) || len(creds.data) != 1 || creds.data[old.CredentialRef] != "" {
		t.Fatalf("unexpected credential routing or key rotation: %v", requests)
	}
	// A previous release could persist HTTP. Keep displaying it for repair, but do not send it.
	latest, err := store.GetTranslationProvider(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveTranslationProvider(ctx, sqlc.SaveTranslationProviderParams{
		ID: id, CredentialRef: latest.CredentialRef, Endpoint: "http://new.example.test/v2",
	}); err != nil {
		t.Fatal(err)
	}
	if result := svc.TestProvider(ctx, id, nil); result.OK || result.ErrorCode != "InvalidEndpoint" || len(requests) != 2 {
		t.Fatalf("saved HTTP endpoint should fail without calling the transport: %+v", result)
	}
}

func TestTranslationClientRejectsCredentialRedirects(t *testing.T) {
	for _, location := range []string{"https://other.example.test/translate", "http://translation.googleapis.com/translate"} {
		client := defaultHTTPClient()
		calls := 0
		client.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls++
			resp := jsonResponse(http.StatusTemporaryRedirect, nil)
			resp.Header.Set("Location", location)
			return resp, nil
		})
		engine, err := newGoogleBYOKEngine(resolvedProviderConfig{Secret: "dummy"}, client)
		if err != nil {
			t.Fatal(err)
		}
		_, err = engine.TranslateBatch(context.Background(), BatchRequest{Texts: []string{"Article."}, TargetLang: "zh-CN"})
		if modelErrorCode(err) != "ProviderHTTPError" || calls != 1 {
			t.Fatalf("redirect %q received a follow-up request: %d requests, err=%v", location, calls, err)
		}
	}
}
