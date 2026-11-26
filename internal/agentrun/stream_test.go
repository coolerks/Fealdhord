package agentrun

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/coolerks/Fealdhord/internal/ai"
)

type streamModel func(context.Context, *ai.ModelRequest) (<-chan ai.ModelEvent, error)

func (m streamModel) Stream(ctx context.Context, request *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
	return m(ctx, request)
}

func (streamModel) Generate(context.Context, *ai.ModelRequest) (*ai.ModelResponse, error) {
	panic("unexpected non-streaming request")
}

func modelEvents(events ...ai.ModelEvent) <-chan ai.ModelEvent {
	ch := make(chan ai.ModelEvent, len(events))
	for _, event := range events {
		ch <- event
	}
	close(ch)
	return ch
}

func TestToolLoopWaitsForTerminalAndReusesPriorResult(t *testing.T) {
	reads, turn := 0, 0
	call := ai.ToolCall{ID: "call", Name: "read", ArgumentsJSON: "{}"}
	tool := ReadTool{Definition: ai.ToolDefinition{Name: "read"}, Read: func(context.Context, ai.ToolCall) (string, error) {
		reads++
		return "approved text", nil
	}}
	model := streamModel(func(_ context.Context, req *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
		turn++
		if len(req.Tools) != 1 {
			t.Errorf("tools=%+v", req.Tools)
		}
		if turn > 1 {
			result := req.Messages[len(req.Messages)-1]
			if result.Role != "tool" || result.ToolCallID != call.ID || result.Content[0].Text != "approved text" {
				t.Errorf("continuation=%+v", result)
			}
		}
		if turn == 3 {
			return modelEvents(ai.ModelEvent{Type: "text_delta", Text: "done"}, ai.ModelEvent{Type: "message_end", FinishReason: "stop"}), nil
		}
		return modelEvents(ai.ModelEvent{Type: "tool_call_end", ToolCall: &call}, ai.ModelEvent{Type: "message_end", FinishReason: "tool_calls"}), nil
	})
	var output []ai.ModelEvent
	for event := range Stream(context.Background(), model, ai.ModelRequest{}, []ReadTool{tool}) {
		output = append(output, event)
	}
	if reads != 1 || turn != 3 || len(output) != 4 ||
		output[0].Type != "tool_work_started" || output[0].ToolName != call.Name ||
		output[1].Type != "tool_work_finished" || output[1].ToolStatus != "completed" ||
		output[2].Text != "done" || output[3].FinishReason != "stop" {
		t.Fatalf("reads=%d turns=%d output=%+v", reads, turn, output)
	}
}

func TestToolLoopStopsBeforeUnsafeCalls(t *testing.T) {
	for _, tc := range []struct {
		name, want              string
		callsPerTurn, wantReads int
	}{
		{"missing terminal", "ProviderProtocolError", 1, 0},
		{"unknown tool", "ToolNotAllowed", 1, 0},
		{"turn limit", "AgentLimitExceeded", 1, 15},
		{"call limit", "AgentLimitExceeded", 32, 64},
		{"cancel", "", 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reads, turns := 0, 0
			tool := ReadTool{Definition: ai.ToolDefinition{Name: "read"}, Read: func(context.Context, ai.ToolCall) (string, error) {
				reads++
				if tc.name == "cancel" {
					cancel()
				}
				return "content", nil
			}}
			model := streamModel(func(_ context.Context, _ *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
				turns++
				var events []ai.ModelEvent
				for i := 0; i < tc.callsPerTurn; i++ {
					name := "read"
					if tc.name == "unknown tool" {
						name = "unregistered"
					}
					call := ai.ToolCall{ID: strconv.Itoa(turns*100 + i), Name: name, ArgumentsJSON: "{}"}
					events = append(events, ai.ModelEvent{Type: "tool_call_end", ToolCall: &call})
				}
				if tc.name != "missing terminal" {
					events = append(events, ai.ModelEvent{Type: "message_end", FinishReason: "tool_calls"})
				}
				return modelEvents(events...), nil
			})
			got := ""
			for event := range Stream(ctx, model, ai.ModelRequest{}, []ReadTool{tool}) {
				if event.Error != nil {
					got = event.Error.Error()
				}
			}
			if got != tc.want || reads != tc.wantReads {
				t.Fatalf("got %q, %d reads", got, reads)
			}
		})
	}
}

