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

func TestResponsesRetriesWithoutUnavailableSummary(t *testing.T) {
	requests := make(chan string, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
			return
		}
		body := string(payload)
		requests <- body
		if strings.Contains(body, `"summary":"auto"`) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"type\":\"response.created\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\n")
		fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"output\":[{\"type\":\"reasoning\",\"summary\":[{\"type\":\"summary_text\",\"text\":\"final summary\"}]}]}}\n\n")
	}))
	defer server.Close()
	model, err := NewResponses(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	model.host = "api.openai.com" // Exercise the official-host path without calling an external service.
	events, err := model.Stream(t.Context(), &ModelRequest{
		Model: "test-model", ReasoningEffort: "low", IncludeReasoningSummary: true,
		Messages: []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "question"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var summary string
	for event := range events {
		if event.Error != nil {
			t.Fatal(event.Error)
		}
		if event.Type == "reasoning_summary_delta" {
			summary += event.Text
		}
	}
	if len(requests) != 2 || summary != "final summary" {
		t.Fatalf("requests = %d, summary = %q", len(requests), summary)
	}
	first, second := <-requests, <-requests
	if !strings.Contains(first, `"summary":"auto"`) || strings.Contains(second, `"summary"`) || !strings.Contains(second, `"effort":"low"`) {
		t.Fatalf("unexpected requests: %s, %s", first, second)
	}
}

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
				`{"type":"response.output_item.added","item":{"type":"reasoning","id":"think","encrypted_content":"opaque"}}`,
				`{"type":"response.reasoning_text.delta","delta":"Responses thinking"}`,
				`{"type":"response.reasoning_summary_text.delta","delta":"provider summary"}`,
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
				`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"Claude thinking"}}`,
				`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"opaque"}}`,
				`{"type":"content_block_stop","index":0}`,
				`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
				`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"checking"}}`,
				`{"type":"content_block_stop","index":1}`,
				`{"type":"content_block_start","index":2,"content_block":{"type":"tool_use","id":"next","name":"lookup","input":{}}}`,
				`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"{\"q\":"}}`,
				`{"type":"content_block_delta","index":2,"delta":{"type":"input_json_delta","partial_json":"1}"}}`,
				`{"type":"content_block_stop","index":2}`,
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
						case "reasoning_text_delta":
							if tc.name == "responses" && event.Text != "Responses thinking" ||
								tc.name == "messages" && event.Text != "Claude thinking" {
								t.Fatalf("thinking = %q", event.Text)
							}
						case "reasoning_summary_delta":
							if event.Text != "provider summary" {
								t.Fatalf("summary = %q", event.Text)
							}
						case "text_delta":
							if event.Text != "checking" {
								t.Fatalf("visible text = %q", event.Text)
							}
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
					if tc.name == "responses" {
						expected = slices.Insert(expected, 1, "reasoning_text_delta", "reasoning_summary_delta")
					} else {
						expected = slices.Insert(expected, 1, "reasoning_text_delta")
					}
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
