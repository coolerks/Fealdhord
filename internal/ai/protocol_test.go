package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestProtocolToolRoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		newModel     func(string, string, *http.Client) (Model, error)
		requestParts []string
		reply        string
		stream       []string
	}{
		{
			name:         "responses",
			newModel:     func(base, key string, client *http.Client) (Model, error) { return NewResponses(base, key, client) },
			requestParts: []string{`"instructions":"system"`, `"type":"function_call_output"`, `"call_id":"previous"`, `"store":false`},
			reply:        `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"checking"}]},{"type":"function_call","id":"item","call_id":"next","name":"lookup","arguments":"{\"q\":1}"}],"usage":{"input_tokens":5,"output_tokens":3}}`,
			stream: []string{
				`{"type":"response.created"}`,
				`{"type":"response.future.delta","delta":{"nested":true}}`,
				`{"type":"response.output_text.delta","delta":"checking"}`,
				`{"type":"response.output_item.added","item":{"type":"function_call","id":"item","call_id":"next","name":"lookup","arguments":""}}`,
				`{"type":"response.function_call_arguments.delta","item_id":"item","delta":"{\"q\":"}`,
				`{"type":"response.function_call_arguments.delta","item_id":"item","delta":"1}"}`,
			},
		},
		{
			name:         "messages",
			newModel:     func(base, key string, client *http.Client) (Model, error) { return NewMessages(base, key, client) },
			requestParts: []string{`"system":"system"`, `"type":"tool_result"`, `"tool_use_id":"previous"`, `"max_tokens":4096`},
			reply:        `{"type":"message","role":"assistant","content":[{"type":"text","text":"checking"},{"type":"tool_use","id":"next","name":"lookup","input":{"q":1}}],"stop_reason":"tool_use","usage":{"input_tokens":5,"output_tokens":3}}`,
			stream: []string{
				`{"type":"message_start","message":{"type":"message","role":"assistant","usage":{"input_tokens":5,"output_tokens":1}}}`,
				`{"type":"ping"}`,
				`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"checking"}}`,
				`{"type":"content_block_stop","index":0}`,
				`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"next","name":"lookup","input":{}}}`,
				`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`,
				`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"1}"}}`,
				`{"type":"content_block_stop","index":1}`,
				`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":3}}`,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, truncated := range []bool{false, true} {
				t.Run(fmt.Sprint("truncated=", truncated), func(t *testing.T) {
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if r.URL.Path != "/custom/v1/"+tc.name {
							t.Errorf("path = %s", r.URL.Path)
						}
						if tc.name == "messages" {
							if r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("x-api-key") != "test-key" || r.Header.Get("Authorization") != "" {
								t.Error("wrong Messages headers")
							}
						} else if r.Header.Get("Authorization") != "Bearer test-key" {
							t.Error("wrong Responses headers")
						}
						var payload json.RawMessage
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
							t.Error(err)
						}
						for _, part := range tc.requestParts {
							if !strings.Contains(string(payload), part) {
								t.Errorf("payload missing %s: %s", part, payload)
							}
						}
						if !strings.Contains(string(payload), `"stream":true`) {
							fmt.Fprint(w, tc.reply)
							return
						}
						w.Header().Set("Content-Type", "text/event-stream")
						for _, event := range tc.stream {
							fmt.Fprintf(w, "data: %s\n\n", event)
						}
						if !truncated {
							if tc.name == "responses" {
								fmt.Fprintf(w, "data: {\"type\":\"response.completed\",\"response\":%s}\n\n", tc.reply)
							} else {
								fmt.Fprint(w, "data: {\"type\":\"message_stop\"}\n\n")
							}
						}
					}))
					defer server.Close()
					model, err := tc.newModel(server.URL+"/custom/v1", "test-key", server.Client())
					if err != nil {
						t.Fatal(err)
					}
					req := &ModelRequest{Model: "example", System: "system", Tools: []ToolDefinition{{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}}, Messages: []Message{
						{Role: "user", Content: []ContentBlock{{Type: "text", Text: "look up"}}},
						{Role: "assistant", ToolCalls: []ToolCall{{ID: "previous", Name: "lookup", ArgumentsJSON: "{}"}}},
						{Role: "tool", ToolCallID: "previous", Content: []ContentBlock{{Type: "text", Text: "result"}}},
					}}
					result, err := model.Generate(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					want := ToolCall{ID: "next", Name: "lookup", ArgumentsJSON: `{"q":1}`}
					if !slices.Equal(result.Message.ToolCalls, []ToolCall{want}) || result.FinishReason != "tool_calls" || result.Message.Content[0].Text != "checking" {
						t.Fatalf("result = %+v", result)
					}
					events, err := model.Stream(context.Background(), req)
					if err != nil {
						t.Fatal(err)
					}
					var types []string
					var messageID string
					for event := range events {
						types = append(types, event.Type)
						if messageID == "" {
							messageID = event.MessageID
						}
						if event.Sequence != uint64(len(types)) || event.MessageID == "" || event.MessageID != messageID {
							t.Fatalf("event identity = %+v", event)
						}
						switch event.Type {
						case "tool_call_end":
							if event.ToolCall == nil || *event.ToolCall != want {
								t.Fatalf("call = %+v", event.ToolCall)
							}
						case "usage":
							if event.Usage.TotalTokens == nil || *event.Usage.TotalTokens != 8 {
								t.Fatalf("usage = %+v", event.Usage)
							}
						case "error":
							if event.Error.Error() != "ProviderProtocolError" {
								t.Fatal(event.Error)
							}
						}
					}
					expected := []string{"message_start", "text_delta", "tool_call_start", "tool_call_arguments_delta", "tool_call_arguments_delta"}
					if truncated {
						expected = append(expected, "error")
					} else {
						expected = append(expected, "tool_call_end", "usage", "message_end")
					}
					if !slices.Equal(types, expected) {
						t.Fatalf("events = %v, want %v", types, expected)
					}
				})
			}
		})
	}
}
