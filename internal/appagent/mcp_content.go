package appagent

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"slices"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (c *clientConnection) contentTools(connection string) []Tool {
	r := NewRegistry()
	if len(c.status.Resources) > 0 {
		type resourceArgs struct {
			URI string `json:"uri" jsonschema:"required,description=已发现的资源 URI"`
		}
		Register(r, clientToolName(c.status.ID, "read_resource", "resource"), "读取 MCP "+connection+" 提供的文本资源", true,
			func(ctx context.Context, input resourceArgs) any {
				if !slices.Contains(c.status.Resources, input.URI) {
					return map[string]string{"error": "ResourceNotFound"}
				}
				ctx, cancel := toolDeadline(ctx, c.target.ToolTimeout)
				defer cancel()
				output, err := c.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: input.URI})
				if err != nil || output.NeedsInput() {
					return map[string]string{"error": "MCPResourceUnavailable"}
				}
				var texts []string
				for _, content := range output.Contents {
					if content.Text != "" {
						texts = append(texts, content.Text)
					}
				}
				return map[string]any{"source": input.URI, "text": texts}
			})
		r.tools[len(r.tools)-1].permissionID = externalToolPermissionID(c.target, "resource", "read_resource")
		r.tools[len(r.tools)-1].externalName = "读取资源"
	}
	if len(c.status.Prompts) > 0 {
		type promptArgs struct {
			Name      string            `json:"name" jsonschema:"required"`
			Arguments map[string]string `json:"arguments,omitempty"`
		}
		Register(r, clientToolName(c.status.ID, "get_prompt", "prompt"), "获取 MCP "+connection+" 的提示词模板文本", true,
			func(ctx context.Context, input promptArgs) any {
				if !slices.Contains(c.status.Prompts, input.Name) || len(input.Arguments) > 16 {
					return map[string]string{"error": "PromptNotFound"}
				}
				ctx, cancel := toolDeadline(ctx, c.target.ToolTimeout)
				defer cancel()
				output, err := c.session.GetPrompt(ctx, &mcp.GetPromptParams{Name: input.Name, Arguments: input.Arguments})
				if err != nil || output.NeedsInput() {
					return map[string]string{"error": "MCPPromptUnavailable"}
				}
				var texts []string
				for _, message := range output.Messages {
					if content, ok := message.Content.(*mcp.TextContent); ok {
						texts = append(texts, content.Text)
					}
				}
				return map[string]any{"source": input.Name, "text": texts}
			})
		r.tools[len(r.tools)-1].permissionID = externalToolPermissionID(c.target, "prompt", "get_prompt")
		r.tools[len(r.tools)-1].externalName = "获取提示词"
	}
	return r.tools
}

func (a *Platform) addResourceTemplate(server *mcp.Server, tool Tool, kind, principal string) {
	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name: kind, URITemplate: "fealdhord://" + kind + "/{id}", MIMEType: "application/json",
		Description: tool.Description,
	}, func(ctx context.Context, request *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri, err := url.Parse(request.Params.URI)
		if err != nil {
			return nil, mcp.ResourceNotFoundError(request.Params.URI)
		}
		itemID := strings.TrimPrefix(uri.Path, "/")
		if uri.Scheme != "fealdhord" || uri.Host != kind || uri.User != nil || uri.RawQuery != "" || uri.Fragment != "" ||
			len(itemID) == 0 || len(itemID) > 128 || strings.Contains(itemID, "/") {
			return nil, mcp.ResourceNotFoundError(request.Params.URI)
		}
		input, _ := json.Marshal(id{ID: itemID})
		result, err := a.registry.execute(ctx, principal, tool, input)
		if err != nil {
			return nil, errors.New("ResourceUnavailable")
		}
		var envelope struct {
			Error json.RawMessage `json:"error"`
			OK    *bool           `json:"ok"`
		}
		if json.Unmarshal([]byte(result), &envelope) == nil && (len(envelope.Error) != 0 || envelope.OK != nil && !*envelope.OK) || result == `"NotFound"` {
			return nil, mcp.ResourceNotFoundError(request.Params.URI)
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{
			{URI: request.Params.URI, MIMEType: "application/json", Text: result},
		}}, nil
	})
}