func TestStreamWithOptionsRetriesTransientErrorsAndPreservesPriorTools(t *testing.T) {
	var attempts, toolCalls int
	model := streamModel(func(_ context.Context, _ *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
		attempts++
		switch attempts {
		case 1:
			call := ai.ToolCall{ID: "c1", Name: "read", ArgumentsJSON: "{}"}
			return modelEvents(
				ai.ModelEvent{Type: "tool_call_end", ToolCall: &call},
				ai.ModelEvent{Type: "message_end", FinishReason: "tool_calls"},
			), nil
		case 2:
			// Simulate mid-stream cut off (missing message_end -> ProviderProtocolError)
			return modelEvents(ai.ModelEvent{Type: "text_delta", Text: "残缺"}), nil
		case 3:
			// Simulate transient connection failure
			return nil, &ai.ModelError{Code: "ProviderConnectionError"}
		default:
			return modelEvents(
				ai.ModelEvent{Type: "text_delta", Text: "重试成功"},
				ai.ModelEvent{Type: "message_end", FinishReason: "stop"},
			), nil
		}
	})
	tool := ReadTool{
		Definition: ai.ToolDefinition{Name: "read"},
		Read: func(context.Context, ai.ToolCall) (string, error) {
			toolCalls++
			return "ok", nil
		},
	}

	var retries []int
	var finished bool
	var lastErr error
	for event := range StreamWithOptions(context.Background(), model, ai.ModelRequest{}, []ReadTool{tool}, StreamOptions{
		MaxRetries: 5,
		BaseDelay:  time.Millisecond,
	}) {
		if event.Type == "retry_scheduled" {
			retries = append(retries, event.RetryAttempt)
		}
		if event.Type == "message_end" {
			finished = true
		}
		if event.Error != nil && event.Type == "error" {
			lastErr = event.Error
		}
	}
	if !finished || lastErr != nil {
		t.Fatalf("expected retry success, finished=%v err=%v", finished, lastErr)
	}
	if toolCalls != 1 {
		t.Fatalf("expected prior tool call preserved (1), got %d", toolCalls)
	}
	if len(retries) != 2 || retries[0] != 1 || retries[1] != 2 {
		t.Fatalf("unexpected retry attempts: %v", retries)
	}
}

func TestStreamWithOptionsDoesNotRetryAuthErrors(t *testing.T) {
	var attempts int
	model := streamModel(func(_ context.Context, _ *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
		attempts++
		return nil, &ai.ModelError{Code: "AuthenticationError"}
	})
	var got string
	for event := range StreamWithOptions(context.Background(), model, ai.ModelRequest{}, nil, StreamOptions{
		MaxRetries: 5,
		BaseDelay:  time.Millisecond,
	}) {
		if event.Type == "error" && event.Error != nil {
			got = event.Error.Error()
		}
	}
	if attempts != 1 || got != "AuthenticationError" {
		t.Fatalf("expected 1 attempt with AuthenticationError, got %d (%q)", attempts, got)
	}
}

func TestConfiguredLimitsDistinguishRoundsAndCalls(t *testing.T) {
	for _, tc := range []struct {
		name, kind                string
		options                   StreamOptions
		perRound, reads, expected int
	}{
		{"rounds", "model_rounds", StreamOptions{MaxTurns: 200, MaxCalls: 300}, 1, 199, 200},
		{"calls", "tool_calls", StreamOptions{MaxTurns: 200, MaxCalls: 300}, 2, 300, 300},
		{"within limit", "", StreamOptions{MaxTurns: 3, MaxCalls: 2}, 2, 2, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			turn, reads := 0, 0
			tool := ReadTool{Definition: ai.ToolDefinition{Name: "read"}, Read: func(context.Context, ai.ToolCall) (string, error) {
				reads++
				return "ok", nil
			}}
			model := streamModel(func(_ context.Context, _ *ai.ModelRequest) (<-chan ai.ModelEvent, error) {
				turn++
				events := []ai.ModelEvent{{Type: "text_delta", Text: "partial"}}
				if tc.kind == "" && turn == 2 {
					return modelEvents(append(events, ai.ModelEvent{Type: "message_end", FinishReason: "stop"})...), nil
				}
				for n := range tc.perRound {
					call := ai.ToolCall{ID: fmt.Sprintf("%d-%d", turn, n), Name: "read"}
					events = append(events, ai.ModelEvent{Type: "tool_call_end", ToolCall: &call})
				}
				return modelEvents(append(events, ai.ModelEvent{Type: "message_end", FinishReason: "tool_calls"})...), nil
			})
			var limit *LimitError
			var text string
			var finished bool
			for event := range StreamWithOptions(t.Context(), model, ai.ModelRequest{}, []ReadTool{tool}, tc.options) {
				switch event.Type {
				case "error":
					if !errors.As(event.Error, &limit) {
						t.Fatalf("unexpected error: %v", event.Error)
					}
				case "text_delta":
					text += event.Text
				case "message_end":
					finished = true
				}
			}
			if reads != tc.reads || text == "" || finished != (tc.kind == "") {
				t.Fatalf("reads=%d, finished=%t, text length=%d", reads, finished, len(text))
			}
			if tc.kind != "" && (limit == nil || limit.Limit != (RunLimit{Kind: tc.kind, Value: tc.expected})) {
				t.Fatalf("unexpected limit: %+v", limit)
			}
		})
	}
}
