package appagent

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/coolerks/Fealdhord/internal/agentrun"
)

const loadMCPName = "load_mcp"
const loadMCPDescription = "按连接 ID 加载已配置的 MCP 服务，使工具在后续轮次可用。"

type mcpCatalogEntry struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Tools []string `json:"tools,omitempty"`
}

func mcpToolHints(names []string) []string {
	hints := make([]string, 0, min(12, len(names)))
	for _, name := range names {
		if len(name) > 80 || strings.ContainsAny(name, "\r\n\t") {
			continue
		}
		hints = append(hints, name)
		if len(hints) == cap(hints) {
			break
		}
	}
	return hints
}

func (a *Platform) mcpCatalog() []mcpCatalogEntry {
	configs, err := a.savedClients()
	if err != nil {
		return nil
	}
	items := make([]mcpCatalogEntry, 0, len(configs))
	for id, config := range configs {
		if !config.Paused {
			items = append(items, mcpCatalogEntry{ID: id, Name: config.Name, Tools: mcpToolHints(config.ToolHints)})
		}
	}
	slices.SortFunc(items, func(a, b mcpCatalogEntry) int { return strings.Compare(a.ID, b.ID) })
	return items
}

func (a *Platform) loadSavedMCP(ctx context.Context, id string) (mcpCatalogEntry, error) {
	a.clientEdit.Lock()
	defer a.clientEdit.Unlock()
	if err := ctx.Err(); err != nil {
		return mcpCatalogEntry{}, err
	}
	configs, err := a.savedClients()
	if err != nil {
		return mcpCatalogEntry{}, err
	}
	config, exists := configs[id]
	if !exists || config.Paused {
		return mcpCatalogEntry{}, errors.New("MCPClientUnavailable")
	}
	a.mu.Lock()
	conn := a.clients[id]
	a.mu.Unlock()
	if conn == nil {
		connection, log, _, err := a.openConfiguredClient(ctx, config, config)
		if err != nil {
			// The transport can contain endpoints or command output.
			return mcpCatalogEntry{}, errors.New("MCPConnectionFailed")
		}
		if err := ctx.Err(); err != nil {
			_ = connection.session.Close()
			if log != nil {
				log.finish()
			}
			return mcpCatalogEntry{}, err
		}
		a.installClient(config, connection, log)
		conn = connection
	}
	return mcpCatalogEntry{ID: id, Name: config.Name, Tools: mcpToolHints(conn.status.Tools)}, nil
}

func (a *Platform) mcpLoader(conversationID string, catalog []mcpCatalogEntry, onLoaded func()) agentrun.ReadTool {
	tool := a.registry.chatTool(conversationID, loadMCPName, Tool{
		Name: loadMCPName, Description: loadMCPDescription,
		schema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"MCP 连接 ID"}},"required":["id"],"additionalProperties":false}`),
		invoke: func(ctx context.Context, args json.RawMessage) (any, error) {
			var input struct {
				ID string `json:"id"`
			}
			if json.Unmarshal(args, &input) != nil || !clientIDPattern.MatchString(input.ID) {
				return nil, errors.New("ValidationError")
			}
			if agentrun.ApprovalMode(ctx) == agentrun.ApprovalDeny ||
				!slices.ContainsFunc(catalog, func(item mcpCatalogEntry) bool { return item.ID == input.ID }) {
				return nil, errors.New("PermissionDenied")
			}
			entry, err := a.loadSavedMCP(ctx, input.ID)
			if err != nil {
				return nil, err
			}
			onLoaded()
			return map[string]any{"id": entry.ID, "name": entry.Name, "tools": entry.Tools, "loaded": true}, nil
		},
	})
	// Connection startup is held until plan approval. Saved connections can be
	// opened automatically in an execution run, subject to its tool rules.
	tool.RequiresApproval = true
	return tool
}
