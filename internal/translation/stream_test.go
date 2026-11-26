package translation

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/database/sqlc"
	"github.com/coolerks/Fealdhord/internal/reader"
)

func TestTranslationUsesFirstPassHeadroomAndCommitsEveryRecordDuringStream(t *testing.T) {
	_, store, id := phase1Article(t, "<p>Opening paragraph.</p><p>Middle paragraph.</p><p>Closing paragraph.</p>")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	lastVisible := make(chan struct{})
	var limits, progress []int
	model := &fakeModel{stream: func(ctx context.Context, req *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
		if req.ReasoningEffort != "low" {
			t.Fatalf("translation should use the available low effort, got %q", req.ReasoningEffort)
		}
		limits = append(limits, *req.MaxOutputTokens)
		events := make(chan ai.ModelEvent, 4)
		// The real six-paragraph replay used 467 reasoning and 124 answer tokens.
		if *req.MaxOutputTokens < 591 {
			events <- ai.ModelEvent{Type: "message_end", FinishReason: "length"}
			close(events)
			return events, nil
		}
		events <- ai.ModelEvent{Type: "reasoning_text_delta", Text: "Planning the translation."}
		events <- ai.ModelEvent{Type: "text_delta", Text: "{\"id\":\"title\",\"text\":\"标题\"}\n{\"id\":\"b0\",\"text\":\"第一段\"}\n{\"id\":\"b1\",\"text\":\"第二段\"}\n{\"id\":\"b2\",\"text\":\"第三段"}
		events <- ai.ModelEvent{Type: "text_delta", Text: "\"}"}
		go func() {
			defer close(events)
			select {
			case <-ctx.Done():
			case <-lastVisible:
				events <- ai.ModelEvent{Type: "message_end", FinishReason: "stop"}
			}
		}()
		return events, nil
	}}
	resolver := fakeResolver{model: model, reasoningLevels: []aiconfig.ReasoningLevel{{Value: "max"}, {Value: "low"}}}
	svc := New(store, resolver, reader.NewService(store, nil, nil, nil), func(event ProgressEvent) {
		record := event.Record
		if record.Status != "translating" || record.TranslatedBlocks == 0 {
			return
		}
		progress = append(progress, record.TranslatedBlocks)
		saved, err := store.GetArticleTranslation(ctx, id, record.TargetLang, record.SourceMode)
		if err != nil || int(saved.TranslatedBlocks) != record.TranslatedBlocks {
			t.Fatalf("each progress event must follow its database commit: %+v %v", saved, err)
		}
		if slices.Equal(progress, []int{1, 2, 3}) {
			close(lastVisible)
		}
	}, nil)
	result, err := svc.Translate(ctx, TranslateRequest{ArticleID: id, SourceMode: "feed"})
	if err != nil || result.Status != "completed" || result.TranslatedBlocks != 3 || len(limits) != 1 ||
		!slices.Equal(progress, []int{1, 2, 3}) {
		t.Fatalf("need one request and three commits before the stream ends: limits=%v progress=%v status=%s err=%v",
			limits, progress, result.Status, err)
	}
}

