package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
