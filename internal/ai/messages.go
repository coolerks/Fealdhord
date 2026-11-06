package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type Messages struct{ *modelClient }

var _ Model = (*Messages)(nil)

func NewMessages(baseURL, apiKey string, client *http.Client) (*Messages, error) {
	transport, err := newModelClient(baseURL, apiKey, "messages", client)
	if err != nil {
		return nil, err
	}
	return &Messages{transport}, nil
}

type messageBlock struct {
	Type      string          `json:"type"`
	Text      *string         `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   *string         `json:"content,omitempty"`
}

type messagesTurn struct {
	Role    string         `json:"role"`
	Content []messageBlock `json:"content"`
}

type messagesResult struct {
	messagesTurn
	Type       string      `json:"type"`
	StopReason string      `json:"stop_reason"`
	Usage      *tokenUsage `json:"usage"`
}

func (m *Messages) request(ctx context.Context, req *ModelRequest, stream bool) (*http.Response, error) {
	history, err := checkedHistory(req)
	if err != nil {
		return nil, err
	}
	limit := 4096
	if req.MaxOutputTokens != nil {
		limit = *req.MaxOutputTokens
	}
	if limit < 1 {
		return nil, &ModelError{Code: "ValidationError"}
	}
	var systems []string
	var turns []messagesTurn
	for _, msg := range history.Messages {
		if msg.Role == "system" {
			if len(turns) != 0 {
				return nil, &ModelError{Code: "CapabilityError"}
			}
			systems = append(systems, msg.Content)
			continue
		}
		role := msg.Role
		var blocks []messageBlock
		if role == "tool" {
			role = "user"
			blocks = append(blocks, messageBlock{Type: "tool_result", ToolUseID: msg.ToolCallID, Content: &msg.Content})
		} else {
			if msg.Content != "" || len(msg.ToolCalls) == 0 {
				blocks = append(blocks, messageBlock{Type: "text", Text: &msg.Content})
			}
			for _, call := range msg.ToolCalls {
				blocks = append(blocks, messageBlock{Type: "tool_use", ID: call.ID, Name: call.Function.Name, Input: json.RawMessage(call.Function.Arguments)})
			}
		}
		if last := len(turns) - 1; last >= 0 && turns[last].Role == role {
			turns[last].Content = append(turns[last].Content, blocks...)
		} else {
			turns = append(turns, messagesTurn{Role: role, Content: blocks})
		}
	}
	if len(turns) == 0 || turns[0].Role != "user" {
		return nil, &ModelError{Code: "ValidationError"}
	}
	payload := map[string]any{"model": req.Model, "messages": turns, "max_tokens": limit, "stream": stream}
	if len(systems) > 0 {
		payload["system"] = strings.Join(systems, "\n\n")
	}
	if len(req.Stop) > 0 {
		payload["stop_sequences"] = req.Stop
	}
	if req.Temperature != nil {
		payload["temperature"] = req.Temperature
	}
	if req.TopP != nil {
		payload["top_p"] = req.TopP
	}
	if len(history.Tools) > 0 {
		tools := make([]map[string]any, 0, len(history.Tools))
		for _, tool := range history.Tools {
			tools = append(tools, map[string]any{"name": tool.Function.Name, "description": tool.Function.Description, "input_schema": tool.Function.Parameters})
		}
		payload["tools"] = tools
	}
	switch req.ToolChoice.Mode {
	case "required":
		payload["tool_choice"] = map[string]string{"type": "any"}
	case "function":
		payload["tool_choice"] = map[string]string{"type": "tool", "name": req.ToolChoice.Name}
	case "auto", "none":
		if len(history.Tools) > 0 {
			payload["tool_choice"] = map[string]string{"type": req.ToolChoice.Mode}
		}
	}
	return m.post(ctx, payload, true)
}

func (m *Messages) Generate(ctx context.Context, req *ModelRequest) (*ModelResponse, error) {
	res, err := m.request(ctx, req, false)
	if err != nil {
		return nil, err
	}
	var result messagesResult
	if err := decodeModelResponse(res, &result); err != nil {
		return nil, err
	}
	if result.Type != "message" || result.Role != "assistant" {
		return nil, protocolError()
	}
	var calls []chatToolCall
	var text []ContentBlock
	for _, block := range result.Content {
		switch block.Type {
		case "text":
			if block.Text == nil {
				return nil, protocolError()
			}
			text = append(text, ContentBlock{Type: "text", Text: *block.Text})
		case "tool_use":
			calls = append(calls, wireToolCall(block.ID, block.Name, string(block.Input)))
		}
	}
	normalized, err := normalizeMessages(result.StopReason, result.Usage, calls, offeredChatTools(req))
	if err != nil {
		return nil, err
	}
	normalized.Message.Content = text
	return normalized, nil
}

func normalizeMessages(reason string, usage *tokenUsage, wire []chatToolCall, offered map[string]bool) (*ModelResponse, error) {
	var finish string
	switch reason {
	case "end_turn", "stop_sequence":
		finish = "stop"
	case "max_tokens", "model_context_window_exceeded":
		finish = "length"
	case "tool_use":
		finish = "tool_calls"
	case "refusal":
		finish = "refusal"
	default:
		return nil, protocolError()
	}
	if finish == "length" || finish == "refusal" {
		wire = nil
	}
	if (finish == "tool_calls") != (len(wire) > 0) {
		return nil, protocolError()
	}
	calls, err := completeChatCalls(wire, offered)
	if err != nil {
		return nil, err
	}
	return &ModelResponse{Message: Message{Role: "assistant", ToolCalls: calls}, FinishReason: finish, Usage: usage.normalized()}, nil
}

func (m *Messages) Stream(ctx context.Context, req *ModelRequest) (<-chan ModelEvent, error) {
	offered := offeredChatTools(req)
	return modelStream(ctx, func(ctx context.Context) (*http.Response, error) {
		return m.request(ctx, req, true)
	}, func(body io.Reader, send func(ModelEvent) bool) error {
		state := messagesStream{blocks: make(map[int]*messagesStreamBlock), offered: offered}
		return readModelSSE(body, func(data string) (bool, error) { return state.read(data, send) })
	})
}

type messagesStreamBlock struct {
	kind    string
	closed  bool
	initial json.RawMessage
	call    *pendingChatCall
}

type messagesStream struct {
	blocks  map[int]*messagesStreamBlock
	offered map[string]bool
	usage   *tokenUsage
	reason  string
	started bool
}

func (s *messagesStream) read(data string, send func(ModelEvent) bool) (bool, error) {
	var event struct {
		Type    string         `json:"type"`
		Index   *int           `json:"index"`
		Message messagesResult `json:"message"`
		Block   messageBlock   `json:"content_block"`
		Delta   struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			Partial    string `json:"partial_json"`
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		Usage *tokenUsage `json:"usage"`
	}
	if json.Unmarshal([]byte(data), &event) != nil || event.Type == "" {
		return false, protocolError()
	}
	if event.Type == "message_start" {
		if s.started || event.Message.Role != "assistant" {
			return false, protocolError()
		}
		s.started, s.usage = true, event.Message.Usage
		return false, emit(send, ModelEvent{Type: "message_start"})
	}
	switch event.Type {
	case "error":
		return false, protocolError()
	case "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop":
		if !s.started {
			return false, protocolError()
		}
	default:
		return false, nil
	}
	if event.Type == "message_delta" {
		if event.Delta.StopReason != "" {
			if s.reason != "" && s.reason != event.Delta.StopReason {
				return false, protocolError()
			}
			s.reason = event.Delta.StopReason
		}
		if event.Usage != nil {
			if s.usage == nil {
				s.usage = &tokenUsage{}
			}
			if event.Usage.Input != nil {
				s.usage.Input = event.Usage.Input
			}
			if event.Usage.Output != nil {
				s.usage.Output = event.Usage.Output
			}
		}
		return false, nil
	}
	if event.Type == "message_stop" {
		var calls []chatToolCall
		for index := range len(s.blocks) {
			block := s.blocks[index]
			if block == nil || !block.closed {
				return false, protocolError()
			}
			if block.call != nil {
				call := block.call.wire
				call.Function.Arguments = block.call.arguments.String()
				if call.Function.Arguments == "" {
					call.Function.Arguments = string(block.initial)
				}
				calls = append(calls, call)
			}
		}
		result, err := normalizeMessages(s.reason, s.usage, calls, s.offered)
		if err != nil {
			return false, err
		}
		return true, emitResult(send, result)
	}
	if event.Index == nil || *event.Index < 0 || *event.Index >= 4096 || s.reason != "" {
		return false, protocolError()
	}
	index := *event.Index
	block := s.blocks[index]
	if event.Type == "content_block_start" {
		if block != nil {
			return false, protocolError()
		}
		block = &messagesStreamBlock{kind: event.Block.Type}
		s.blocks[index] = block
		switch block.kind {
		case "text":
			if event.Block.Text != nil && *event.Block.Text != "" {
				return false, emit(send, ModelEvent{Type: "text_delta", Text: *event.Block.Text})
			}
		case "tool_use":
			if event.Block.ID == "" || !s.offered[event.Block.Name] {
				return false, protocolError()
			}
			for _, prior := range s.blocks {
				if prior.call != nil && prior.call.wire.ID == event.Block.ID {
					return false, protocolError()
				}
			}
			block.call = &pendingChatCall{wire: wireToolCall(event.Block.ID, event.Block.Name, "")}
			block.initial = event.Block.Input
			return false, emit(send, ModelEvent{Type: "tool_call_start", CallID: event.Block.ID, ToolName: event.Block.Name})
		}
		return false, nil
	}
	if block == nil || block.closed {
		return false, protocolError()
	}
	if event.Type == "content_block_stop" {
		block.closed = true
		return false, nil
	}
	switch event.Delta.Type {
	case "text_delta":
		if block.kind != "text" {
			return false, protocolError()
		}
		return false, emit(send, ModelEvent{Type: "text_delta", Text: event.Delta.Text})
	case "input_json_delta":
		if block.call == nil {
			return false, protocolError()
		}
		return false, emitArguments(send, block.call, event.Delta.Partial)
	}
	return false, nil
}
