package appagent

import (
	"archive/zip"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/coolerks/Fealdhord/internal/database"
	"github.com/coolerks/Fealdhord/internal/library"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/zalando/go-keyring"
)

func TestBuiltinToolsHaveDistinctWireNames(t *testing.T) {
	r := Builtins(Services{})
	tools := r.ChatTools("check")
	if len(tools) != 45 {
		t.Fatalf("expected 45 tools, got %d", len(tools))
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

func TestChatApprovalModes(t *testing.T) {
	registry := NewRegistry()
	Register(registry, "item.write", "write", true, func(_ context.Context, input id) id { return input })
	tool := registry.ChatTools("conversation")[0]
	call := ai.ToolCall{Name: tool.Definition.Name, ArgumentsJSON: `{"id":"item"}`}
	for _, test := range []struct{ mode, want string }{
		{agentrun.ApprovalBypass, `{"id":"item"}`}, {agentrun.ApprovalDeny, `{"error":"PermissionDenied"}`},
	} {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		result, err := tool.Read(agentrun.WithApprovalMode(ctx, test.mode), call)
		cancel()
		if err != nil || result != test.want || len(registry.Pending()) != 0 {
			t.Fatalf("%s: result=%s pending=%v err=%v", test.mode, result, registry.Pending(), err)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	done := make(chan string, 1)
	go func() {
		result, _ := tool.Read(agentrun.WithApprovalMode(ctx, agentrun.ApprovalAsk), call)
		done <- result
	}()
	for len(registry.Pending()) == 0 && ctx.Err() == nil {
		time.Sleep(time.Millisecond)
	}
	pending := registry.Pending()
	if len(pending) != 1 || pending[0].ConversationID != "conversation" || !registry.Decide(pending[0].ID, true) {
		t.Fatalf("expected a chat-scoped approval: %v", pending)
	}
	if result := <-done; result != `{"id":"item"}` {
		t.Fatalf("approved result = %s", result)
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
	if err != nil || len(tools) != 2 || tools[0].Definition.Name != "article_read" || tools[1].Definition.Name != "read" || strings.Contains(prompt, "Second") || !strings.Contains(prompt, "skill://two/SKILL.md") {
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
	const standard = "---\nname: fixture\ndescription: fixture\n---\nUse FAQ"
	for _, tc := range []struct {
		name, root, markdown, resource, wantError string
		extraResourceBytes                        int
	}{
		{name: "wrapped", root: "fixture/", markdown: standard, resource: "references/faq.md"},
		{name: "UTF-8 BOM and CRLF", markdown: "\uFEFF" + strings.ReplaceAll(standard, "\n", "\r\n"), resource: "references/faq.md"},
		{name: "escape", markdown: standard, resource: "references/../outside.md", wantError: "InvalidSkillPackage"},
		{name: "executable", markdown: standard, resource: "scripts/program.py"},
		{name: "larger reference", markdown: standard, resource: "references/faq.md", extraResourceBytes: 300 << 10},
		{name: "large manifest", markdown: standard + strings.Repeat("a", maxSkillText), resource: "references/faq.md", wantError: "SkillTextTooLarge"},
		{name: "malformed YAML", markdown: strings.Replace(standard, "name: fixture", "name: [", 1), resource: "references/faq.md", wantError: "SkillFrontMatterInvalid"},
		{name: "invalid name", markdown: strings.Replace(standard, "name: fixture", "name: FAQ", 1), resource: "references/faq.md", wantError: "SkillNameInvalid"},
		{name: "missing description", markdown: strings.Replace(standard, "description: fixture\n", "", 1), resource: "references/faq.md", wantError: "SkillDescriptionInvalid"},
		{name: "invalid UTF-8", markdown: standard + "\xff", resource: "references/faq.md", wantError: "SkillEncodingInvalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "skill.zip")
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			zipper := zip.NewWriter(file)
			files := [][2]string{{"SKILL.md", tc.markdown}, {tc.resource, "fixture text"}}
			if tc.extraResourceBytes > 0 {
				files = append(files, [2]string{"references/extra.txt", strings.Repeat("a", tc.extraResourceBytes)})
			}
			for _, file := range files {
				entry, err := zipper.Create(tc.root + file[0])
				if err != nil {
					t.Fatal(err)
				}
				if _, err = io.WriteString(entry, file[1]); err != nil {
					t.Fatal(err)
				}
			}
			if err := errors.Join(zipper.Close(), file.Close()); err != nil {
				t.Fatal(err)
			}
			got, err := extractSkillPackage(t.Context(), path, t.TempDir())
			if tc.wantError != "" {
				if err == nil || err.Error() != tc.wantError {
					t.Fatalf("want %s, got %v", tc.wantError, err)
				}
				return
			}
			if err != nil || got.Markdown != standard || len(got.Resources) == 0 || got.Resources[0] != (SkillResource{Path: tc.resource, Size: int64(len("fixture text"))}) {
				t.Fatalf("invalid package: %+v %v", got, err)
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
	httpConfig := ServerConfig{Enabled: true, Port: port}
	if err := p.SaveMCPServer(httpConfig); err != nil {
		t.Fatal(err)
	}
	previous := p.MCPServer().URL
	badCertPath := filepath.Join(dir, "other-host.pem")
	other := &x509.Certificate{
		SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		DNSNames: []string{"elsewhere.test"},
	}
	signer := certificate.PrivateKey.(crypto.Signer)
	encoded, err := x509.CreateCertificate(rand.Reader, other, other, signer.Public(), certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badCertPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: encoded}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := p.SaveMCPServer(ServerConfig{Enabled: true, Port: port, TLS: true, Certificate: badCertPath, PrivateKey: keyPath}); err == nil || err.Error() != "InvalidTLSCertificate" {
		t.Fatalf("expected wrong-host certificate rejection, got %v", err)
	}
	path := filepath.Join(dir, "mcp-server.json")
	if err := os.Rename(path, path+".backup"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	httpsConfig := ServerConfig{Enabled: true, Port: port, TLS: true, Certificate: certPath, PrivateKey: keyPath}
	if err := p.SaveMCPServer(httpsConfig); err == nil || err.Error() != "StorageError" {
		t.Fatalf("expected settings write failure, got %v", err)
	}
	if current := p.MCPServer().URL; current != previous {
		t.Fatalf("failed save changed endpoint: %q", current)
	}
	httpClient := &http.Client{Timeout: 3 * time.Second}
	response, err := httpClient.Get(previous)
	if err != nil {
		t.Fatalf("old HTTP listener unavailable: %v", err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old listener returned %d", response.StatusCode)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := p.SaveMCPServer(httpsConfig); err != nil {
		t.Fatal(err)
	}
	response, err = httpClient.Get(previous)
	if err != nil {
		t.Fatalf("existing HTTP connection: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("plaintext connection survived TLS activation: %d", response.StatusCode)
	}

	endpoint := p.MCPServer().URL
	response, err = prototype.Client().Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !strings.HasPrefix(endpoint, "https://127.0.0.1:") || response.TLS == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("HTTPS handshake or authentication failed: %s %d", endpoint, response.StatusCode)
	}
	if err := os.Remove(certPath); err != nil {
		t.Fatal(err)
	}
	if _, err := p.RotateMCPToken(); err != nil {
		t.Fatalf("credential changes should use the loaded certificate: %v", err)
	}
}

func TestMCPGrantUpdateKeepsTokenUntilExplicitRotation(t *testing.T) {
	r := NewRegistry()
	Register(r, "item.read", "read", false, func(_ context.Context, q id) id { return q })
	Register(r, "other.read", "other", false, func(_ context.Context, q id) id { return q })
	dir := t.TempDir()
	p := NewPlatform(r, dir)
	defer p.Shutdown()
	bound, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := bound.Addr().(*net.TCPAddr).Port
	bound.Close()
	config := ServerConfig{Port: port, Tools: []string{"item.read", "other.read"}}
	if err := p.SaveMCPServer(config); err != nil {
		t.Fatal(err)
	}
	grant := MCPGrant{Name: "reader", Tools: []string{"item.read"}}
	token, err := p.SaveGrant(grant, false)
	if err != nil {
		t.Fatal(err)
	}
	config.Enabled = true
	if err := p.SaveMCPServer(config); err != nil {
		t.Fatal(err)
	}
	assertTools := func(platform *Platform, token, expected string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
			Endpoint: platform.MCPServer().URL, HTTPClient: &http.Client{Transport: bearerTransport{token: token}},
		}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		result, err := session.ListTools(ctx, nil)
		if err != nil || len(result.Tools) != 1 || result.Tools[0].Name != expected {
			t.Fatalf("expected only %s, got %v, %v", expected, result, err)
		}
	}
	assertTools(p, token, "item_read")
	grant.Tools = []string{"other.read"}
	if _, err := p.SaveGrant(grant, false); err == nil || err.Error() != "ClientGrantExists" {
		t.Fatalf("duplicate creation returned %v", err)
	}
	if err := p.UpdateGrant(grant); err != nil {
		t.Fatal(err)
	}
	assertTools(p, token, "other_read")
	p.Shutdown()
	reopened := NewPlatform(r, dir)
	defer reopened.Shutdown()
	assertTools(reopened, token, "other_read")
	replacement, err := reopened.RotateGrantToken(grant.Name)
	if err != nil || replacement == token {
		t.Fatalf("expected a replacement token, got error %v", err)
	}
	assertTools(reopened, replacement, "other_read")
	oldClient := &http.Client{Transport: bearerTransport{token: token}, Timeout: 3 * time.Second}
	response, err := oldClient.Get(reopened.MCPServer().URL)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("old token returned %d", response.StatusCode)
	}
}

func TestMCPRevokesOnlyClientAndRemainsClosedToItAfterRestart(t *testing.T) {
	bound, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := bound.Addr().(*net.TCPAddr).Port
	bound.Close()
	dir := t.TempDir()
	p := NewPlatform(NewRegistry(), dir)
	defer p.Shutdown()
	config := ServerConfig{Enabled: true, Port: port}
	if err := p.SaveMCPServer(config); err == nil || err.Error() != "CreateCredentialFirst" {
		t.Fatalf("initial enable should require credentials: %v", err)
	}
	token, err := p.SaveGrant(MCPGrant{Name: "only"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.SaveMCPServer(config); err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{Transport: bearerTransport{token: token}, Timeout: 3 * time.Second}
	assertRevoked := func(p *Platform) {
		t.Helper()
		state := p.MCPServer()
		if !state.Config.Enabled || state.URL == "" || len(p.Grants()) != 0 {
			t.Fatalf("unexpected state: %+v, grants: %v", state, p.Grants())
		}
		response, err := httpClient.Get(state.URL)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusUnauthorized {
			t.Fatalf("revoked token returned %d", response.StatusCode)
		}
	}
	if _, err := p.SaveGrant(MCPGrant{Name: "only"}, true); err != nil {
		t.Fatal(err)
	}
	assertRevoked(p)
	p.Shutdown()
	reopened := NewPlatform(NewRegistry(), dir)
	defer reopened.Shutdown()
	assertRevoked(reopened)
}

type mockMCPClientCredentials map[string]string

func (c mockMCPClientCredentials) Get(ref string) (string, error) {
	value, ok := c[ref]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}

func (c mockMCPClientCredentials) Set(ref, value string) error {
	c[ref] = value
	return nil
}

func (c mockMCPClientCredentials) Delete(ref string) error {
	delete(c, ref)
	return nil
}

func TestMCPClientSettingsRejectUnreadableAndOversizedChanges(t *testing.T) {
	dir := t.TempDir()
	p := NewPlatform(NewRegistry(), dir)
	old := []byte(`{"example":{"name":"example","command":"old"}`)
	path := filepath.Join(dir, "mcp-clients.json")
	if err := os.WriteFile(path, old, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := p.MCPClients(); err == nil {
		t.Fatal("corrupt settings reported as empty")
	}
	if err := p.ForgetMCP("example"); err == nil {
		t.Fatal("removing a connection would overwrite corrupt settings")
	}
	if _, err := p.ConnectMCP(t.Context(), ClientConfig{Name: "example", Command: "does-not-exist"}); err == nil || err.Error() != "StorageError" {
		t.Fatalf("connect should stop before opening a new connection: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != string(old) {
		t.Fatalf("existing file changed: %v", err)
	}
	prior := ClientConfig{Name: "example", Endpoint: "https://example.test/mcp"}
	values := map[string]ClientConfig{prior.Name: prior}
	if err := atomicJSON(dir, "mcp-clients.json", values); err != nil {
		t.Fatal(err)
	}
	large := ClientConfig{Name: "large", Command: "test", Args: []string{strings.Repeat("x", maxMCPClientSettings)}}
	if err := p.persistClientSecrets(large, values, clientSecrets{}); err == nil || err.Error() != "ClientSettingsTooLarge" {
		t.Fatalf("oversized settings write: %v", err)
	}
	clients, err := p.MCPClients()
	if err != nil || len(clients) != 1 || clients[0].Name != prior.Name {
		t.Fatalf("saved clients: %+v, %v", clients, err)
	}
	ref := clientCredentialRef(prior)
	credentials := mockMCPClientCredentials{ref: "old-key"}
	p.credentials = credentials
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	prior.Bearer = "new-key"
	delete(values, "large")
	if err := p.persistClientSecrets(prior, values, clientSecrets{Bearer: prior.Bearer}); err == nil || credentials[ref] != "old-key" {
		t.Fatalf("failed save should preserve old credential: %v", err)
	}
}

func TestSkillPackageRequiresInstructionsAndUsesSavedBody(t *testing.T) {
	p := NewPlatform(NewRegistry(), t.TempDir())
	const header = "---\nname: my-skill\ndescription: 从用户提供的材料中提炼要点\n---\n"
	if _, err := NewAPI(p, nil).SaveSkillPackage(Skill{Markdown: header, Enabled: true}); err == nil || err.Error() != "SkillBodyRequired" {
		t.Fatalf("empty body validation: %v", err)
	}
	const body = "用简洁的中文列出主要观点。材料不足时，指出缺少的信息。"
	if _, err := NewAPI(p, nil).SaveSkillPackage(Skill{Markdown: header + "\n" + body + "\n", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	tools, instructions, err := p.RunTools("chat", []string{"my-skill"}, false)
	if err != nil || strings.Contains(instructions, body) {
		t.Fatalf("Skill body disclosed in initial prompt: %q %v", instructions, err)
	}
	page, err := tools[0].Read(t.Context(), ai.ToolCall{ArgumentsJSON: `{"path":"skill://my-skill/SKILL.md"}`})
	if err != nil || !strings.Contains(page, body) {
		t.Fatalf("saved Skill unavailable through read: %q %v", page, err)
	}
}

func TestLargePreviouslySavedSkillCanBeListedAndToggled(t *testing.T) {
	p := NewPlatform(NewRegistry(), t.TempDir())
	markdown := "---\nname: big-skill\ndescription: test\n---\n" + strings.Repeat("<tag>", 40000)
	old, err := parseSkill(markdown, true)
	if err != nil {
		t.Fatal(err)
	}
	old.Resources = []SkillResource{{Path: "references/example.txt", Text: "保留引用"}}
	data, err := json.Marshal(old)
	if err != nil || len(data) <= 512<<10 {
		t.Fatalf("expected encoded fixture beyond the old limit: %d, %v", len(data), err)
	}
	if err := atomicFile(filepath.Join(p.dir, "skills"), old.Name+".json", data); err != nil {
		t.Fatal(err)
	}
	if skills := p.Skills(); len(skills) != 1 || skills[0].Name != old.Name {
		t.Fatalf("expected previously stored Skill to be readable, got %d entries", len(skills))
	}
	saved, err := p.saveSkill(markdown, false)
	if err != nil || saved.Enabled || !slices.Equal(saved.Resources, old.Resources) {
		t.Fatalf("toggle should retain references, error: %v", err)
	}
}
