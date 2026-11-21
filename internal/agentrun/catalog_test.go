package agentrun

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/coolerks/Fealdhord/internal/ai"
)

func TestCompactCatalogKeepsAuthorizedOperations(t *testing.T) {
	var called ai.ToolCall
	originals := make([]ReadTool, 0, 4)
	for _, name := range []string{"article_search", "feed_list", "category_list", "article_favorite"} {
		originals = append(originals, ReadTool{
			Definition: ai.ToolDefinition{Name: name, Description: strings.Repeat("允许访问", 48), InputSchema: json.RawMessage(`{"type":"object"}`)},
			Read:       func(_ context.Context, call ai.ToolCall) (string, error) { called = call; return `{"ok":true}`, nil },
		})
	}
	compact := CompactToolCatalog(ai.ModelRequest{Budget: ai.Budget{InputLimit: 1200}}, originals)
	if len(compact) != 2 {
		t.Fatalf("expected compact catalog, got %d tools", len(compact))
	}
	ctx := context.Background()
	result, err := compact[0].Read(ctx, ai.ToolCall{ArgumentsJSON: `{"name":"article_search"}`})
	if err != nil || !strings.Contains(result, "inputSchema") {
		t.Fatalf("tool schema not available: %q %v", result, err)
	}
	_, err = compact[1].Read(ctx, ai.ToolCall{ID: "call-1", ArgumentsJSON: `{"name":"article_favorite","arguments":{"id":"item"}}`})
	if err != nil || called.Name != "article_favorite" || called.ID != "call-1" || called.ArgumentsJSON != `{"id":"item"}` {
		t.Fatalf("lost actual invocation: %+v %v", called, err)
	}
	result, err = compact[1].Read(ctx, ai.ToolCall{ArgumentsJSON: `{"name":"unregistered","arguments":{}}`})
	if err != nil || !strings.Contains(result, "ToolNotAllowed") {
		t.Fatalf("unregistered operation accepted: %q %v", result, err)
	}
}
