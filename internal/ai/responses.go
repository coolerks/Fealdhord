package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type Responses struct{ *modelClient }

var _ Model = (*Responses)(nil)

func NewResponses(baseURL, apiKey string, client *http.Client) (*Responses, error) {
	transport, err := newModelClient(baseURL, apiKey, "responses", client)
	if err != nil {
		return nil, err
	}
	return &Responses{transport}, nil
}

type responseItem struct {
	Type      string  `json:"type,omitempty"`
	ID        string  `json:"id,omitempty"`
	Role      string  `json:"role,omitempty"`
	Content   any     `json:"content,omitempty"`
	CallID    string  `json:"call_id,omitempty"`
	Name      string  `json:"name,omitempty"`
	Arguments *string `json:"arguments,omitempty"`
	Output    *string `json:"output,omitempty"`
}

type responsePart struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Refusal string `json:"refusal"`
}

type responseOutput struct {
	responseItem
	Content []responsePart `json:"content"`
}

type responsesResult struct {
	Status     string           `json:"status"`
	Output     []responseOutput `json:"output"`
	Incomplete struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage *tokenUsage `json:"usage"`
}

func checkedHistory(req *ModelRequest) (chatRequest, error) {
	var history chatRequest
	if req == nil || strings.TrimSpace(req.Model) == "" || len(req.Messages) == 0 {
		return history, &ModelError{Code: "ValidationError"}
	}
	err := encodeChatHistory(&history, req)
	return history, err
}

func (r *Responses) request(ctx context.Context, req *ModelRequest, stream bool) (*http.Response, error) {
	history, err := checkedHistory(req)
	if err != nil {
		return nil, err
	}
	if len(req.Stop) > 0 {
		return nil, &ModelError{Code: "CapabilityError"}
	}
	input := make([]responseItem, 0, len(history.Messages))
	messages := history.Messages
	if req.System != "" {
		messages = messages[1:]
	}
	for _, msg := range messages {
		if msg.Role == "tool" {
			input = append(input, responseItem{Type: "function_call_output", CallID: msg.ToolCallID, Output: &msg.Content})
			continue
		}
		if msg.Content != "" || len(msg.ToolCalls) == 0 {
			input = append(input, responseItem{Role: msg.Role, Content: msg.Content})
		}
		for _, call := range msg.ToolCalls {
			input = append(input, responseItem{Type: "function_call", CallID: call.ID, Name: call.Function.Name, Arguments: &call.Function.Arguments})
		}
	}
	payload := map[string]any{"model": req.Model, "input": input, "store": false, "stream": stream}
	if req.System != "" {
		payload["instructions"] = req.System
	}
	if req.Temperature != nil {
		payload["temperature"] = req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = req.TopP
	}
	if req.MaxOutputTokens != nil {
		payload["max_output_tokens"] = req.MaxOutputTokens
	}
	if len(history.Tools) > 0 {
		tools := make([]map[string]any, 0, len(history.Tools))
		for _, tool := range history.Tools {
			tools = append(tools, map[string]any{"type": "function", "name": tool.Function.Name, "description": tool.Function.Description, "parameters": tool.Function.Parameters, "strict": false})
		}
		payload["tools"] = tools
	}
	if req.ToolChoice.Mode == "function" {
		payload["tool_choice"] = map[string]string{"type": "function", "name": req.ToolChoice.Name}
	} else if req.ToolChoice.Mode != "" {
		payload["tool_choice"] = req.ToolChoice.Mode
	}
	return r.post(ctx, payload, false)
}

func (r *Responses) Generate(ctx context.Context, req *ModelRequest) (*ModelResponse, error) {
	res, err := r.request(ctx, req, false)
	if err != nil {
		return nil, err
	}
	var result responsesResult
	if err := decodeModelResponse(res, &result); err != nil {
		return nil, err
	}
	return result.normalize(offeredChatTools(req))
}

