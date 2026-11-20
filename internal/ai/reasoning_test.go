package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type reasoningTransport func(*http.Request) (*http.Response, error)

func (transport reasoningTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestReasoningEffortWire(t *testing.T) {
	tests := []struct {
		name string
		base string
		wire string
	}{
		{"chat", "https://example.test/v1", `"reasoning_effort":"custom_max"`},
		{"openrouter", "https://openrouter.ai/api/v1", `"reasoning":{"effort":"custom_max"}`},
		{"responses", "https://example.test/v1", `"reasoning":{"effort":"custom_max"}`},
		{"messages", "https://example.test/v1", `"output_config":{"effort":"custom_max"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var payload json.RawMessage
			client := &http.Client{Transport: reasoningTransport(func(r *http.Request) (*http.Response, error) {
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}"))}, nil
			})}
			var send func(context.Context, *ModelRequest, bool) (*http.Response, error)
			switch tc.name {
			case "responses":
				model, err := NewResponses(tc.base, "", client)
				if err != nil {
					t.Fatal(err)
				}
				send = model.request
			case "messages":
				model, err := NewMessages(tc.base, "", client)
				if err != nil {
					t.Fatal(err)
				}
				send = model.request
			default:
				model, err := NewChatCompletions(tc.base, "", client)
				if err != nil {
					t.Fatal(err)
				}
				send = model.request
			}
			for _, effort := range []string{"", "custom_max"} {
				request := &ModelRequest{Model: "example", ReasoningEffort: effort,
					Messages: []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "question"}}}}}
				response, err := send(context.Background(), request, true)
				if err != nil {
					t.Fatal(err)
				}
				response.Body.Close()
				if effort != "" {
					if !strings.Contains(string(payload), tc.wire) {
						t.Fatalf("effort missing: %s", payload)
					}
				} else if strings.Contains(string(payload), `"reasoning`) || strings.Contains(string(payload), `"output_config"`) {
					t.Fatalf("default effort serialized: %s", payload)
				}
			}
		})
	}
}
