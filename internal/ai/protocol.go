package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
)

type tokenUsage struct {
	Input  *int `json:"input_tokens"`
	Output *int `json:"output_tokens"`
	Total  *int `json:"total_tokens"`
}

func (u *tokenUsage) normalized() *Usage {
	if u == nil {
		return nil
	}
	total := u.Total
	if total == nil && u.Input != nil && u.Output != nil {
		sum := *u.Input + *u.Output
		total = &sum
	}
	return &Usage{InputTokens: u.Input, OutputTokens: u.Output, TotalTokens: total}
}

func decodeModelResponse(response *http.Response, result any) error {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return modelIOError(err, "ProviderProtocolError")
	}
	if len(body) > maxResponseBytes || json.Unmarshal(body, result) != nil {
		return protocolError()
	}
	return nil
}

func wireToolCall(id, name, arguments string) chatToolCall {
	return chatToolCall{ID: id, Type: "function", Function: chatFunction{Name: name, Arguments: arguments}}
}

func emit(send func(ModelEvent) bool, event ModelEvent) error {
	if !send(event) {
		return context.Canceled
	}
	return nil
}

func emitArguments(send func(ModelEvent) bool, call *pendingChatCall, delta string) error {
	if len(delta) > maxToolArguments-call.arguments.Len() {
		return protocolError()
	}
	if delta == "" {
		return nil
	}
	call.arguments.WriteString(delta)
	return emit(send, ModelEvent{Type: "tool_call_arguments_delta", CallID: call.wire.ID, ArgumentsDelta: delta})
}

func emitResult(send func(ModelEvent) bool, result *ModelResponse) error {
	for _, call := range result.Message.ToolCalls {
		if err := emit(send, ModelEvent{Type: "tool_call_end", CallID: call.ID, ToolName: call.Name, ToolCall: &call}); err != nil {
			return err
		}
	}
	if result.Usage != nil {
		if err := emit(send, ModelEvent{Type: "usage", Usage: result.Usage}); err != nil {
			return err
		}
	}
	return emit(send, ModelEvent{Type: "message_end", FinishReason: result.FinishReason})
}
