package ai

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
)

const (
	maxChatTools     = 128
	maxToolArguments = 64 << 10
)

var toolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type chatToolDefinition struct {
	Type     string                 `json:"type"`
	Function chatFunctionDefinition `json:"function"`
}

type chatFunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type chatToolCall struct {
	Index    *int         `json:"index,omitempty"`
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

func jsonObject(value string) bool {
	trimmed := strings.TrimSpace(value)
	return strings.HasPrefix(trimmed, "{") && json.Valid([]byte(trimmed))
}

func offeredChatTools(req *ModelRequest) map[string]bool {
	names := make(map[string]bool)
	if req == nil {
		return nil
	}
	for _, tool := range req.Tools {
		if req.ToolChoice.Mode != "none" && (req.ToolChoice.Mode != "function" || req.ToolChoice.Name == tool.Name) {
			names[tool.Name] = true
		}
	}
	return names
}

func encodeChatHistory(payload *chatRequest, req *ModelRequest) error {
	invalid := &ModelError{Code: "ValidationError"}
	if len(req.Tools) > maxChatTools {
		return invalid
	}
	names := make(map[string]bool, len(req.Tools))
	for _, tool := range req.Tools {
		if !toolNamePattern.MatchString(tool.Name) || names[tool.Name] || !jsonObject(string(tool.InputSchema)) {
			return invalid
		}
		names[tool.Name] = true
		payload.Tools = append(payload.Tools, chatToolDefinition{
			Type:     "function",
			Function: chatFunctionDefinition{Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema},
		})
	}

	switch choice := req.ToolChoice; choice.Mode {
	case "":
		if choice.Name != "" {
			return invalid
		}
	case "auto", "none", "required":
		if choice.Name != "" || (choice.Mode == "required" && len(names) == 0) {
			return invalid
		}
		payload.ToolChoice = choice.Mode
	case "function":
		if !names[choice.Name] {
			return invalid
		}
		payload.ToolChoice = chatToolDefinition{Type: "function", Function: chatFunctionDefinition{Name: choice.Name}}
	default:
		return invalid
	}

	if req.System != "" {
		payload.Messages = append(payload.Messages, chatMessage{Role: "system", Content: req.System})
	}
	pending := make(map[string]bool)
	for _, message := range req.Messages {
		if (len(pending) > 0) != (message.Role == "tool") {
			return invalid
		}
		wire := chatMessage{Role: message.Role, ToolCallID: message.ToolCallID}
		var content strings.Builder
		for _, block := range message.Content {
			if block.Type != "text" {
				return &ModelError{Code: "CapabilityError"}
			}
			content.WriteString(block.Text)
		}
		wire.Content = content.String()
		if message.Role != "assistant" && len(message.ToolCalls) > 0 ||
			message.Role != "tool" && message.ToolCallID != "" {
			return invalid
		}
		switch message.Role {
		case "system", "user":
		case "assistant":
			if len(message.ToolCalls) > maxChatTools {
				return invalid
			}
			for _, call := range message.ToolCalls {
				if call.ID == "" || pending[call.ID] || !toolNamePattern.MatchString(call.Name) ||
					len(call.ArgumentsJSON) > maxToolArguments || !jsonObject(call.ArgumentsJSON) {
					return invalid
				}
				pending[call.ID] = true
				wire.ToolCalls = append(wire.ToolCalls, chatToolCall{
					ID: call.ID, Type: "function",
					Function: chatFunction{Name: call.Name, Arguments: call.ArgumentsJSON},
				})
			}
		case "tool":
			if !pending[message.ToolCallID] {
				return invalid
			}
			delete(pending, message.ToolCallID)
		default:
			return &ModelError{Code: "CapabilityError"}
		}
		payload.Messages = append(payload.Messages, wire)
	}
	if len(pending) > 0 {
		return invalid
	}
	return nil
}

func completeChatCalls(wire []chatToolCall, offered map[string]bool) ([]ToolCall, error) {
	if len(wire) > maxChatTools {
		return nil, protocolError()
	}
	var calls []ToolCall
	seen := make(map[string]bool, len(wire))
	for _, call := range wire {
		if call.Type != "function" || call.ID == "" || seen[call.ID] || !offered[call.Function.Name] ||
			len(call.Function.Arguments) > maxToolArguments || !jsonObject(call.Function.Arguments) {
			return nil, protocolError()
		}
		seen[call.ID] = true
		calls = append(calls, ToolCall{ID: call.ID, Name: call.Function.Name, ArgumentsJSON: call.Function.Arguments})
	}
	return calls, nil
}

type pendingChatCall struct {
	wire      chatToolCall
	arguments strings.Builder
}

func appendChatToolDelta(calls map[int]*pendingChatCall, delta chatToolCall, offered map[string]bool, send func(ModelEvent) bool) error {
	if delta.Index == nil || *delta.Index < 0 || *delta.Index >= maxChatTools {
		return protocolError()
	}
	index := *delta.Index
	call := calls[index]
	if call == nil {
		if delta.ID == "" || delta.Type != "function" || !offered[delta.Function.Name] {
			return protocolError()
		}
		for _, prior := range calls {
			if prior.wire.ID == delta.ID {
				return protocolError()
			}
		}
		call = &pendingChatCall{wire: delta}
		calls[index] = call
		if !send(ModelEvent{Type: "tool_call_start", CallID: delta.ID, ToolName: delta.Function.Name}) {
			return context.Canceled
		}
	} else if (delta.ID != "" && delta.ID != call.wire.ID) ||
		(delta.Type != "" && delta.Type != call.wire.Type) ||
		(delta.Function.Name != "" && delta.Function.Name != call.wire.Function.Name) {
		return protocolError()
	}
	arguments := delta.Function.Arguments
	if len(arguments) > maxToolArguments-call.arguments.Len() {
		return protocolError()
	}
	call.arguments.WriteString(arguments)
	if arguments != "" && !send(ModelEvent{Type: "tool_call_arguments_delta", CallID: call.wire.ID, ArgumentsDelta: arguments}) {
		return context.Canceled
	}
	return nil
}