func TestTranslationCommitsBeforeStreamEndsAndResumesMissingIDs(t *testing.T) {
	path, store, id := phase1Article(t, "<p>Opening paragraph.</p><p>Missing paragraph in the middle.</p><p>Last paragraph.</p>")
	visible := make(chan struct{})
	var calls [][]string
	model := &fakeModel{stream: func(ctx context.Context, req *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
		payload := readPhase1Payload(t, req)
		var ids []string
		for _, item := range payload.Items {
			ids = append(ids, item.ID)
		}
		calls = append(calls, ids)
		if len(calls) == 2 {
			return nil, &ai.ModelError{Code: "RateLimitError"}
		}
		events := make(chan ai.ModelEvent, 8)
		if len(calls) == 1 {
			events <- ai.ModelEvent{Type: "text_delta", Text: "{\"id\":\"title\",\"text\":\"示例标题\"}\n{\"id\":\"b0\","}
			events <- ai.ModelEvent{Type: "text_delta", Text: "\"text\":\"开头第一段\"}\n"}
			go func() {
				defer close(events)
				select {
				case <-ctx.Done():
					return
				case <-visible:
				}
				events <- ai.ModelEvent{Type: "text_delta", Text: "说明文字\n{\"id\":\"b0\",\"text\":\"重复项\"}\n{\"id\":\"b99\",\"text\":\"陌生段落\"}\n{\"id\":\"b2\",\"text\":\"最后一段\"}\n{\"id\":\"b1\",\"text\":\"未完"}
				events <- ai.ModelEvent{Type: "error", Error: &ai.ModelError{Code: "ProviderConnectionError"}}
			}()
		} else {
			events <- ai.ModelEvent{Type: "text_delta", Text: "{\"id\":\"b1\",\"text\":\"补译的中间段\"}"}
			events <- ai.ModelEvent{Type: "message_end", FinishReason: "stop"}
			close(events)
		}
		return events, nil
	}}
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	svc := New(store, fakeResolver{model: model}, reader.NewService(store, nil, nil, nil), func(event ProgressEvent) {
		if event.Record.Status == "translating" && event.Record.TranslatedBlocks == 1 {
			close(visible)
		}
	}, nil)
	req := TranslateRequest{ArticleID: id, SourceMode: "feed"}
	partial, err := svc.Translate(ctx, req)
	if modelErrorCode(err) != "RateLimitError" || partial.Status != "partial" || partial.TranslatedBlocks != 2 ||
		partial.Blocks[0].TranslatedHTML != "开头第一段" || partial.Blocks[1].ID != "b2" {
		t.Fatalf("expected two verified blocks after disconnect: %+v, %v", partial, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	svc = New(reopened, fakeResolver{model: model}, reader.NewService(reopened, nil, nil, nil), nil, nil)
	complete, err := svc.Translate(ctx, req)
	if err != nil || complete.Status != "completed" || complete.TranslatedBlocks != 3 || len(calls) != 3 ||
		!slices.Equal(calls[1], []string{"b1"}) || !slices.Equal(calls[2], []string{"b1"}) {
		t.Fatalf("resume should translate only the missing ID: status=%s, calls=%v, err=%v", complete.Status, calls, err)
	}
}

func TestTranslationSavesCompleteLastRecordWhenProviderDisconnects(t *testing.T) {
	_, store, id := phase1Article(t, "<p>Opening paragraph.</p><p>Closing paragraph.</p>")
	calls := 0
	model := &fakeModel{stream: func(_ context.Context, _ *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
		calls++
		events := make(chan ai.ModelEvent, 2)
		events <- ai.ModelEvent{Type: "text_delta", Text: "{\"id\":\"title\",\"text\":\"标题\"}\n{\"id\":\"b0\",\"text\":\"开头一段。\"}\n{\"id\":\"b1\",\"text\":\"结尾一段。\"}"}
		events <- ai.ModelEvent{Type: "error", Error: &ai.ModelError{Code: "ProviderProtocolError"}}
		close(events)
		return events, nil
	}}
	svc := New(store, fakeResolver{model: model}, reader.NewService(store, nil, nil, nil), nil, nil)
	req := TranslateRequest{ArticleID: id, SourceMode: "feed"}
	result, err := svc.Translate(t.Context(), req)
	if err != nil || result.Status != "completed" || len(result.Blocks) != 2 || calls != 1 {
		t.Fatalf("complete final record should need no retry: %s %d calls=%d err=%v", result.Status, len(result.Blocks), calls, err)
	}
	saved, err := svc.Get(t.Context(), id, "zh-CN", "feed")
	if err != nil || len(saved.Blocks) != 2 || saved.Blocks[1].TranslatedHTML != "结尾一段。" {
		t.Fatalf("last record was not saved: %+v %v", saved, err)
	}
}

func TestTranslationRetainsCodeOnlyBlocksWithoutSendingThem(t *testing.T) {
	_, store, id := phase1Article(t, `<table><tr><td><code>/plan</code></td><td>An ordinary label.</td></tr></table>`)
	model := &fakeModel{fn: func(_ context.Context, request *ai.ModelRequest, _ int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, request)
		if len(payload.Items) != 1 || payload.Items[0].ID != "b1" || strings.Contains(payload.Items[0].Text, "/plan") {
			t.Fatalf("only the prose cell should reach the model: %+v", payload.Items)
		}
		payload.Items[0].Text = "普通说明。"
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	svc := New(store, fakeResolver{model: model}, reader.NewService(store, nil, nil, nil), nil, nil)
	result, err := svc.Translate(t.Context(), TranslateRequest{ArticleID: id, SourceMode: "feed"})
	if err != nil || result.Status != "completed" || len(result.Blocks) != 2 || result.Blocks[0].TranslatedHTML != "<code>/plan</code>" || model.calls.Load() != 1 {
		t.Fatalf("protected content should be retained and counted: %+v %v", result, err)
	}
}

func TestTranslationBatchLimitAndGraphemeBoundaries(t *testing.T) {
	p := batchPlanner{model: "test", system: "Translate.", budget: ai.ContextBudget(ai.Metadata{
		Context: 65536, OutputLimit: 10000,
	}), batchBytes: 1024}
	cluster := "👩🏽‍💻"
	block := SourceBlock{ID: "b0", SourceHash: "long", InnerHTML: strings.Repeat(cluster, 400)}
	plan, err := p.split(block)
	if err != nil || len(plan.units) < 2 {
		t.Fatalf("expected a split at the configured limit: %d, %v", len(plan.units), err)
	}
	var joined strings.Builder
	for _, unit := range plan.units {
		if !utf8.ValidString(unit.HTML) || len(unit.HTML) > 1024 || strings.ReplaceAll(unit.HTML, cluster, "") != "" {
			t.Fatalf("broken grapheme at split: %q", unit.HTML)
		}
		joined.WriteString(unit.HTML)
	}
	if joined.String() != block.InnerHTML {
		t.Fatal("split did not preserve every character")
	}
}

func TestTranslationUsesSavedByteLimit(t *testing.T) {
	html := strings.Repeat("<p>"+strings.Repeat("One clear sentence. ", 18)+"</p>", 7)
	_, store, id := phase1Article(t, html)
	settings, err := store.ArticleAISettings(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.SaveArticleAISettings(t.Context(), sqlc.SaveArticleAISettingsParams{
		Revision: settings.Revision, RequestsPerHour: 100, TokensPerHour: 200000,
		TranslationBatchKib: sql.NullInt64{Int64: 1, Valid: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	model := &fakeModel{fn: func(_ context.Context, req *ai.ModelRequest, _ int32) (*ai.ModelResponse, error) {
		payload := readPhase1Payload(t, req)
		sourceBytes := len(payload.Title)
		for i, item := range payload.Items {
			sourceBytes += len(item.Text)
			payload.Items[i].Text = "这是这一段的译文。"
		}
		if sourceBytes > 1<<10 {
			t.Fatalf("source uses %d bytes, above saved cap", sourceBytes)
		}
		return phase1ModelReply(t, payload, "stop"), nil
	}}
	svc := New(store, fakeResolver{model: model}, reader.NewService(store, nil, nil, nil), nil, nil)
	out, err := svc.Translate(t.Context(), TranslateRequest{ArticleID: id, SourceMode: "feed"})
	if err != nil || out.TranslatedBlocks != 7 || model.calls.Load() < 2 {
		t.Fatalf("expected multiple batches and complete translation: %d blocks, %d calls, %v", out.TranslatedBlocks, model.calls.Load(), err)
	}
}
