package appagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/coolerks/Fealdhord/internal/agentrun"
	"github.com/coolerks/Fealdhord/internal/ai"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func customToolContext(t *testing.T, platform *Platform) context.Context {
	t.Helper()
	rules, err := platform.LoadToolPermissions()
	if err != nil {
		t.Fatal(err)
	}
	return agentrun.WithToolPermissions(agentrun.WithApprovalMode(t.Context(), agentrun.ApprovalCustom), rules)
}

func TestToolPermissionsKeepSendSnapshotAcrossCompactAndLegacyCalls(t *testing.T) {
	registry := NewRegistry()
	calls := 0
	Register(registry, "item.read", strings.Repeat("read ", 150), false, func(context.Context, struct{}) string {
		calls++
		return "read"
	})
	Register(registry, "item.write", "write", true, func(context.Context, struct{}) string {
		calls++
		return "write"
	})
	Register(registry, "item.list", "list", false, func(context.Context, struct{}) string { return "list" })
	dir := t.TempDir()
	platform := NewPlatform(registry, dir)
	if err := platform.SaveToolPermission("builtin:item.read", "deny"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveToolPermission("builtin:item.write", "bypass"); err != nil {
		t.Fatal(err)
	}
	platform = NewPlatform(registry, dir)
	snapshot := customToolContext(t, platform)
	tools := platform.ChatTools("chat")
	compact := agentrun.CompactToolCatalog(ai.ModelRequest{Budget: ai.Budget{InputLimit: 256}}, tools)
	if len(compact) != 2 || compact[1].Definition.Name != "run_app_tool" {
		t.Fatal("expected the compact invocation path")
	}
	if err := platform.SaveToolPermission("builtin:item.read", ""); err != nil {
		t.Fatal(err)
	}
	call := ai.ToolCall{ArgumentsJSON: "{}"}
	got, err := tools[0].Read(snapshot, call)
	if err != nil || got != `{"error":"PermissionDenied"}` {
		t.Fatalf("send snapshot lost denial: %s %v", got, err)
	}
	got, err = compact[1].Read(snapshot, ai.ToolCall{ArgumentsJSON: `{"name":"item_read","arguments":{}}`})
	if err != nil || got != `{"error":"PermissionDenied"}` || calls != 0 {
		t.Fatalf("compact lookup bypassed policy: %s %v, calls=%d", got, err, calls)
	}
	for _, test := range []struct {
		ctx   context.Context
		tool  agentrun.ReadTool
		value string
	}{
		{customToolContext(t, platform), tools[0], `"read"`},
		{snapshot, tools[1], `"write"`},
		{agentrun.WithApprovalMode(snapshot, "bypass"), tools[0], `"read"`},
		{agentrun.WithApprovalMode(snapshot, "deny"), tools[1], `{"error":"PermissionDenied"}`},
	} {
		if got, err := test.tool.Read(test.ctx, call); err != nil || got != test.value {
			t.Fatalf("mode %q tool %s: %s %v", agentrun.ApprovalMode(test.ctx), test.tool.Definition.Name, got, err)
		}
	}
	if got, err := registry.execute(snapshot, "mcp", registry.Tools()[0], []byte("{}")); err != nil || got != `"read"` || calls != 4 {
		t.Fatalf("MCP server inherited the Chat policy: %s %v, calls=%d", got, err, calls)
	}
	if err := os.WriteFile(filepath.Join(dir, toolPermissionsFile), []byte(`{"broken":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := platform.LoadToolPermissions(); err != errToolPermissionsUnavailable {
		t.Fatalf("corrupt rules treated as empty: %v", err)
	}
}

func TestExternalToolPermissionsAreScopedToConnectionAndTarget(t *testing.T) {
	var firstCalls, secondCalls atomic.Int32
	server := func(calls *atomic.Int32) *httptest.Server {
		m := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
		mcp.AddTool(m, &mcp.Tool{Name: "lookup"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
			calls.Add(1)
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "found"}}}, nil, nil
		})
		return httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return m }, nil))
	}
	first, second := server(&firstCalls), server(&secondCalls)
	defer first.Close()
	defer second.Close()
	platform := NewPlatform(NewRegistry(), t.TempDir())
	defer platform.Shutdown()
	a := ClientConfig{Name: "alpha", Endpoint: first.URL}
	b := ClientConfig{Name: "beta", Endpoint: second.URL}
	for _, config := range []ClientConfig{a, b} {
		if _, err := platform.ConnectMCP(t.Context(), config); err != nil {
			t.Fatal(err)
		}
	}
	catalog, err := platform.ToolPermissions()
	if err != nil || len(catalog) != 3 || catalog[1].Connection != a.Name || catalog[2].Connection != b.Name ||
		catalog[1].Name != "lookup" || catalog[2].Name != "lookup" || catalog[1].ID == catalog[2].ID {
		t.Fatalf("same-name tool identities: %+v %v", catalog, err)
	}
	alphaID, betaID := catalog[1].ID, catalog[2].ID
	clients, err := platform.MCPClients()
	if err != nil || len(clients) != 2 {
		t.Fatalf("clients: %+v %v", clients, err)
	}
	betaWire := clientToolName(clients[1].ID, "lookup", "tool")
	if err := platform.SaveToolPermission(alphaID, "bypass"); err != nil {
		t.Fatal(err)
	}
	if err := platform.SaveToolPermission(betaID, "deny"); err != nil {
		t.Fatal(err)
	}
	platform.DisconnectMCP(a.Name)
	catalog, err = platform.ToolPermissions()
	if err != nil || catalog[1].Available || catalog[1].ID != alphaID {
		t.Fatalf("disconnected rule disappeared: %+v %v", catalog, err)
	}
	if _, err := platform.ConnectMCP(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	ctx := customToolContext(t, platform)
	for _, tool := range platform.ChatTools("chat") {
		result, err := tool.Read(ctx, ai.ToolCall{ArgumentsJSON: "{}"})
		if err != nil || strings.Contains(result, "PermissionDenied") != (tool.Definition.Name == betaWire) {
			t.Fatalf("wrong decision for %s: %s %v", tool.Definition.Name, result, err)
		}
	}
	if firstCalls.Load() != 1 || secondCalls.Load() != 0 {
		t.Fatalf("unexpected calls %d %d", firstCalls.Load(), secondCalls.Load())
	}
	a.Endpoint = second.URL
	if _, err := platform.ConnectMCP(t.Context(), a); err != nil {
		t.Fatal(err)
	}
	catalog, err = platform.ToolPermissions()
	if err != nil || len(catalog) != 4 || !catalog[1].Available || catalog[1].ID == alphaID ||
		catalog[1].Mode != "" || catalog[2].Available || catalog[2].ID != alphaID {
		t.Fatalf("new target inherited old rule: %+v %v", catalog, err)
	}
	if err := platform.SaveToolPermission(alphaID, ""); err != nil {
		t.Fatal(err)
	}
}

func TestSaveToolPermissionsAtomicBatchAndDescriptionRuntimeOnly(t *testing.T) {
	registry := NewRegistry()
	Register(registry, "item.read", "读取指定条目", false, func(context.Context, struct{}) string { return "ok" })
	Register(registry, "item.write", "写入指定条目", true, func(context.Context, struct{}) string { return "ok" })
	dir := t.TempDir()
	platform := NewPlatform(registry, dir)

	// 批量保存成功
	if err := platform.SaveToolPermissions([]ToolPermissionChange{
		{ID: "builtin:item.read", Mode: "ask"},
		{ID: "builtin:item.write", Mode: "bypass"},
	}); err != nil {
		t.Fatal(err)
	}

	catalog, err := platform.ToolPermissions()
	if err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]ToolPermission, len(catalog))
	for _, item := range catalog {
		byID[item.ID] = item
	}
	if byID["builtin:item.read"].Mode != "ask" || byID["builtin:item.read"].Description != "读取指定条目" {
		t.Fatalf("unexpected item.read permission: %+v", byID["builtin:item.read"])
	}
	if byID["builtin:item.write"].Mode != "bypass" || byID["builtin:item.write"].Description != "写入指定条目" {
		t.Fatalf("unexpected item.write permission: %+v", byID["builtin:item.write"])
	}

	// 落盘文件不包含 description
	raw, err := os.ReadFile(filepath.Join(dir, toolPermissionsFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "description") || strings.Contains(string(raw), "读取指定条目") {
		t.Fatalf("persisted rules should omit description: %s", string(raw))
	}

	// 批量中包含非法工具时整体回滚，不影响已有规则
	if err := platform.SaveToolPermissions([]ToolPermissionChange{
		{ID: "builtin:item.read", Mode: "deny"},
		{ID: "builtin:missing", Mode: "bypass"},
	}); err == nil || err.Error() != "UnknownTool" {
		t.Fatalf("expected UnknownTool, got %v", err)
	}
	rules, err := platform.LoadToolPermissions()
	if err != nil || rules["builtin:item.read"] != "ask" || rules["builtin:item.write"] != "bypass" {
		t.Fatalf("failed batch mutated saved rules: %+v %v", rules, err)
	}

	// 批量恢复默认
	if err := platform.SaveToolPermissions([]ToolPermissionChange{
		{ID: "builtin:item.read", Mode: ""},
		{ID: "builtin:item.write", Mode: ""},
	}); err != nil {
		t.Fatal(err)
	}
	rules, err = platform.LoadToolPermissions()
	if err != nil || len(rules) != 0 {
		t.Fatalf("expected empty rules after reset, got %+v %v", rules, err)
	}
}
