package translation

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/reader"
)

type fakeModel struct {
	calls atomic.Int32
	fn    func(ctx context.Context, req *ai.ModelRequest, call int32) (*ai.ModelResponse, error)
}

func (f *fakeModel) Generate(ctx context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
	n := f.calls.Add(1)
	return f.fn(ctx, req, n)
}

func (f *fakeModel) Stream(context.Context, *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
	return nil, errors.New("not implemented")
}

type fakeResolver struct {
	model ai.Model
}

func (r fakeResolver) Resolve(context.Context, string) (aiconfig.Model, ai.Model, error) {
	return aiconfig.Model{ID: "tr-model", Name: "gpt-test"}, r.model, nil
}

func TestDetectLocal(t *testing.T) {
	cases := []struct {
		text      string
		wantLang  string
		confident bool
	}{
		{"这是一篇关于分布式系统架构与数据一致性的中文技术文章。", "zh-CN", true},
		{"這是一篇關於分散式系統架構與資料一致性的繁體中文技術文章，探討網路通訊與開關問題。", "zh-TW", true},
		{"This is an article about distributed systems and the architecture that makes them reliable.", "en", true},
		{"これは分散システムとネットワークに関する日本語の技術記事です。", "ja", true},
		{"이것은 분산 시스템 아키텍처에 관한 한국어 기술 기사입니다.", "ko", true},
		{"OK", "", false},
	}
	for _, tc := range cases {
		lang, conf := DetectLocal(tc.text)
		if lang != tc.wantLang || conf != tc.confident {
			t.Errorf("DetectLocal(%q) = (%q, %v), want (%q, %v)", tc.text, lang, conf, tc.wantLang, tc.confident)
		}
	}
}

func TestExtractBlocksAndSanitize(t *testing.T) {
	rawHTML := `<h2 data-tr-block="b0">Introduction</h2>
<p data-tr-block="b1">Read <a href="#reader-link" data-reader-link="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef">the docs</a> carefully.<img src="/media/inline/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" alt="fig"/></p>
<pre><code class="language-go">fmt.Println("do not translate")</code></pre>
<p data-tr-block="b2">Second paragraph.</p>`

	blocks, err := ExtractBlocks(rawHTML)
	if err != nil {
		t.Fatalf("extract blocks: %v", err)
	}
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d: %+v", len(blocks), blocks)
	}
	if strings.Contains(blocks[1].InnerHTML, "<img") {
		t.Fatalf("expected img stripped from translatable innerHTML, got %q", blocks[1].InnerHTML)
	}
	if !strings.Contains(blocks[1].InnerHTML, `data-reader-link=`) {
		t.Fatalf("expected reader link preserved, got %q", blocks[1].InnerHTML)
	}

	cleaned := SanitizeTranslatedHTML(`阅读<a href="#reader-link" data-reader-link="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef">文档</a><script>alert(1)</script>`)
	if strings.Contains(cleaned, "<script") {
		t.Fatalf("xss not stripped: %q", cleaned)
	}
	if !strings.Contains(cleaned, `data-reader-link=`) {
		t.Fatalf("valid reader link stripped: %q", cleaned)
	}
}

