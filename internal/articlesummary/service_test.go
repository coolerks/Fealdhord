package articlesummary

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
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
	finalPartAttempts := 0
	model := testModel(func(_ context.Context, req *ai.ModelRequest) (*ai.ModelResponse, error) {
		calls++
		var input summaryInput
		if err := json.Unmarshal([]byte(req.Messages[0].Content[0].Text), &input); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(req.System, noteInstructions) {
			if strings.Contains(input.Text, "July 1") {
				finalPartAttempts++
				if finalPartAttempts == 1 {
					return &ai.ModelResponse{FinishReason: "stop"}, nil
				}
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
	if err != nil || calls < 4 || finalPartAttempts != 2 || !strings.Contains(result, "July 1") {
		t.Fatalf("长文摘要：%q %d %v", result, calls, err)
	}
	repeatCalls := 0
	repeat := testModel(func(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error) {
		repeatCalls++
		return output("generated", "Content should stay unchanged."), nil
	})
	if _, err := summarize(t.Context(), cfg, repeat, "Title", "Content should stay unchanged.", true); err == nil ||
		err.Error() != "InvalidModelOutput" || repeatCalls != 2 {
		t.Fatalf("重复正文应在一次补发后结束：调用 %d 次，错误 %v", repeatCalls, err)
	}
	truncated := output("generated", "A complete-looking result.")
	truncated.FinishReason = "length"
	if _, err := parseOutput(truncated, 100); err == nil {
		t.Fatal("截断被接受")
	}
}

func TestSummaryRecoversAndCachesValidModelOutput(t *testing.T) {
	const summary = "维护窗口定于下周，现有用户无需迁移。"
	for _, test := range []struct {
		name  string
		first *ai.ModelResponse
	}{
		{name: "JSON 代码围栏"},
		{name: "输出截断", first: &ai.ModelResponse{FinishReason: "length"}},
		{name: "格式错误", first: &ai.ModelResponse{FinishReason: "stop"}},
		{name: "只重复标题", first: output("generated", "测试正文")},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, store, _, source := summaryFixture(t, strings.Repeat("这段正文详细介绍了维护计划和现有用户的处理方案。", 40), true)
			var calls, firstLimit int
			var firstInput string
			resolver := testResolver{testModel(func(_ context.Context, request *ai.ModelRequest) (*ai.ModelResponse, error) {
				calls++
				input := request.Messages[0].Content[0].Text
				if *request.MaxOutputTokens > request.Budget.OutputReserve {
					t.Errorf("输出额度超过模型预留：%d > %d", *request.MaxOutputTokens, request.Budget.OutputReserve)
				}
				if calls == 1 {
					firstLimit = *request.MaxOutputTokens
					firstInput = input
					if firstLimit < ai.EstimateTokens(strings.Repeat("中", 720)) {
						t.Errorf("完整中文摘要的输出额度不足：%d token", firstLimit)
					}
					if test.first != nil {
						return test.first, nil
					}
				}
				if input != firstInput {
					t.Error("重试修改了摘要来源")
				}
				if test.first != nil && test.first.FinishReason == "length" && *request.MaxOutputTokens <= firstLimit {
					t.Errorf("截断后的输出额度未提高：%d → %d", firstLimit, *request.MaxOutputTokens)
				}
				result := output("generated", summary)
				result.Message.Content[0].Text = "```json\r\n" + result.Message.Content[0].Text + "\r\n```"
				return result, nil
			})}
			service := New(store, resolver, reader.NewService(store, nil, nil, nil))
			result, err := service.Generate(t.Context(), Request{Source: source})
			wantCalls := 1
			if test.first != nil {
				wantCalls = 2
			}
			if err != nil || result.Status != "generated" || result.Text != summary || calls != wantCalls {
				t.Fatalf("生成摘要：%+v，调用 %d 次，错误 %v", result, calls, err)
			}
			cached, err := service.Get(t.Context(), source)
			if err != nil || cached != result {
				t.Fatalf("摘要未保存：%+v %v", cached, err)
			}
		})
	}
}

func TestSummaryRetriesATimedOutRequestWithFreshDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, store, _, source := summaryFixture(t, strings.Repeat("这段正文介绍了维护计划和停机范围。", 45), true)
		var calls int
		resolver := testResolver{testModel(func(ctx context.Context, _ *ai.ModelRequest) (*ai.ModelResponse, error) {
			calls++
			if calls == 1 {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			if ctx.Err() != nil {
				t.Error("重试继承了已过期的请求时限")
			}
			return output("generated", "维护范围仅涉及测试区。"), nil
		})}
		service := New(store, resolver, reader.NewService(store, nil, nil, nil))
		result, err := service.Generate(t.Context(), Request{Source: source})
		if err != nil || result.Status != "generated" || calls != 2 {
			t.Fatalf("单次请求超时后生成摘要：%+v，调用 %d 次，错误 %v", result, calls, err)
		}
	})
}

