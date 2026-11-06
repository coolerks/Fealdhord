package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func chatFixture(t *testing.T, handler http.HandlerFunc) (*ChatCompletions, *ModelRequest) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	model, err := NewChatCompletions(server.URL+"/v1", "fixture-secret", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	zero := 0.0
	return model, &ModelRequest{
		Model: "test-model", System: "用中文回复", Temperature: &zero,
		Messages: []Message{{Role: "user", Content: []ContentBlock{{Type: "text", Text: "你好"}}}},
	}
}

func TestChatGenerateWire(t *testing.T) {
	model, request := chatFixture(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/v1/chat/completions" ||
			r.Header.Get("Authorization") != "Bearer fixture-secret" {
			t.Errorf("请求路径或认证不正确")
		}
		var payload chatRequest
		if json.NewDecoder(r.Body).Decode(&payload) != nil || payload.Stream ||
			payload.Temperature == nil || *payload.Temperature != 0 || len(payload.Messages) != 2 ||
			payload.Messages[0].Role != "system" || payload.Messages[1].Content != "你好" {
			t.Errorf("请求体不正确：%+v", payload)
		}
		io.WriteString(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"你好"},"finish_reason":"stop"}],"usage":{"completion_tokens":0}}`)
	})
	result, err := model.Generate(context.Background(), request)
	if err != nil || result.Message.Content[0].Text != "你好" || result.FinishReason != "stop" ||
		result.Usage.InputTokens != nil || result.Usage.OutputTokens == nil || *result.Usage.OutputTokens != 0 {
		t.Fatalf("响应归一化失败：%+v %v", result, err)
	}
}

func TestChatStreamTerminalAndUsage(t *testing.T) {
	for _, test := range []struct {
		name, tail, terminal string
	}{
		{"完成", "data: [DONE]\n\n", "message_end"},
		{"提前断开", "", "error"},
		{"坏帧", "data: {\n\n", "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model, request := chatFixture(t, func(w http.ResponseWriter, r *http.Request) {
				var payload chatRequest
				if json.NewDecoder(r.Body).Decode(&payload) != nil || !payload.Stream || !payload.StreamOptions["include_usage"] {
					t.Error("未请求流式用量")
				}
				w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
				io.WriteString(w, ": comment\r\n\r\ndata: {\"choices\":[\r\ndata: {\"delta\":{\"content\":\"你好\"},\"finish_reason\":\"stop\"}]}\r\n\r\n")
				io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"total_tokens\":3}}\n\n"+test.tail)
			})
			stream, err := model.Stream(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			var got []ModelEvent
			for event := range stream {
				got = append(got, event)
				if event.MessageID == "" || event.MessageID != got[0].MessageID || event.Sequence != uint64(len(got)) {
					t.Fatal("事件身份或序号不正确")
				}
			}
			if len(got) != 4 || got[0].Type != "message_start" || got[1].Text != "你好" ||
				got[2].Usage == nil || *got[2].Usage.TotalTokens != 3 || got[3].Type != test.terminal {
				t.Fatalf("流式事件不正确：%+v", got)
			}
		})
	}
}

func TestChatStreamRepeatedFinishInUsageFrame(t *testing.T) {
	for _, test := range []struct {
		name, usageChoice string
		wantError         bool
	}{
		{"用量帧", `{"index":0,"delta":{"content":"","role":"assistant"},"finish_reason":"stop"}`, false},
		{"结束原因冲突", `{"index":0,"delta":{},"finish_reason":"length"}`, true},
		{"结束后仍有正文", `{"index":0,"delta":{"content":"unexpected"},"finish_reason":"stop"}`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			stream := "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"你好\"},\"finish_reason\":\"stop\"}]}\n\n" +
				"data: {\"choices\":[" + test.usageChoice + "],\"usage\":{\"total_tokens\":3}}\n\ndata: [DONE]\n\n"
			var events []ModelEvent
			err := readChatStream(strings.NewReader(stream), nil, func(event ModelEvent) bool {
				events = append(events, event)
				return true
			})
			if test.wantError {
				if err == nil || len(events) != 2 {
					t.Fatalf("err = %v, events = %+v", err, events)
				}
				return
			}
			if err != nil || len(events) != 4 || events[2].Usage == nil || events[3].FinishReason != "stop" {
				t.Fatalf("err = %v, events = %+v", err, events)
			}
		})
	}
}

func TestChatRateLimitAndCancellation(t *testing.T) {
	model, request := chatFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		http.Error(w, "fixture-secret", http.StatusTooManyRequests)
	})
	_, err := model.Generate(context.Background(), request)
	var typed *ModelError
	if !errors.As(err, &typed) || typed.Code != "RateLimitError" || typed.RetryAfter != 3*time.Second {
		t.Fatalf("限流未归一化：%v", err)
	}

	model, request = chatFixture(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream, err := model.Stream(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	<-stream // 已打开响应并开始发送事件。
	cancel()
	select {
	case event := <-stream:
		if event.Type != "error" || !errors.As(event.Error, &typed) || typed.Code != "Cancelled" {
			t.Fatalf("取消未归一化：%+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("取消没有停止流式读取")
	}
}

func TestChatToolsRoundTrip(t *testing.T) {
	model, req := chatFixture(t, func(w http.ResponseWriter, r *http.Request) {
		var payload chatRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("请求解码失败：%v", err)
			return
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Type != "function" ||
			payload.Tools[0].Function.Name != "lookup" {
			t.Errorf("工具声明错误：%+v", payload.Tools)
		}
		if len(payload.Messages) == 2 {
			choice, ok := payload.ToolChoice.(map[string]any)
			if !ok || choice["type"] != "function" {
				t.Errorf("工具选择错误：%v", payload.ToolChoice)
			}
			io.WriteString(w, `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"id\":1}"}}]},"finish_reason":"tool_calls"}]}`)
			return
		}
		if len(payload.Messages) != 4 {
			t.Errorf("续轮消息数量错误：%d", len(payload.Messages))
			return
		}
		assistant, result := payload.Messages[2], payload.Messages[3]
		if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 ||
			assistant.ToolCalls[0].ID != "call_1" || assistant.ToolCalls[0].Function.Arguments != `{"id":1}` ||
			result.Role != "tool" || result.ToolCallID != "call_1" || result.Content != "找到文章" {
			t.Errorf("续轮请求不正确：%+v %+v", assistant, result)
		}
		io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"已找到"},"finish_reason":"stop"}]}`)
	})
	req.Tools = []ToolDefinition{{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}}
	req.ToolChoice = ToolChoice{Mode: "function", Name: "lookup"}
	first, err := model.Generate(context.Background(), req)
	if err != nil || first.FinishReason != "tool_calls" || len(first.Message.ToolCalls) != 1 {
		t.Fatalf("工具调用未归一化：%+v %v", first, err)
	}
	req.ToolChoice = ToolChoice{}
	req.Messages = append(req.Messages, first.Message, Message{
		Role: "tool", ToolCallID: first.Message.ToolCalls[0].ID,
		Content: []ContentBlock{{Type: "text", Text: "找到文章"}},
	})
	second, err := model.Generate(context.Background(), req)
	if err != nil || second.Message.Content[0].Text != "已找到" {
		t.Fatalf("工具结果续轮失败：%+v %v", second, err)
	}
	req.Messages[len(req.Messages)-1].ToolCallID = "unknown"
	_, err = model.Generate(context.Background(), req)
	var modelErr *ModelError
	if !errors.As(err, &modelErr) || modelErr.Code != "ValidationError" {
		t.Fatalf("错误调用 ID 未拒绝：%v", err)
	}
}

