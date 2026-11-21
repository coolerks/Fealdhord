package appagent

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/library"
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

func TestScopedToolsCheckRealArticleMembership(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, filepath.Join(t.TempDir(), "scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	feedID, err := store.EnsureFeed(ctx, "https://example.test/allowed", "allowed")
	if err != nil {
		t.Fatal(err)
	}
	otherFeed, err := store.EnsureFeed(ctx, "https://example.test/excluded", "excluded")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: feedID, GUID: "1", Title: "允许的文章", Summary: "唯一关键词"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.UpsertArticle(ctx, database.ArticleInput{FeedID: otherFeed, GUID: "2", Title: "边界外文章", Summary: "唯一关键词"})
	if err != nil {
		t.Fatal(err)
	}
	r := Builtins(Services{Store: store, Library: library.NewService(store, nil)})
	scope := MCPScope{Kind: "feed", ID: feedID}
	for _, operation := range r.tools {
		switch operation.Name {
		case "article.search":
			result, err := r.execute(ctx, "test", r.scopedTool(operation, scope), json.RawMessage(`{"search":"唯一关键词"}`))
			if err != nil || !strings.Contains(result, first) || strings.Contains(result, second) {
				t.Fatalf("search escaped scope: %q %v", result, err)
			}
		case "article.get_metadata":
			outside, _ := json.Marshal(id{ID: second})
			if result, err := r.execute(ctx, "test", r.scopedTool(operation, scope), outside); err == nil || result != "" {
				t.Fatalf("read escaped scope %q %v", result, err)
			}
		}
	}
}

func TestSkillPackageAcceptsTextAndRejectsTraversal(t *testing.T) {
	for _, tc := range []struct {
		name, resource string
		valid          bool
	}{
		{"reference", "fixture/references/faq.md", true},
		{"escape", "fixture/references/../outside.md", false},
		{"executable", "fixture/assets/program.py", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "skill.zip")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			zipper := zip.NewWriter(file)
			for name, contents := range map[string]string{
				"fixture/SKILL.md": "---\nname: fixture\ndescription: fixture\n---\nUse FAQ",
				tc.resource:        "fixture text",
			} {
				entry, err := zipper.Create(name)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = io.WriteString(entry, contents); err != nil {
					t.Fatal(err)
				}
			}
			if err := errors.Join(zipper.Close(), file.Close()); err != nil {
				t.Fatal(err)
			}
			got, err := loadSkillFile(path)
			if tc.valid {
				if err != nil || len(got.Resources) != 1 || got.Resources[0].Path != "references/faq.md" {
					t.Fatalf("invalid package: %+v %v", got, err)
				}
			} else if err == nil {
				t.Fatal("unsafe package accepted")
			}
		})
	}
}

func TestMCPServerHTTPS(t *testing.T) {
	prototype := httptest.NewTLSServer(nil)
	defer prototype.Close()
	certificate := prototype.TLS.Certificates[0]
	private, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	for _, value := range []struct {
		path, kind string
		data       []byte
	}{
		{certPath, "CERTIFICATE", certificate.Certificate[0]},
		{keyPath, "PRIVATE KEY", private},
	} {
		if err := os.WriteFile(value.path, pem.EncodeToMemory(&pem.Block{Type: value.kind, Bytes: value.data}), 0600); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	p := NewPlatform(NewRegistry(), dir)
	defer p.Shutdown()
	if _, err := p.RotateMCPToken(); err != nil {
		t.Fatal(err)
	}
	err = p.SaveMCPServer(ServerConfig{Enabled: true, Port: port, TLS: true, Certificate: certPath, PrivateKey: keyPath})
	if err != nil {
		t.Fatal(err)
	}
	endpoint := p.MCPServer().URL
	response, err := prototype.Client().Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !strings.HasPrefix(endpoint, "https://127.0.0.1:") || response.TLS == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("HTTPS handshake or authentication failed: %s %d", endpoint, response.StatusCode)
	}
}
