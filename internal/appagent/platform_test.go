package appagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuiltinToolsHaveDistinctWireNames(t *testing.T) {
	r := Builtins(Services{})
	tools := r.ChatTools("check")
	if len(tools) != 44 {
		t.Fatalf("expected 44 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tool := range tools {
		name := tool.Definition.Name
		if names[name] || !json.Valid(tool.Definition.InputSchema) {
			t.Fatalf("invalid definition for %s", name)
		}
		names[name] = true
	}
}

func TestGuardedToolApprovalSingleUseAndAudit(t *testing.T) {
	r := NewRegistry()
	r.auditDir = t.TempDir()
	Register(r, "item.write", "write", true, func(_ context.Context, input id) id { return input })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	errs := make(chan error, 1)
	go func() {
		result, err := r.execute(ctx, "test", r.tools[0], json.RawMessage(`{"id":"item"}`))
		if err == nil && result != `{"id":"item"}` {
			err = errors.New("wrong result")
		}
		errs <- err
	}()
	for len(r.Pending()) == 0 && ctx.Err() == nil {
		time.Sleep(time.Millisecond)
	}
	pending := r.Pending()
	if len(pending) != 1 || !r.Decide(pending[0].ID, true) || r.Decide(pending[0].ID, true) {
		t.Fatalf("approval must be consumed exactly once: %v", pending)
	}
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
	audit, err := os.ReadFile(filepath.Join(r.auditDir, "tool-audit.jsonl"))
	if err != nil || !strings.Contains(string(audit), `"status":"complete"`) || strings.Contains(string(audit), `"id":"item"`) {
		t.Fatalf("unexpected audit: %s, %v", audit, err)
	}
}

func TestMCPHTTP(t *testing.T) {
	r := NewRegistry()
	Register(r, "item.read", "read", false, func(_ context.Context, q id) id { return q })
	Register(r, "internal.read", "internal", false, func(_ context.Context, q id) id { return q })
	a := NewPlatform(r, t.TempDir())
	digest := sha256.Sum256([]byte("test-secret"))
	config := serverSettings{Config: ServerConfig{Tools: []string{"item.read", "internal.read"}}, Clients: map[string]clientGrant{
		"test": {Digest: hex.EncodeToString(digest[:]), Tools: []string{"item.read"}},
	}}
	server := httptest.NewUnstartedServer(nil)
	host := server.Listener.Addr().String()
	server.Config.Handler = a.handler(config, host, "http")
	server.Start()
	defer server.Close()
	response, err := http.Get(server.URL + "/mcp")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.StatusCode)
	}
	reentrant := &http.Client{Transport: bearerTransport{token: "test-secret"}}
	api := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	session, err := api.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: reentrant}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	list, err := session.ListTools(ctx, nil)
	if err != nil || len(list.Tools) != 1 || list.Tools[0].Name != "item_read" {
		t.Fatalf("unexpected list: %v, %v", list, err)
	}
	output, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "item_read", Arguments: map[string]string{"id": "value"}})
	if err != nil || output.IsError || len(output.Content) != 1 || output.Content[0].(*mcp.TextContent).Text != `{"id":"value"}` {
		t.Fatalf("unexpected response: %v, %v", output, err)
	}
	if denied, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "internal_read"}); err == nil && !denied.IsError {
		t.Fatal("per-client grant allowed an unlisted tool")
	}
}

func TestAllowedPeer(t *testing.T) {
	for _, tc := range []struct {
		external, tls bool
		ip            string
		want          bool
	}{
		{false, true, "192.168.1.1", false},
		{false, false, "127.0.0.1", true},
		{true, false, "192.168.1.1", true},
		{true, false, "8.8.8.8", false},
		{true, true, "8.8.8.8", true},
	} {
		if got := allowedPeer(ServerConfig{External: tc.external, TLS: tc.tls}, netip.MustParseAddr(tc.ip)); got != tc.want {
			t.Errorf("%+v: got %v", tc, got)
		}
	}
}

func TestSkillActivationIntersectsToolAllowlist(t *testing.T) {
	r := NewRegistry()
	Register(r, "article.read", "read", false, func(context.Context, id) id { return id{} })
	Register(r, "article.write", "write", true, func(context.Context, id) id { return id{} })
	p := NewPlatform(r, t.TempDir())
	for _, markdown := range []string{
		"---\nname: one\ndescription: read\nallowed-tools: article.read article.write\n---\nFirst",
		"---\nname: two\ndescription: read\nallowed-tools: article.read\n---\nSecond",
	} {
		if _, err := p.saveSkill(markdown, true); err != nil {
			t.Fatal(err)
		}
	}
	tools, prompt, err := p.RunTools("id", []string{"one", "two"}, false)
	if err != nil || len(tools) != 1 || tools[0].Definition.Name != "article_read" || !strings.Contains(prompt, "Second") {
		t.Fatalf("unexpected activation: %v %q %v", tools, prompt, err)
	}
}
