package agentrun

import (
	"context"
	"strconv"
	"testing"

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
	if reads != 1 || turn != 3 || len(output) != 2 || output[0].Text != "done" || output[1].FinishReason != "stop" {
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
