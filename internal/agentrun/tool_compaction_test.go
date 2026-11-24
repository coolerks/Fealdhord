package agentrun

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

func TestLongArticleAutomaticallyReadsAndCompressesToTail(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "complete"
		if partial {
			name = "partial"
		}
		t.Run(name, func(t *testing.T) {
			body := strings.Repeat("黑客松前的无关内容🙂\n\n", 5500) + "末尾证据：只在最后提到向量数据库。"
			source := NewArticleSource("one", "original", body)
			read, pages, rounds, batches := 0, 0, 0, 0
			tool := ReadTool{
				Definition: ai.ToolDefinition{Name: "article_extract"},
				Read: func(ctx context.Context, call ai.ToolCall) (string, error) {
					read++
					return ResultStoreFrom(ctx).PublishArticle("chat", call.Name, call.ArgumentsJSON, source)
				},
				ArticleNext: func(ctx context.Context, call ai.ToolCall, cursor string) (string, error) {
					pages++
					return ResultStoreFrom(ctx).Continue("chat", call.Name, cursor, nil)
				},
			}
			model := streamModel(func(_ context.Context, req *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
				if ai.EstimateRequest(*req) > req.Budget.InputLimit {
					t.Error("request exceeds budget")
				}
				if len(req.Tools) == 0 {
					batches++
					if partial && batches == 3 {
						return nil, failure("ProviderProtocolError")
					}
					material := req.Messages[0].Content[0].Text
					note := "已检查此区间"
					if strings.Contains(material, "末尾证据") {
						note = "末尾证据：只在最后提到向量数据库。"
					}
					return modelEvents(ai.ModelEvent{Type: "text_delta", Text: note}, ai.ModelEvent{Type: "message_end", FinishReason: "stop"}), nil
				}
				rounds++
				if rounds == 1 {
					call := ai.ToolCall{ID: "article-call", Name: "article_extract", ArgumentsJSON: `{"id":"one"}`}
					return modelEvents(ai.ModelEvent{Type: "tool_call_end", ToolCall: &call}, ai.ModelEvent{Type: "message_end", FinishReason: "tool_calls"}), nil
				}
				reply := req.Messages[len(req.Messages)-1]
				var result struct {
					Coverage, Summary, Error string
					End, Offset              int
				}
				if reply.Role != "tool" || reply.ToolCallID != "article-call" || json.Unmarshal([]byte(reply.Content[0].Text), &result) != nil {
					t.Fatal("missing paired article tool result")
				}
				if partial {
					if result.Coverage != "partial" || result.Error != "ArticleReadIncomplete" || result.End <= 0 || result.End >= len(body) {
						t.Fatalf("incomplete coverage %+v", result)
					}
					resumed, err := ResumeArticle(source, result.End, source.Version)
					if err != nil || resumed.Start != result.End {
						t.Fatalf("resume coverage: %+v %v", resumed, err)
					}
				} else if result.Coverage != "complete" || result.End != len(body) || !strings.Contains(result.Summary, "末尾证据") {
					t.Fatalf("tail missing: %+v", result)
				}
				return modelEvents(ai.ModelEvent{Type: "text_delta", Text: "final"}, ai.ModelEvent{Type: "message_end", FinishReason: "stop"}), nil
			})
			req := ai.ModelRequest{Budget: ai.ContextBudget(ai.Metadata{Context: 6500}),
				Messages: []ai.Message{{Role: "user", Content: textContent("全文有没有提到向量数据库？")}}}
			var answer string
			var finalError string
			for event := range Stream(t.Context(), model, req, []ReadTool{tool}) {
				if event.Error != nil {
					finalError = event.Error.Error()
				}
				if event.Type == "text_delta" {
					answer += event.Text
				}
			}
			if (partial && finalError != "ArticleReadIncomplete") || (!partial && finalError != "") {
				t.Fatalf("unexpected final error: %q", finalError)
			}
			if answer != "final" || read != 1 || rounds != 2 || batches < 3 || (!partial && pages < 10) {
				t.Fatalf("answer=%q reads=%d rounds=%d pages=%d batches=%d", answer, read, rounds, pages, batches)
			}
		})
	}
}

