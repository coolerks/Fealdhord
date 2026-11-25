package articlesummary

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/aiconfig"
	"github.com/coolerks/Fealdhord/internal/articleai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/reader"
)

type testModel func(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error)

func (m testModel) Generate(ctx context.Context, request *ai.ModelRequest) (*ai.ModelResponse, error) {
	return m(ctx, request)
}

func (testModel) Stream(context.Context, *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
	panic("摘要使用单次请求")
}

type testResolver struct{ model ai.Model }

func (r testResolver) Resolve(_ context.Context, task string) (aiconfig.Model, ai.Model, error) {
	if task != "summary" {
		return aiconfig.Model{}, nil, failure("ConfigurationError")
	}
	return aiconfig.Model{ID: "test", Name: "test-model"}, r.model, nil
}

func output(status, text string) *ai.ModelResponse {
	raw, _ := json.Marshal(map[string]string{"status": status, "text": text})
	return &ai.ModelResponse{FinishReason: "stop", Message: ai.Message{
		Content: []ai.ContentBlock{{Type: "text", Text: string(raw)}},
	}}
}

func summaryFixture(t *testing.T, body string, explicit bool) (string, *database.Store, database.ArticleInput, Source) {
	t.Helper()
	ctx := t.Context()
	filename := filepath.Join(t.TempDir(), "library.db")
	store, err := database.Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	feedID, err := store.EnsureFeed(ctx, "https://example.test/feed", "测试订阅")
	if err != nil {
		t.Fatal(err)
	}
	input := database.ArticleInput{FeedID: feedID, GUID: "article", Title: "测试正文", Summary: "测试摘录", Text: body, FeedContentExplicit: explicit}
	id, err := store.UpsertArticle(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	article, err := store.ReaderArticle(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	return filename, store, input, Source{ArticleID: id, Mode: "feed", Version: article.Hash}
}

func TestSummaryRespectsSettingsAndCachesBothOutcomes(t *testing.T) {
	filename, store, _, source := summaryFixture(t, strings.Repeat("这段正文介绍维护窗口和完整的背景说明。", 12), true)
	ctx := t.Context()
	var calls int
	resolver := testResolver{testModel(func(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error) {
		calls++
		return output("generated", "近期安排了维护窗口。"), nil
	})}
	service := New(store, resolver, reader.NewService(store, nil, nil, nil))
	result, err := service.Generate(ctx, Request{Source: source, Automatic: true})
	if err != nil || result.Status != "none" || calls != 0 {
		t.Fatalf("自动默认关闭：%+v %d %v", result, calls, err)
	}
	result, err = service.Generate(ctx, Request{Source: source})
	if err != nil || result.Status != "skipped" || calls != 0 {
		t.Fatalf("短文未直接跳过：%+v %d %v", result, calls, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	service = New(reopened, resolver, reader.NewService(reopened, nil, nil, nil))
	result, err = service.Get(ctx, source)
	if err != nil || result.Status != "skipped" {
		t.Fatalf("未复用短文判断：%+v %v", result, err)
	}
	configuration := articleai.New(reopened, nil, nil)
	settings, err := configuration.Settings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	settings.Summary = &articleai.SummarySettings{OnOpen: true, HanLimit: 50, EnglishLimit: 350}
	if _, err = configuration.Save(ctx, settings); err != nil {
		t.Fatal(err)
	}
	result, err = service.Generate(ctx, Request{Source: source, Automatic: true})
	if err != nil || result.Status != "generated" || calls != 1 {
		t.Fatalf("缩小阈值后未生成：%+v %d %v", result, calls, err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
	offline, err := database.Open(ctx, filename)
	if err != nil {
		t.Fatal(err)
	}
	defer offline.Close()
	cache := New(offline, nil, nil)
	result, err = cache.Generate(ctx, Request{Source: source})
	if err != nil || result.Text != "近期安排了维护窗口。" || calls != 1 {
		t.Fatalf("重开时未复用已生成的摘要：%+v %d %v", result, calls, err)
	}
}

func TestSummaryWaitsForCompleteBody(t *testing.T) {
	_, store, _, source := summaryFixture(t, "只有摘录", false)
	service := New(store, nil, reader.NewService(store, nil, nil, nil))
	if _, err := service.Generate(t.Context(), Request{Source: source}); err == nil || err.Error() != "BodyUnavailable" {
		t.Fatalf("摘录被接受：%v", err)
	}
}

func TestSummaryCoalescesRequestsAndRejectsLateResult(t *testing.T) {
	_, store, input, source := summaryFixture(t, strings.Repeat("正文细节很多，需要检查最后的方案。", 42), true)
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	resolver := testResolver{testModel(func(ctx context.Context, _ *ai.ModelRequest) (*ai.ModelResponse, error) {
		if calls.Add(1) == 1 {
			close(entered)
		}
		select {
		case <-release:
			return output("generated", "旧版方案将推迟。"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})}
	service := New(store, resolver, reader.NewService(store, nil, nil, nil))
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	errors := make(chan error, 2)
	for range 2 {
		go func() {
			_, err := service.Generate(ctx, Request{Source: source})
			errors <- err
		}()
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("摘要调用未启动")
	}
	for {
		service.mu.Lock()
		waiters := service.active[source].waiters
		service.mu.Unlock()
		if waiters == 2 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("第二个请求未合并")
		case <-time.After(time.Millisecond):
		}
	}
	input.Text = "更新后的文章正文。"
	if _, err := store.UpsertArticle(ctx, input); err != nil {
		t.Fatal(err)
	}
	close(release)
	for range 2 {
		if err := <-errors; err == nil || err.Error() != "Conflict" {
			t.Fatalf("旧结果未被拦截：%v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("重复模型请求：%d", calls.Load())
	}
}

func TestShortBodyCountingAndDensity(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       bool
	}{
		{"中文在线上", strings.Repeat("字", 600), true},
		{"中文超线", strings.Repeat("字", 601), false},
		{"英文在线上", strings.Repeat("word ", 350), true},
		{"英文超线", strings.Repeat("word ", 351), false},
		{"中英在线上", strings.Repeat("字", 300) + strings.Repeat("word ", 175), true},
		{"中英超线", strings.Repeat("字", 300) + strings.Repeat("word ", 176), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if result := withinLimits(tc.text, 600, 350); result != tc.want {
				t.Fatalf("短文判断为 %v，预期 %v", result, tc.want)
			}
		})
	}
	if body, dense := readableBody("<p>近期举办活动。</p><p>活动将在公园举行。</p><pre>code</pre>", ""); dense || strings.Contains(body, "code") {
		t.Fatalf("普通多段公告：%q %v", body, dense)
	}
	if _, dense := readableBody("<p>新规下月生效。</p><p>适用全体成员。</p><p>持旧票的人除外。</p>", ""); !dense {
		t.Fatal("遗漏分散的条件")
	}
}

func TestLongSummaryIncludesTheFinalPartAndRejectsInvalidOutput(t *testing.T) {
	body := strings.Repeat("Earlier paragraphs cover the detailed background of a long local project.\n\n", 160) +
		"The final ruling takes effect on July 1, with a waiver for existing holders."
	cfg := aiconfig.Model{Name: "test", Metadata: ai.Metadata{Context: 2200}}
	calls := 0
	model := testModel(func(_ context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
		calls++
		var input summaryInput
		if err := json.Unmarshal([]byte(req.Messages[0].Content[0].Text), &input); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(req.System, noteInstructions) {
			if strings.Contains(input.Text, "July 1") {
				return output("generated", "July 1; waiver for existing holders."), nil
			}
			return output("generated", "Local project background."), nil
		}
		if !strings.Contains(input.Text, "July 1; waiver for existing holders.") {
			t.Fatal("后段关键事实未进入最终合并")
		}
		return output("generated", "The final ruling begins July 1, with a waiver for existing holders."), nil
	})
	result, err := summarize(t.Context(), cfg, model, "A local project", body, false)
	if err != nil || calls < 3 || !strings.Contains(result, "July 1") {
		t.Fatalf("长文摘要：%q %d %v", result, calls, err)
	}
	repeat := testModel(func(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error) {
		return output("generated", "Content should stay unchanged."), nil
	})
	if _, err := summarize(t.Context(), cfg, repeat, "Title", "Content should stay unchanged.", true); err == nil {
		t.Fatal("复述被接受")
	}
	truncated := output("generated", "A complete-looking result.")
	truncated.FinishReason = "length"
	if _, err := parseOutput(truncated, 100); err == nil {
		t.Fatal("截断被接受")
	}
}
