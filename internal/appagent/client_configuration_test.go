package appagent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPUnicodeRenameKeepsLegacyCredentialAndPagedTools(t *testing.T) {
	var calls atomic.Int32
	m := mcp.NewServer(&mcp.Implementation{Name: "paged", Version: "1"}, &mcp.ServerOptions{PageSize: 7})
	for i := range 47 {
		mcp.AddTool(m, &mcp.Tool{Name: fmt.Sprintf("tool_%02d", i)}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			calls.Add(1)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil, nil
		})
	}
	handle := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return m }, nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer legacy-test" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handle.ServeHTTP(w, r)
	}))
	defer server.Close()
	dir := t.TempDir()
	platform := NewPlatform(NewRegistry(), dir)
	defer platform.Shutdown()
	old := ClientConfig{Name: "legacy", Endpoint: server.URL}
	oldRef := clientCredentialRef(old)
	platform.credentials = mockMCPClientCredentials{oldRef: "legacy-test"}
	if err := atomicJSON(dir, "mcp-clients.json", map[string]ClientConfig{old.Name: old}); err != nil {
		t.Fatal(err)
	}
	// This is the permission key written by versions that used the display name as identity.
	legacyIdentity := fmt.Sprintf(`{"Connection":"legacy","Endpoint":%q,"Command":"","Kind":"tool","Name":"tool_46"}`, server.URL)
	previousID := fmt.Sprintf("mcp:%x", sha256.Sum256([]byte(legacyIdentity)))
	first, err := platform.ConnectMCP(t.Context(), old)
	if err != nil || first.ID != "legacy" || len(first.Tools) != 47 || first.Config.ToolTimeout != 120 {
		t.Fatalf("old connection: %+v %v", first, err)
	}
	if err := platform.SaveToolPermission(previousID, agentrun.ApprovalBypass); err != nil {
		t.Fatal(err)
	}
	renamed := first.Config
	renamed.Name = "投资经理"
	second, err := platform.ConnectMCP(t.Context(), renamed)
	if err != nil || second.ID != first.ID || second.Name != renamed.Name || len(second.Tools) != 47 {
		t.Fatalf("renamed connection: %+v %v", second, err)
	}
	list, err := platform.MCPClients()
	if err != nil || len(list) != 1 {
		t.Fatalf("rename left a duplicate: %+v %v", list, err)
	}
	wire := clientToolName(second.ID, "tool_46", "tool")
	tools := platform.ChatTools("chat")
	index := slices.IndexFunc(tools, func(tool agentrun.ReadTool) bool { return tool.Definition.Name == wire })
	if index == -1 {
		t.Fatal("tool on final page missing")
	}
	ctx, cancel := context.WithTimeout(customToolContext(t, platform), 2*time.Second)
	defer cancel()
	if result, err := tools[index].Read(ctx, ai.ToolCall{ArgumentsJSON: "{}"}); err != nil || calls.Load() != 1 || !strings.Contains(result, "ok") {
		t.Fatalf("final-page tool was not called: %q %v", result, err)
	}
	file, _ := os.ReadFile(filepath.Join(dir, "mcp-clients.json"))
	if strings.Contains(string(file), "legacy-test") || second.Config.SecretRef != "" || second.Config.Bearer != "" {
		t.Fatal("credential leaked through persisted settings or API")
	}
}