func (r responsesResult) normalize(offered map[string]bool) (*ModelResponse, error) {
	result := &ModelResponse{Message: Message{Role: "assistant"}, Usage: r.Usage.normalized()}
	switch r.Status {
	case "completed":
		result.FinishReason = "stop"
	case "incomplete":
		switch r.Incomplete.Reason {
		case "max_output_tokens":
			result.FinishReason = "length"
		case "content_filter":
			result.FinishReason = "content_filter"
		default:
			return nil, protocolError()
		}
	default:
		return nil, protocolError()
	}
	var calls []chatToolCall
	for _, item := range r.Output {
		switch item.Type {
		case "message":
			if item.Role != "assistant" {
				return nil, protocolError()
			}
			for _, part := range item.Content {
				switch part.Type {
				case "output_text":
					result.Message.Content = append(result.Message.Content, ContentBlock{Type: "text", Text: part.Text})
				case "refusal":
					result.Message.Content = append(result.Message.Content, ContentBlock{Type: "text", Text: part.Refusal})
					if r.Status == "completed" {
						result.FinishReason = "refusal"
					}
				}
			}
		case "function_call":
			if item.Arguments == nil || result.FinishReason == "refusal" {
				return nil, protocolError()
			}
			if r.Status == "completed" {
				calls = append(calls, wireToolCall(item.CallID, item.Name, *item.Arguments))
			}
		}
	}
	var err error
	result.Message.ToolCalls, err = completeChatCalls(calls, offered)
	if len(calls) > 0 {
		if result.FinishReason != "stop" {
			return nil, protocolError()
		}
		result.FinishReason = "tool_calls"
	}
	return result, err
}

func (r *Responses) Stream(ctx context.Context, req *ModelRequest) (<-chan ModelEvent, error) {
	offered := offeredChatTools(req)
	return modelStream(ctx, func(ctx context.Context) (*http.Response, error) {
		return r.request(ctx, req, true)
	}, func(body io.Reader, send func(ModelEvent) bool) error {
		started := false
		pending := make(map[string]*pendingChatCall)
		callIDs := make(map[string]bool)
		return readModelSSE(body, func(data string) (bool, error) {
			var event struct {
				Type     string          `json:"type"`
				Delta    json.RawMessage `json:"delta"`
				ItemID   string          `json:"item_id"`
				Item     responseItem    `json:"item"`
				Response responsesResult `json:"response"`
			}
			if json.Unmarshal([]byte(data), &event) != nil || event.Type == "" {
				return false, protocolError()
			}
			switch event.Type {
			case "response.created":
				if started {
					return false, protocolError()
				}
				started = true
				return false, emit(send, ModelEvent{Type: "message_start"})
			case "response.output_text.delta", "response.refusal.delta":
				if !started {
					return false, protocolError()
				}
				var delta string
				if json.Unmarshal(event.Delta, &delta) != nil {
					return false, protocolError()
				}
				return false, emit(send, ModelEvent{Type: "text_delta", Text: delta})
			case "response.output_item.added":
				item := event.Item
				if item.Type != "function_call" {
					break
				}
				if !started || item.ID == "" || item.CallID == "" || pending[item.ID] != nil || callIDs[item.CallID] || !offered[item.Name] || len(pending) == maxChatTools {
					return false, protocolError()
				}
				call := &pendingChatCall{wire: wireToolCall(item.CallID, item.Name, "")}
				pending[item.ID] = call
				callIDs[item.CallID] = true
				if err := emit(send, ModelEvent{Type: "tool_call_start", CallID: item.CallID, ToolName: item.Name}); err != nil {
					return false, err
				}
				if item.Arguments != nil {
					return false, emitArguments(send, call, *item.Arguments)
				}
			case "response.function_call_arguments.delta":
				call := pending[event.ItemID]
				if call == nil {
					return false, protocolError()
				}
				var delta string
				if json.Unmarshal(event.Delta, &delta) != nil {
					return false, protocolError()
				}
				return false, emitArguments(send, call, delta)
			case "response.completed", "response.incomplete":
				if !started || event.Type != "response."+event.Response.Status {
					return false, protocolError()
				}
				result, err := event.Response.normalize(offered)
				if err != nil {
					return false, err
				}
				if event.Response.Status == "completed" {
					if len(result.Message.ToolCalls) != len(pending) {
						return false, protocolError()
					}
					for _, item := range event.Response.Output {
						if item.Type != "function_call" {
							continue
						}
						call := pending[item.ID]
						if call == nil || call.wire.ID != item.CallID || call.wire.Function.Name != item.Name || call.arguments.String() != *item.Arguments {
							return false, protocolError()
						}
					}
				}
				return true, emitResult(send, result)
			case "error", "response.failed":
				return false, protocolError()
			}
			return false, nil
		})
	})
}