func TestChatToolStreamInterleaving(t *testing.T) {
	for _, test := range []struct {
		name, lastArgument, tail, terminal string
	}{
		{"完整", "}", "data: [DONE]\n\n", "message_end"},
		{"第二项未闭合", "", "data: [DONE]\n\n", "error"},
		{"缺少流终态", "}", "", "error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			model, req := chatFixture(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"a","type":"function","function":{"name":"lookup","arguments":"{\"x\":"}}]}}]}`+"\n\n")
				io.WriteString(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":1,"id":"b","type":"function","function":{"name":"lookup","arguments":"{\"y\":2"}}]}}]}`+"\n\n")
				io.WriteString(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}},{"index":1,"function":{"arguments":"`+test.lastArgument+`"}}]},"finish_reason":"tool_calls"}]}`+"\n\n")
				io.WriteString(w, test.tail)
			})
			req.Tools = []ToolDefinition{{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}}
			stream, err := model.Stream(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			var complete []ToolCall
			var last string
			deltas := map[string]string{}
			for event := range stream {
				last = event.Type
				switch event.Type {
				case "tool_call_arguments_delta":
					deltas[event.CallID] += event.ArgumentsDelta
				case "tool_call_end":
					complete = append(complete, *event.ToolCall)
				}
			}
			if last != test.terminal || deltas["a"] != `{"x":1}` || deltas["b"] != `{"y":2`+test.lastArgument {
				t.Fatalf("流式事件错误：%q %+v", last, deltas)
			}
			if test.terminal == "error" && len(complete) != 0 {
				t.Fatalf("异常流发布了完整调用：%+v", complete)
			}
			if test.terminal == "message_end" && (len(complete) != 2 ||
				complete[0].ArgumentsJSON != deltas["a"] || complete[1].ArgumentsJSON != deltas["b"]) {
				t.Fatalf("完整调用拼接错误：%+v", complete)
			}
		})
	}
}