func TestCompactionRetriesInvalidSummaryAndReportsProgress(t *testing.T) {
	for _, retries := range []int{0, 2} {
		t.Run(fmt.Sprint("retries=", retries), func(t *testing.T) {
			var outputs, scheduled []int
			started := 0
			model := streamModel(func(_ context.Context, req *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
				outputs = append(outputs, *req.MaxOutputTokens)
				switch len(outputs) {
				case 1:
					return modelEvents(ai.ModelEvent{Type: "message_end", FinishReason: "stop"}), nil
				case 2:
					return modelEvents(ai.ModelEvent{Type: "text_delta", Text: "截断内容"},
						ai.ModelEvent{Type: "message_end", FinishReason: "length"}), nil
				default:
					return modelEvents(ai.ModelEvent{Type: "text_delta", Text: "完整摘要"},
						ai.ModelEvent{Type: "message_end", FinishReason: "stop"}), nil
				}
			})
			run := runCompaction{
				model: model, opts: StreamOptions{MaxRetries: retries, BaseDelay: time.Millisecond},
				forward: func(event ai.ModelEvent) bool {
					switch event.Type {
					case "compaction_retry_scheduled":
						if event.RetryMax != retries {
							t.Errorf("retry maximum is %d, want %d", event.RetryMax, retries)
						}
						scheduled = append(scheduled, event.RetryAttempt)
					case "retry_started":
						started++
					case "retry_scheduled", "text_delta":
						t.Errorf("unexpected main-reply event %s from a compression attempt", event.Type)
					}
					return true
				},
			}
			got, err := run.compress(t.Context(), ai.ModelRequest{Budget: ai.ContextBudget(ai.Metadata{Context: 8192})}, "压缩", "资料")
			if retries == 0 {
				if err == nil || got != "" || len(outputs) != 1 || started != 0 || len(scheduled) != 0 {
					t.Fatalf("disabled retries: %q %v, output caps %v, retries %v", got, err, outputs, scheduled)
				}
				return
			}
			if err != nil || got != "完整摘要" || !slices.Equal(outputs, []int{1024, 1024, 2048}) ||
				!slices.Equal(scheduled, []int{1, 2}) || started != 2 {
				t.Fatalf("retried compression: %q %v, output caps %v, retries %v, started %d", got, err, outputs, scheduled, started)
			}
		})
	}
}

func TestArticleCompressionFailureKeepsTextAndManualPagesCompleteCoverage(t *testing.T) {
	body := strings.Repeat("中", 4635)
	source := NewArticleSource("fixture", "feed", body)
	store := &ResultStore{}
	read, err := store.PublishArticle("chat", "article_get", "{}", source)
	if err != nil {
		t.Fatal(err)
	}
	tool := ReadTool{ArticleNext: func(_ context.Context, _ ai.ToolCall, cursor string) (string, error) {
		return store.Continue("chat", "article_get", cursor, nil)
	}}
	calls := 0
	run := runCompaction{
		model: streamModel(func(context.Context, *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
			calls++
			return modelEvents(ai.ModelEvent{Type: "message_end", FinishReason: "stop"}), nil
		}),
		forward: func(ai.ModelEvent) bool { return true },
	}
	request := ai.ModelRequest{Budget: ai.ContextBudget(ai.Metadata{Context: 10000}),
		Messages: []ai.Message{{Role: "user", Content: textContent("检查这篇文章的全文。")}}}

	var fallback struct {
		Error, Stage, Cause, Text string
		End                       int
	}
	for range 2 {
		result, err := run.completeArticle(t.Context(), request, ai.ToolCall{}, tool, read)
		if err != nil || json.Unmarshal([]byte(result), &fallback) != nil || fallback.Error != "ArticleReadIncomplete" ||
			fallback.Stage != "compaction" || fallback.End <= 0 || fallback.End >= len(body) || fallback.Text != body[:fallback.End] {
			t.Fatalf("fallback discarded available text: %+v, err %v", fallback, err)
		}
	}
	if calls != 1 {
		t.Fatalf("repeated a failed compression rather than returning a readable page: %d requests", calls)
	}
	// The model resumes through the ordinary page_only interface with the same content version.
	resumed, err := ResumeArticle(source, fallback.End, source.Version)
	if err != nil {
		t.Fatal(err)
	}
	resumed.PageOnly = true
	last, err := store.PublishArticle("chat", "article_get", "{}", resumed)
	if err != nil {
		t.Fatal(err)
	}
	out, err := run.completeArticle(t.Context(), request, ai.ToolCall{}, tool, last)
	if err != nil || out != last || len(run.incomplete) != 0 {
		t.Fatalf("manual coverage still marked incomplete: %v, %v", run.incomplete, err)
	}
}
