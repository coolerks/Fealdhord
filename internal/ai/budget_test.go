package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBudgetPreservesLatestToolContinuation(t *testing.T) {
	budget := ContextBudget(Metadata{Context: 128, OutputLimit: 24})
	request := ModelRequest{Messages: []Message{
		{Role: "user", Content: []ContentBlock{{Text: strings.Repeat("旧消息", 48)}}},
		{Role: "assistant", Content: []ContentBlock{{Text: "old"}}},
		{Role: "user", Content: []ContentBlock{{Text: "recent"}}},
		{Role: "assistant", ToolCalls: []ToolCall{{ID: "read", Name: "search", ArgumentsJSON: "{}"}}},
		{Role: "tool", ToolCallID: "read", Content: []ContentBlock{{Text: "result"}}},
	}}
	if err := budget.Fit(&request); err != nil || len(request.Messages) != 3 || request.Messages[2].ToolCallID != "read" {
		t.Fatalf("unexpected request: %+v, %v", request, err)
	}
	if *request.MaxOutputTokens != 24 {
		t.Fatal("model limit should bound the output budget")
	}
}

func TestCatalogCanonicalSuffix(t *testing.T) {
	index := NewCatalogIndex(t.TempDir())
	index.models = make(map[string][]Metadata)
	indexEntries(index.models, map[string]directoryEntry{"anthropic/claude-example": {Context: 200000}}, "Models.dev", "")
	got := index.Match("gateway/provider/anthropic/claude-example")
	if got.Context != 200000 || got.Brand != "anthropic" {
		t.Fatalf("unexpected match: %+v", got)
	}
}

func TestLogoRejectsExecutableAndExternalContent(t *testing.T) {
	if !safeLogo([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><path d="M0 0"/></svg>`)) {
		t.Fatal("ordinary logo rejected")
	}
	for _, data := range []string{`<svg onload="alert(1)"/>`, `<svg><script/></svg>`, `<svg><path fill="url(https://example.test/asset)"/></svg>`} {
		if safeLogo([]byte(data)) {
			t.Errorf("accepted unsafe logo: %s", data)
		}
	}
}

func TestProviderCountUsesSameMessagesPayload(t *testing.T) {
	counted := false
	client := &http.Client{Transport: countTransport(func(req *http.Request) (*http.Response, error) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			return nil, err
		}
		counted = req.URL.Path == "/v1/messages/count_tokens" && req.Header.Get("x-api-key") == "fixture-key" &&
			body["messages"] != nil && body["max_tokens"] == nil && body["tools"] != nil
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"input_tokens":23}`)),
			Header: http.Header{"Content-Type": {"application/json"}}}, nil
	})}
	model, err := NewMessages("https://api.anthropic.com/v1", "fixture-key", client)
	if err != nil {
		t.Fatal(err)
	}
	req := ModelRequest{
		Model: "fixture", Budget: ContextBudget(Metadata{Context: 128}),
		Messages: []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "Test"}}}},
		Tools:    []ToolDefinition{{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	}
	if result, err := model.CountInput(context.Background(), &req); err != nil || result != 23 || !counted {
		t.Fatalf("provider token count=%d, valid request=%t, error=%v", result, counted, err)
	}
}

type countTransport func(*http.Request) (*http.Response, error)

func (fn countTransport) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }
