package agentrun

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/coolerks/Fealdhord/internal/ai"
)

// CompactToolCatalog retains access to authorized tools when sending all schemas
// would consume too much of the model input budget.
func CompactToolCatalog(request ai.ModelRequest, tools []ReadTool) []ReadTool {
	if request.Budget.InputLimit <= 0 || len(tools) < 3 {
		return tools
	}
	definitions := make([]ai.ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		definitions = append(definitions, tool.Definition)
	}
	cost := ai.EstimateRequest(ai.ModelRequest{Tools: definitions})
	if len(tools) <= 90 && cost <= request.Budget.InputLimit/3 {
		return tools
	}
	discovery := ReadTool{
		Definition: ai.ToolDefinition{
			Name: "find_app_tools", Description: "查询已授权的应用或 MCP 工具。按名称可读取参数结构；列表返回 next_offset 时可续查。使用 run_app_tool 调用。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"},"name":{"type":"string"},"offset":{"type":"integer","minimum":0}},"additionalProperties":false}`),
		},
		Read: func(_ context.Context, call ai.ToolCall) (string, error) {
			var input struct {
				Query, Name string
				Offset      int
			}
			if json.Unmarshal([]byte(call.ArgumentsJSON), &input) != nil || input.Offset < 0 {
				return `{"error":"ValidationError"}`, nil
			}
			if input.Name != "" {
				for _, tool := range tools {
					if tool.Definition.Name == input.Name {
						definition := tool.Definition
						data, err := json.Marshal(map[string]any{"name": definition.Name, "description": definition.Description, "inputSchema": definition.InputSchema})
						if err != nil || len(data) > maxToolReplyBytes {
							return `{"error":"ToolSchemaLimit"}`, nil
						}
						return string(data), nil
					}
				}
				return `{"error":"ToolNotAllowed"}`, nil
			}
			type entry struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			matches := make([]entry, 0, 8)
			total := 0
			query := strings.ToLower(strings.TrimSpace(input.Query))
			for _, tool := range tools {
				definition := tool.Definition
				if strings.Contains(strings.ToLower(definition.Name+" "+definition.Description), query) {
					if total >= input.Offset && len(matches) < 8 {
						description := []rune(definition.Description)
						matches = append(matches, entry{Name: definition.Name, Description: string(description[:min(len(description), 120)])})
					}
					total++
				}
			}
			result := map[string]any{"tools": matches, "totalAvailable": len(tools), "total_matches": total}
			if input.Offset < total-len(matches) {
				result["next_offset"] = input.Offset + len(matches)
			}
			data, _ := json.Marshal(result)
			return string(data), nil
		},
	}
	resolveInvocation := func(call ai.ToolCall) (ai.ToolCall, ReadTool, string) {
		var input struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if json.Unmarshal([]byte(call.ArgumentsJSON), &input) != nil || !strings.HasPrefix(strings.TrimSpace(string(input.Arguments)), "{") {
			return call, ReadTool{}, "ValidationError"
		}
		for _, tool := range tools {
			if tool.Definition.Name == input.Name {
				return ai.ToolCall{ID: call.ID, Name: input.Name, ArgumentsJSON: string(input.Arguments)}, tool, ""
			}
		}
		return call, ReadTool{}, "ToolNotAllowed"
	}
	invocation := ReadTool{
		Definition: ai.ToolDefinition{
			Name: "run_app_tool", Description: "调用通过 find_app_tools 发现并已授权的工具，保持该工具原有审批。",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"},"arguments":{"type":"object"}},"required":["name","arguments"],"additionalProperties":false}`),
		},
		Read: func(ctx context.Context, call ai.ToolCall) (string, error) {
			resolved, tool, code := resolveInvocation(call)
			if code != "" {
				return `{"error":"` + code + `"}`, nil
			}
			return tool.Read(ctx, resolved)
		},
		ArticleNext: func(ctx context.Context, call ai.ToolCall, cursor string) (string, error) {
			resolved, tool, code := resolveInvocation(call)
			if code != "" || tool.ArticleNext == nil {
				return "", ErrResultUnavailable
			}
			return tool.ArticleNext(ctx, resolved, cursor)
		},
		activityCall: func(call ai.ToolCall) ai.ToolCall {
			resolved, _, _ := resolveInvocation(call)
			return resolved
		},
	}
	return []ReadTool{discovery, invocation}
}