func TestMCPHTTPConfigurationAndConnectionRetry(t *testing.T) {
	t.Setenv("FEALDHORD_TEST_HEADER", "header-test-secret")
	var unavailable atomic.Bool
	unavailable.Store(true)
	var requests atomic.Int32
	m := mcp.NewServer(&mcp.Implementation{Name: "http", Version: "1"}, nil)
	base := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return m }, nil)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Example") != "header-test-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			requests.Add(1)
			if unavailable.Swap(false) {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
		}
		base.ServeHTTP(w, r)
	}))
	defer server.Close()
	t.Setenv("FEALDHORD_TEST_HTTP", server.URL)
	platform := NewPlatform(NewRegistry(), t.TempDir())
	platform.credentials = mockMCPClientCredentials{}
	defer platform.Shutdown()
	one := 1
	config := ClientConfig{Name: "远程连接", Endpoint: "${FEALDHORD_TEST_HTTP}",
		Headers: []ClientValue{{Name: "X-Example", Value: "${FEALDHORD_TEST_HEADER}"}}, ConnectionRetries: &one}
	if _, err := platform.ConnectMCP(t.Context(), config); err == nil || requests.Load() != 0 {
		t.Fatalf("HTTPS accepted an untrusted certificate: %v", err)
	}
	config.SkipTLSVerify = true
	status, err := platform.ConnectMCP(t.Context(), config)
	if err != nil || requests.Load() < 2 || status.Config.Endpoint != config.Endpoint || !status.Config.Headers[0].Stored {
		t.Fatalf("configured connection or retry failed: %+v %v (%d tries)", status, err, requests.Load())
	}
	if strings.Contains(status.Config.Headers[0].Value, "header-test-secret") {
		t.Fatal("header secret was returned")
	}
	t.Setenv("FEALDHORD_TEST_HEADER", "new-value")
	if _, err := platform.ConnectMCP(t.Context(), status.Config); err == nil {
		t.Fatal("reconnect did not expand the changed system environment")
	}
	if list, err := platform.MCPClients(); err != nil || len(list) != 1 || !list[0].Connected {
		t.Fatalf("failed reconnect replaced live connection: %+v %v", list, err)
	}
}

func TestMCPStdioEnvironmentWorkingDirectoryAndAttemptLogs(t *testing.T) {
	t.Setenv("FEALDHORD_TEST_STDIO_TOKEN", "stdio-test-secret")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	platform := NewPlatform(NewRegistry(), t.TempDir())
	platform.credentials = mockMCPClientCredentials{}
	defer platform.Shutdown()
	dir := t.TempDir()
	config := ClientConfig{Name: "测试进程", Command: exe, Args: []string{"-test.run=^TestMCPStdioChild$"},
		Environment: []ClientValue{
			{Name: "FEALDHORD_MCP_CHILD", Value: "ready"},
			{Name: "FEALDHORD_CHILD_VALUE", Value: "${FEALDHORD_TEST_STDIO_TOKEN}"},
		}, WorkingDirectory: dir, AttemptID: "stdio-ready"}
	status, err := platform.ConnectMCP(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	tools := platform.ChatTools("chat")
	if len(tools) != 1 {
		t.Fatalf("STDIO tool catalog: %d", len(tools))
	}
	if result, err := tools[0].Read(agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalBypass), ai.ToolCall{ArgumentsJSON: "{}"}); err != nil || !strings.Contains(result, "logged") {
		t.Fatalf("STDIO logging tool response: %q %v", result, err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !slices.ContainsFunc(platform.MCPClientLogs(status.ID, false), func(entry MCPClientLog) bool { return entry.Source == "mcp" }) {
		time.Sleep(time.Millisecond)
	}
	platform.DisconnectMCP(status.ID)
	logs := platform.MCPClientLogs(status.ID, false)
	data, _ := json.Marshal(logs)
	if !strings.Contains(string(data), "启动目录="+filepath.Base(dir)) ||
		!strings.Contains(string(data), "[已遮盖]") || strings.Contains(string(data), "stdio-test-secret") ||
		!strings.Contains(string(data), "structured warning") {
		t.Fatalf("missing or unredacted stderr: %s", data)
	}

	config.Name, config.AttemptID = "启动失败", "stdio-failed"
	config.Environment[0].Value = "fail"
	if _, err := platform.ConnectMCP(t.Context(), config); err == nil {
		t.Fatal("startup failure reported as connected")
	}
	previous, _ := json.Marshal(platform.MCPClientLogs(status.ID, false))
	failed, _ := json.Marshal(platform.MCPClientLogs("stdio-failed", true))
	if strings.Contains(string(previous), "fixture exited unexpectedly") || !strings.Contains(string(failed), "fixture exited unexpectedly") ||
		strings.Contains(string(failed), "stdio-test-secret") {
		t.Fatalf("attempt logs mixed or leaked: %s / %s", previous, failed)
	}
}

func TestMCPStdioChild(t *testing.T) {
	mode := os.Getenv("FEALDHORD_MCP_CHILD")
	if mode == "" {
		return
	}
	if mode == "fail" {
		fmt.Fprintln(os.Stderr, "fixture exited unexpectedly", os.Getenv("FEALDHORD_CHILD_VALUE"))
		os.Exit(2)
	}
	if mode == "hang" {
		fmt.Fprintln(os.Stderr, "fixture waiting for timeout")
		time.Sleep(10 * time.Second)
		os.Exit(2)
	}
	dir, _ := os.Getwd()
	fmt.Fprintln(os.Stderr, "启动目录="+filepath.Base(dir), os.Getenv("FEALDHORD_CHILD_VALUE"))
	m := mcp.NewServer(&mcp.Implementation{Name: "stdio", Version: "1"}, &mcp.ServerOptions{
		Capabilities: &mcp.ServerCapabilities{Logging: &mcp.LoggingCapabilities{}},
	})
	m.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			if method == "server/discover" {
				return nil, &jsonrpc.Error{Code: jsonrpc.CodeMethodNotFound, Message: "legacy test server"}
			}
			return next(ctx, method, request)
		}
	})
	mcp.AddTool(m, &mcp.Tool{Name: "log"}, func(ctx context.Context, request *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if err := request.Session.Log(ctx, &mcp.LoggingMessageParams{Level: "warning", Data: "structured warning"}); err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "logged"}}}, nil, nil
	})
	if err := m.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestMCPStdioStartupTimeoutAndLogCapacity(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	platform := NewPlatform(NewRegistry(), t.TempDir())
	start := time.Now()
	_, err = platform.ConnectMCP(t.Context(), ClientConfig{
		Name: "等待超时", Command: exe, Args: []string{"-test.run=^TestMCPStdioChild$"}, StartupTimeout: 1, AttemptID: "timeout",
		Environment: []ClientValue{{Name: "FEALDHORD_MCP_CHILD", Value: "hang"}},
	})
	if err == nil || err.Error() != "MCPConnectionTimeout" || time.Since(start) > 5*time.Second {
		t.Fatalf("startup was not stopped promptly: %v", err)
	}
	if logs := platform.MCPClientLogs("timeout", true); len(logs) != 2 || !strings.Contains(logs[0].Message, "waiting for timeout") {
		t.Fatalf("timed-out attempt missing stderr: %+v", logs)
	}
	log := newClientLogBuffer(clientSecrets{})
	for range mcpLogCount + 5 {
		_, _ = log.Write([]byte("line\n"))
	}
	_, _ = log.Write([]byte(strings.Repeat("secret sample", mcpLogSize) + "\n"))
	snapshot := log.snapshot()
	if len(snapshot) != mcpLogCount || strings.Contains(snapshot[len(snapshot)-1].Message, "secret sample") {
		t.Fatalf("log capacity or truncation failed: %d", len(snapshot))
	}
}

