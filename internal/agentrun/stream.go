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
	maxToolReplyBytes    = ResultPageBytes
)

type ReadTool struct {
	Definition   ai.ToolDefinition
	Read         func(context.Context, ai.ToolCall) (string, error)
	ArticleNext  func(context.Context, ai.ToolCall, string) (string, error)
	activityCall func(ai.ToolCall) ai.ToolCall
}

type StreamOptions struct {
	MaxRetries int
	BaseDelay  time.Duration
}

// Stream retains the one-turn event contract. Only the final model turn ends the message.
func Stream(ctx context.Context, model ai.Model, request ai.ModelRequest, tools []ReadTool) <-chan ai.ModelEvent {
	return StreamWithOptions(ctx, model, request, tools, StreamOptions{})
}

// StreamWithOptions runs the bounded agent loop with optional per-turn retries on transient errors.
func StreamWithOptions(ctx context.Context, model ai.Model, request ai.ModelRequest, tools []ReadTool, opts StreamOptions) <-chan ai.ModelEvent {
	events := make(chan ai.ModelEvent, 1)
	go func() {
		defer close(events)
		timeout := 180 * time.Second
		if opts.MaxRetries > 0 {
			timeout = 300 * time.Second
		}
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		runCtx = context.WithValue(runCtx, resultStoreKey{}, &ResultStore{})
		err := run(runCtx, model, request, tools, opts, func(event ai.ModelEvent) bool {
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

func run(ctx context.Context, model ai.Model, request ai.ModelRequest, tools []ReadTool, opts StreamOptions, forward func(ai.ModelEvent) bool) error {
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
	var attempts int
	baseSystem := request.System
	var toolSummary string
	compaction := &runCompaction{model: model, opts: opts, forward: forward}
	for turn := 0; turn < maxTurns; turn++ {
		if err := compaction.fitToolHistory(ctx, &request, baseSystem, &toolSummary); err != nil {
			return err
		}
		reply, err := collectWithRetry(ctx, model, &request, forward, opts)
		if err != nil {
			return err
		}
		if reply.finish != "tool_calls" {
			if len(reply.calls) != 0 {
				return failure("ProviderProtocolError")
			}
			if len(compaction.incomplete) > 0 {
				return failure("ArticleReadIncomplete")
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
		attempts += len(reply.calls)
		request.Messages = append(request.Messages, ai.Message{Role: "assistant", Content: textContent(reply.text), ToolCalls: reply.calls, ReasoningContent: reply.reasoning})
		for _, call := range reply.calls {
			if err := ctx.Err(); err != nil {
				return err
			}
			record, exists := executed[call.ID]
			if !exists {
				tool := allowed[call.Name]
				activity := call
				if tool.activityCall != nil {
					activity = tool.activityCall(call)
				}
				event := ai.ModelEvent{Type: "tool_work_started", CallID: call.ID, ToolName: activity.Name, ToolCall: &activity}
				if !forward(event) {
					return ctx.Err()
				}
				result, err := tool.Read(ctx, call)
				if err == nil {
					if len(result) > maxToolReplyBytes {
						result = `{"error":"ToolResultLimit"}`
					} else {
						result, err = compaction.completeArticle(ctx, request, call, tool, result)
					}
				}
				event.Type = "tool_work_finished"
				event.ToolStatus = toolWorkStatus(result, err)
				event.ToolCall = nil
				if err == nil || result != "" {
					event.ToolOutput = &result
				}
				if !forward(event) {
					return ctx.Err()
				}
				if err != nil {
					return err
				}
				record = previousCall{call: call, result: result}
				executed[call.ID] = record
			}
			request.Messages = append(request.Messages, ai.Message{Role: "tool", ToolCallID: call.ID, Content: textContent(record.result)})
		}
	}
	return failure("AgentLimitExceeded")
}

func toolWorkStatus(output string, err error) string {
	if err != nil {
		return "failed"
	}
	var result struct {
		OK      *bool           `json:"ok"`
		IsError bool            `json:"isError"`
		Error   json.RawMessage `json:"error"`
	}
	if json.Unmarshal([]byte(output), &result) == nil {
		switch string(result.Error) {
		case `"PermissionDenied"`, `"PermissionError"`, `"ToolNotAllowed"`:
			return "denied"
		case `"MCPResultUnknown"`:
			return "unknown"
		}
		if (len(result.Error) > 0 && string(result.Error) != "null") || result.IsError || (result.OK != nil && !*result.OK) {
			return "failed"
		}
	}
	return "completed"
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
	text      string
	finish    string
	calls     []ai.ToolCall
	reasoning *string
}

func retryBackoff(attempt int, base, hint time.Duration) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	delay := base << min(attempt, 4)
	if base >= time.Second && delay > 10*time.Second {
		delay = 10 * time.Second
	}
	if hint > delay {
		delay = min(hint, 15*time.Second)
	}
	return delay
}

func collectWithRetry(ctx context.Context, model ai.Model, request *ai.ModelRequest, forward func(ai.ModelEvent) bool, opts StreamOptions) (turnReply, error) {
	maxRetries := max(0, opts.MaxRetries)
	for attempt := 0; ; attempt++ {
		reply, err := collect(ctx, model, request, forward)
		if err == nil {
			return reply, nil
		}
		if ctx.Err() != nil || attempt >= maxRetries {
			return reply, err
		}
		retryable, hint := ai.IsRetryableError(err)
		if !retryable {
			return reply, err
		}
		if !forward(ai.ModelEvent{
			Type:         "retry_scheduled",
			RetryAttempt: attempt + 1,
			RetryMax:     maxRetries,
			Error:        err,
		}) {
			return reply, ctx.Err()
		}
		timer := time.NewTimer(retryBackoff(attempt, opts.BaseDelay, hint))
		select {
		case <-ctx.Done():
			timer.Stop()
			return reply, ctx.Err()
		case <-timer.C:
		}
	}
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
			case "message_start", "usage", "reasoning_text_delta", "reasoning_summary_delta":
				if !forward(event) {
					return reply, ctx.Err()
				}
			case "message_end":
				reply.finish = event.FinishReason
				reply.reasoning = event.ReasoningContent
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
