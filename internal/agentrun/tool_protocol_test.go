package agentrun

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/ai"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(request *http.Request) (*http.Response, error) { return f(request) }

func TestArticleCompressionKeepsToolPairOnAllProtocols(t *testing.T) {
	for _, tc := range []struct {
		name                             string
		newModel                         func(*http.Client) (ai.Model, error)
		call, resultMarker, finish, tail string
	}{
		{
			name: "chat-completions",
			newModel: func(client *http.Client) (ai.Model, error) {
				return ai.NewChatCompletions("http://local.test/v1", "key", client)
			},
			call: `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"read-id","type":"function","function":{"name":"read_article","arguments":"{}"}}]}}]}
{"choices":[{"index":0,"finish_reason":"tool_calls"}]}
[DONE]`,
			resultMarker: `"tool_call_id":"read-id"`,
			finish: `{"choices":[{"index":0,"finish_reason":"stop"}]}
[DONE]`,
			tail: `{"choices":[{"index":0,"delta":{"content":"尾部证据"}}]}`,
		},
		{
			name: "responses",
			newModel: func(client *http.Client) (ai.Model, error) {
				return ai.NewResponses("http://local.test/v1", "key", client)
			},
			call: `{"type":"response.created"}
{"type":"response.output_item.added","item":{"type":"function_call","id":"item","call_id":"read-id","name":"read_article","arguments":""}}
{"type":"response.function_call_arguments.delta","item_id":"item","delta":"{}"}
{"type":"response.completed","response":{"status":"completed","output":[{"type":"function_call","id":"item","call_id":"read-id","name":"read_article","arguments":"{}"}]}}`,
			resultMarker: `"type":"function_call_output"`,
			finish:       `{"type":"response.completed","response":{"status":"completed","output":[]}}`,
			tail: `{"type":"response.created"}
{"type":"response.output_text.delta","delta":"尾部证据"}`,
		},
		{
			name: "messages",
			newModel: func(client *http.Client) (ai.Model, error) {
				return ai.NewMessages("http://local.test/v1", "key", client)
			},
			call: `{"type":"message_start","message":{"type":"message","role":"assistant"}}
{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"read-id","name":"read_article","input":{}}}
{"type":"content_block_stop","index":0}
{"type":"message_delta","delta":{"stop_reason":"tool_use"}}
{"type":"message_stop"}`,
			resultMarker: `"tool_use_id":"read-id"`,
			finish: `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}
{"type":"message_stop"}`,
			tail: `{"type":"message_start","message":{"type":"message","role":"assistant"}}
{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}
{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"尾部证据"}}
{"type":"content_block_stop","index":0}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rounds, compactions := 0, 0
			client := &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
				if strings.HasSuffix(r.URL.Path, "/count_tokens") {
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"input_tokens":3200}`)), Request: r}, nil
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					return nil, err
				}
				payload := string(body)
				reply := tc.finish
				switch {
				case strings.Contains(payload, "本批原文"):
					compactions++
					if strings.Contains(payload, "只有末页提到向量检索") {
						reply = tc.tail + "\n" + reply
					} else {
						reply = strings.ReplaceAll(tc.tail, "尾部证据", "已读本批") + "\n" + reply
					}
				case rounds == 0:
					rounds++
					reply = tc.call
				default:
					rounds++
					if !strings.Contains(payload, tc.resultMarker) || !strings.Contains(payload, "read-id") || !strings.Contains(payload, "尾部证据") {
						t.Error("missing paired result or tail evidence")
					}
					reply = tc.tail + "\n" + reply
				}
				var stream strings.Builder
				for line := range strings.SplitSeq(reply, "\n") {
					stream.WriteString("data: " + line + "\n\n")
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(stream.String())), Request: r}, nil
			})}
			model, err := tc.newModel(client)
			if err != nil {
				t.Fatal(err)
			}
			source := NewArticleSource("id", "feed", strings.Repeat("较早的中文段落。\n\n", 8500)+"只有末页提到向量检索")
			tool := ReadTool{Definition: ai.ToolDefinition{Name: "read_article", InputSchema: json.RawMessage(`{"type":"object"}`)},
				Read: func(ctx context.Context, call ai.ToolCall) (string, error) {
					return ResultStoreFrom(ctx).PublishArticle("owner", call.Name, call.ArgumentsJSON, source)
				},
				ArticleNext: func(ctx context.Context, call ai.ToolCall, next string) (string, error) {
					return ResultStoreFrom(ctx).Continue("owner", call.Name, next, nil)
				},
			}
			req := ai.ModelRequest{Model: "test", Budget: ai.ContextBudget(ai.Metadata{Context: 8192}), Messages: []ai.Message{{Role: "user", Content: textContent("查向量检索")}}}
			var got string
			for event := range Stream(t.Context(), model, req, []ReadTool{tool}) {
				if event.Error != nil {
					t.Fatal(event.Error)
				}
				if event.Type == "text_delta" {
					got += event.Text
				}
			}
			if got != "尾部证据" || rounds != 2 || compactions < 2 {
				t.Fatalf("answer=%q, model turns=%d, batches=%d", got, rounds, compactions)
			}
		})
	}
}