func TestSummaryQueueWaitDoesNotConsumeGenerationTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, store, _, source := summaryFixture(t, strings.Repeat("这段正文介绍了维护计划和停机范围。", 45), true)
		resolver := testResolver{testModel(func(ctx context.Context, _ *ai.ModelRequest) (*ai.ModelResponse, error) {
			timer := time.NewTimer(30 * time.Second)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timer.C:
				return output("generated", "维护范围仅涉及测试区。"), nil
			}
		})}
		service := New(store, resolver, reader.NewService(store, nil, nil, nil))
		for range cap(service.slots) {
			service.slots <- struct{}{}
		}
		go func() {
			time.Sleep(110 * time.Second)
			<-service.slots
		}()
		result, err := service.Generate(t.Context(), Request{Source: source})
		if err != nil || result.Status != "generated" {
			t.Fatalf("排队消耗了模型执行时间：%+v %v", result, err)
		}
	})
}

func TestSummaryRetriesOnlyTransientModelFailures(t *testing.T) {
	for _, test := range []struct {
		code      string
		recover   bool
		wantCalls int
	}{
		{code: "ProviderConnectionError", recover: true, wantCalls: 2},
		{code: "ProviderConnectionError", wantCalls: 2},
		{code: "AuthenticationError", wantCalls: 1},
		{code: "RateLimitError", wantCalls: 1},
	} {
		name := test.code
		if test.recover {
			name += "/recovers"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				model := testModel(func(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error) {
					calls++
					if calls == 2 && test.recover {
						return output("generated", "The west entrance opens in July."), nil
					}
					return nil, failure(test.code)
				})
				body := strings.Repeat("An update on the project explains the opening schedule and exceptions. ", 12)
				result, err := summarize(t.Context(), aiconfig.Model{Name: "test"}, model, "Project update", body, false)
				if calls != test.wantCalls {
					t.Errorf("模型调用 %d 次，预期 %d 次", calls, test.wantCalls)
				}
				if test.recover {
					if err != nil || result == "" {
						t.Fatalf("模型连接恢复后生成摘要：%q %v", result, err)
					}
				} else if err == nil || err.Error() != test.code || result != "" {
					t.Fatalf("应保留模型错误：%q %v", result, err)
				}
			})
		})
	}
}

func TestSummaryCancellationStopsPendingRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		firstCall := make(chan struct{})
		result := make(chan error, 1)
		calls := 0
		model := testModel(func(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error) {
			calls++
			if calls == 1 {
				close(firstCall)
			}
			return nil, failure("ProviderConnectionError")
		})
		go func() {
			body := strings.Repeat("An update on the project explains the opening schedule and exceptions. ", 12)
			_, err := summarize(ctx, aiconfig.Model{Name: "test"}, model, "Project update", body, false)
			result <- err
		}()
		<-firstCall
		synctest.Wait()
		cancel()
		if err := <-result; err == nil || err.Error() != "Cancelled" || calls != 1 {
			t.Fatalf("取消后仍有补发：调用 %d 次，错误 %v", calls, err)
		}
	})
}