func TestMCPAuthenticatedProxyAndToolTimeoutDoNotReplay(t *testing.T) {
	var calls atomic.Int32
	m := mcp.NewServer(&mcp.Implementation{Name: "proxy", Version: "1"}, nil)
	mcp.AddTool(m, &mcp.Tool{Name: "slow"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		calls.Add(1)
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-timer.C:
			return &mcp.CallToolResult{}, nil, nil
		}
	})
	mcpHTTP := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return m }, nil)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !r.URL.IsAbs() || r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpwYXNz" {
			w.WriteHeader(http.StatusProxyAuthRequired)
			return
		}
		mcpHTTP.ServeHTTP(w, r)
	}))
	defer proxy.Close()
	host, port, err := net.SplitHostPort(proxy.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	platform := NewPlatform(NewRegistry(), t.TempDir())
	platform.credentials = mockMCPClientCredentials{}
	defer platform.Shutdown()
	cfg := ClientConfig{Name: "代理", Endpoint: "http://127.0.0.2:12345/mcp", ProxyHost: host, ProxyPort: port,
		ProxyUsername: "user", ProxyPassword: "pass", ToolTimeout: 1}
	status, err := platform.ConnectMCP(t.Context(), cfg)
	if err != nil || !status.Config.ProxyPasswordSet || status.Config.ProxyPassword != "" {
		t.Fatalf("authenticated proxy failed or leaked the password: %+v %v", status, err)
	}
	tools := platform.ChatTools("chat")
	if len(tools) != 1 {
		t.Fatalf("proxy tool catalog: %d", len(tools))
	}
	result, err := tools[0].Read(agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalBypass), ai.ToolCall{ArgumentsJSON: "{}"})
	if err != nil || !strings.Contains(result, "MCPResultUnknown") || calls.Load() != 1 {
		t.Fatalf("timeout result or call count: %q %v (%d)", result, err, calls.Load())
	}
}
