// Package agentrun coordinates bounded model turns with explicitly registered local reads.
package agentrun

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

const (
	maxTurns             = 16
	maxCalls             = 64
	maxContinuationBytes = 128 << 10
	maxToolReplyBytes    = 12 << 10
)

type ReadTool struct {
	Definition ai.ToolDefinition
	Read       func(context.Context, json.RawMessage) string
}

// Stream retains the one-turn event contract. Only the final model turn ends the message.
func Stream(ctx context.Context, model ai.Model, request ai.ModelRequest, tools []ReadTool) <-chan ai.ModelEvent {
	events := make(chan ai.ModelEvent, 1)
	go func() {
		defer close(events)
		runCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
		defer cancel()
		err := run(runCtx, model, request, tools, func(event ai.ModelEvent) bool {
			return emit(runCtx, events, event)
		})
		if err != nil && ctx.Err() == nil {
			if runCtx.Err() == context.DeadlineExceeded {
				err = failure("Timeout")
			}
			emit(ctx, events, ai.ModelEvent{Type: "error", Error: err})
		}
	}()
	return events
}

func emit(ctx context.Context, events chan<- ai.ModelEvent, event ai.ModelEvent) bool {
	select {
	case <-ctx.Done():
		return false
	case events <- event:
		return true
	}
}

type previousCall struct {
	call   ai.ToolCall
	result string
}

func run(ctx context.Context, model ai.Model, request ai.ModelRequest, tools []ReadTool, forward func(ai.ModelEvent) bool) error {
	allowed := make(map[string]ReadTool, len(tools))
	request.Tools = nil
	for _, tool := range tools {
		name := tool.Definition.Name
		if name == "" || tool.Read == nil || allowed[name].Read != nil {
			return failure("ValidationError")
		}
		allowed[name] = tool
		request.Tools = append(request.Tools, tool.Definition)
	}
	request.Messages = append([]ai.Message(nil), request.Messages...)
	executed := make(map[string]previousCall)
	var attempts, continuationBytes int
	for turn := 0; turn < maxTurns; turn++ {
		reply, err := collect(ctx, model, &request, forward)
		if err != nil {
			return err
		}
		if reply.finish != "tool_calls" {
			if len(reply.calls) != 0 {
				return failure("ProviderProtocolError")
			}
			forward(ai.ModelEvent{Type: "message_end", FinishReason: reply.finish})
			return nil
		}
		if len(reply.calls) == 0 {
			return failure("ProviderProtocolError")
		}
		if turn+1 == maxTurns || len(reply.calls) > maxCalls-attempts {
			return failure("AgentLimitExceeded")
		}
		if err := validateCalls(reply.calls, allowed, executed); err != nil {
			return err
		}
		continuationBytes += len(reply.text)
		for _, call := range reply.calls {
			continuationBytes += len(call.ArgumentsJSON) + len(call.Name) + len(call.ID)
		}
		if continuationBytes > maxContinuationBytes {
			return failure("AgentLimitExceeded")
		}
		attempts += len(reply.calls)
		request.Messages = append(request.Messages, ai.Message{Role: "assistant", Content: textContent(reply.text), ToolCalls: reply.calls})
		for _, call := range reply.calls {
			if err := ctx.Err(); err != nil {
				return err
			}
			record, exists := executed[call.ID]
			if !exists {
				record = previousCall{call: call, result: allowed[call.Name].Read(ctx, json.RawMessage(call.ArgumentsJSON))}
				if len(record.result) > maxToolReplyBytes {
					return failure("AgentLimitExceeded")
				}
				executed[call.ID] = record
			}
			continuationBytes += len(record.result)
			if continuationBytes > maxContinuationBytes {
				return failure("AgentLimitExceeded")
			}
			request.Messages = append(request.Messages, ai.Message{Role: "tool", ToolCallID: call.ID, Content: textContent(record.result)})
		}
	}
	return failure("AgentLimitExceeded")
}

func validateCalls(calls []ai.ToolCall, allowed map[string]ReadTool, executed map[string]previousCall) error {
	seen := make(map[string]bool, len(calls))
	for _, call := range calls {
		if allowed[call.Name].Read == nil {
			return failure("ToolNotAllowed")
		}
		if call.ID == "" || seen[call.ID] {
			return failure("ProviderProtocolError")
		}
		seen[call.ID] = true
		if prior, ok := executed[call.ID]; ok && prior.call != call {
			return failure("ToolCallConflict")
		}
	}
	return nil
}

type turnReply struct {
	text   string
	finish string
	calls  []ai.ToolCall
}

func collect(ctx context.Context, model ai.Model, request *ai.ModelRequest, forward func(ai.ModelEvent) bool) (turnReply, error) {
	var reply turnReply
	if err := ctx.Err(); err != nil {
		return reply, err
	}
	turnCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events, err := model.Stream(turnCtx, request)
	if err != nil {
		return reply, err
	}
	var text strings.Builder
	for {
		select {
		case <-ctx.Done():
			return reply, ctx.Err()
		case event, ok := <-events:
			if !ok {
				if reply.finish == "" {
					return reply, failure("ProviderProtocolError")
				}
				reply.text = text.String()
				return reply, nil
			}
			if reply.finish != "" {
				return reply, failure("ProviderProtocolError")
			}
			switch event.Type {
			case "text_delta":
				if text.Len()+len(event.Text) > maxContinuationBytes {
					return reply, failure("ResponseLimit")
				}
				text.WriteString(event.Text)
				if !forward(event) {
					return reply, ctx.Err()
				}
			case "tool_call_end":
				if event.ToolCall == nil || len(reply.calls) >= maxCalls {
					return reply, failure("ProviderProtocolError")
				}
				reply.calls = append(reply.calls, *event.ToolCall)
			case "message_end":
				reply.finish = event.FinishReason
			case "error":
				if event.Error == nil {
					return reply, failure("ProviderProtocolError")
				}
				return reply, event.Error
			}
		}
	}
}

func textContent(text string) []ai.ContentBlock {
	return []ai.ContentBlock{{Type: "text", Text: text}}
}

func failure(code string) error { return &ai.ModelError{Code: code} }
