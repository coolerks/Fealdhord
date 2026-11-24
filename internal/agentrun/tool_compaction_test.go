package agentrun

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

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