func TestTranslationCacheSkipAndResume(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := database.Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC().UnixMilli()
	_, err = store.AddSubscription(ctx, database.SubscriptionInput{
		URL:   "https://example.com/rss.xml",
		Title: "Tech Blog",
	}, []database.ArticleInput{
		{
			GUID:        "en-1",
			Title:       "Understanding Go Concurrency Patterns",
			URL:         "https://example.com/go-concurrency",
			Summary:     "This article explains how goroutines and channels work together in production systems.",
			HTML:        "<h2>Overview</h2><p>First English paragraph with enough content to translate.</p><pre><code>go func() {}()</code></pre><p>Second English paragraph explaining channels.</p>",
			Text:        "Overview First English paragraph Second English paragraph",
			PublishedAt: &now,
			FetchedAt:   now,
		},
		{
			GUID:        "zh-1",
			Title:       "深入理解 Go 语言并发编程模型",
			URL:         "https://example.com/go-zh",
			Summary:     "本文详细介绍 Go 语言的协程调度器与通道通信机制，帮助开发者编写高并发服务。",
			HTML:        "<p>这是一篇中文文章正文内容。</p>",
			Text:        "这是一篇中文文章正文内容。",
			PublishedAt: &now,
			FetchedAt:   now,
		},
	}, database.FeedCheck{At: now, Next: now + 60000})
	if err != nil {
		t.Fatalf("seed feed: %v", err)
	}

	articles, err := store.ListArticles(ctx, 10)
	if err != nil || len(articles) != 2 {
		t.Fatalf("list articles: %v len=%d", err, len(articles))
	}
	var enID, zhID string
	for _, a := range articles {
		if strings.Contains(a.Title, "Understanding") {
			enID = a.ID
		} else {
			zhID = a.ID
		}
	}

	model := &fakeModel{
		fn: func(_ context.Context, req *ai.ModelRequest, _ int32) (*ai.ModelResponse, error) {
			inText := req.Messages[0].Content[0].Text
			if strings.Contains(inText, `"summary"`) {
				return &ai.ModelResponse{
					Message: ai.Message{Role: "assistant", Content: []ai.ContentBlock{{Type: "text", Text: `{"detected_lang":"en","already_target":false,"title":"理解 Go 并发模式","summary":"本文介绍 goroutine 和 channel 如何在生产系统中协同工作。"}`}}},
				}, nil
			}
			var bodyReq struct {
				Title string `json:"title"`
				Items []struct {
					ID   string `json:"id"`
					HTML string `json:"html"`
				} `json:"items"`
			}
			_ = json.Unmarshal([]byte(inText), &bodyReq)
			outItems := make([]map[string]string, 0, len(bodyReq.Items))
			for _, it := range bodyReq.Items {
				outItems = append(outItems, map[string]string{
					"id":   it.ID,
					"html": "译文：" + it.HTML,
				})
			}
			raw, _ := json.Marshal(map[string]any{
				"detected_lang":  "en",
				"already_target": false,
				"title":          "理解 Go 并发模式",
				"items":          outItems,
			})
			return &ai.ModelResponse{
				Message: ai.Message{Role: "assistant", Content: []ai.ContentBlock{{Type: "text", Text: string(raw)}}},
			}, nil
		},
	}

	readerSvc := reader.NewService(store, nil, nil, nil)
	var events []ProgressEvent
	svc := New(store, fakeResolver{model: model}, readerSvc, func(ev ProgressEvent) {
		events = append(events, ev)
	}, nil)

	// 1. 中文文章目标语言 zh-CN：本地初筛直接跳过，0 模型调用。
	zhRec, err := svc.Translate(ctx, TranslateRequest{ArticleID: zhID, TargetLang: "zh-CN", SourceMode: "summary"})
	if err != nil {
		t.Fatalf("translate zh summary: %v", err)
	}
	if zhRec.Status != "skipped_same_lang" || model.calls.Load() != 0 {
		t.Fatalf("expected skipped_same_lang with 0 calls, got status=%q calls=%d", zhRec.Status, model.calls.Load())
	}

	// 2. 英文文章 summary 翻译：调用 1 次模型并落库。
	enSum, err := svc.Translate(ctx, TranslateRequest{ArticleID: enID, TargetLang: "zh-CN", SourceMode: "summary"})
	if err != nil {
		t.Fatalf("translate en summary: %v", err)
	}
	if enSum.Status != "completed" || enSum.TranslatedTitle != "理解 Go 并发模式" || model.calls.Load() != 1 {
		t.Fatalf("unexpected en summary: %+v calls=%d", enSum, model.calls.Load())
	}

	// 3. 再次请求同文章 summary：直接命中缓存，模型调用次数保持 1。
	enSumCached, err := svc.Translate(ctx, TranslateRequest{ArticleID: enID, TargetLang: "zh-CN", SourceMode: "summary"})
	if err != nil || enSumCached.TranslatedTitle != "理解 Go 并发模式" || model.calls.Load() != 1 {
		t.Fatalf("expected cache hit without new model call, got %+v calls=%d err=%v", enSumCached, model.calls.Load(), err)
	}

	// 4. 英文文章 feed 正文翻译：跳过 <pre><code>，仅翻译 3 个文本块并推送事件。
	enBody, err := svc.Translate(ctx, TranslateRequest{ArticleID: enID, TargetLang: "zh-CN", SourceMode: "feed"})
	if err != nil {
		t.Fatalf("translate en feed body: %v", err)
	}
	if enBody.Status != "completed" || len(enBody.Blocks) != 3 || enBody.TranslatedBlocks != 3 || enBody.TotalBlocks != 3 {
		t.Fatalf("unexpected en body translation: %+v", enBody)
	}
	if len(events) == 0 {
		t.Fatalf("expected progress events emitted")
	}

	// 5. 再次请求正文翻译：命中缓存，模型调用次数不变（仍为 2）。
	callsBefore := model.calls.Load()
	_, err = svc.Translate(ctx, TranslateRequest{ArticleID: enID, TargetLang: "zh-CN", SourceMode: "feed"})
	if err != nil || model.calls.Load() != callsBefore {
		t.Fatalf("expected body cache hit, calls went from %d to %d err=%v", callsBefore, model.calls.Load(), err)
	}
}
