package ai

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes = 8 << 20
	modelTimeout     = 2 * time.Minute
)

type ChatCompletions struct {
	endpoint string
	apiKey   string
	client   http.Client
}

var _ Model = (*ChatCompletions)(nil)

// NewChatCompletions accepts the configured API base, including any version prefix.
func NewChatCompletions(baseURL, apiKey string, client *http.Client) (*ChatCompletions, error) {
	base, err := url.Parse(baseURL)
	if err != nil || base.Hostname() == "" || (base.Scheme != "http" && base.Scheme != "https") ||
		base.User != nil || base.Fragment != "" || strings.ContainsAny(apiKey, "\r\n") {
		return nil, &ModelError{Code: "ValidationError"}
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &ChatCompletions{
		endpoint: base.JoinPath("chat/completions").String(),
		apiKey:   apiKey,
		client: http.Client{
			Transport: client.Transport,
			Timeout:   modelTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	Refusal    string         `json:"refusal,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type chatRequest struct {
	Model           string               `json:"model"`
	Messages        []chatMessage        `json:"messages"`
	Temperature     *float64             `json:"temperature,omitempty"`
	TopP            *float64             `json:"top_p,omitempty"`
	MaxOutputTokens *int                 `json:"max_completion_tokens,omitempty"`
	Stop            []string             `json:"stop,omitempty"`
	Stream          bool                 `json:"stream,omitempty"`
	Tools           []chatToolDefinition `json:"tools,omitempty"`
	ToolChoice      any                  `json:"tool_choice,omitempty"`
	StreamOptions   map[string]bool      `json:"stream_options,omitempty"`
}

type chatResponse struct {
	Error   json.RawMessage `json:"error"`
	Choices []struct {
		Index        int         `json:"index"`
		Message      chatMessage `json:"message"`
		Delta        chatMessage `json:"delta"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		InputTokens  *int `json:"prompt_tokens"`
		OutputTokens *int `json:"completion_tokens"`
		TotalTokens  *int `json:"total_tokens"`
	} `json:"usage"`
}

func (r chatResponse) usage() *Usage {
	if r.Usage == nil {
		return nil
	}
	return &Usage{InputTokens: r.Usage.InputTokens, OutputTokens: r.Usage.OutputTokens, TotalTokens: r.Usage.TotalTokens}
}

func (c *ChatCompletions) request(ctx context.Context, req *ModelRequest, stream bool) (*http.Response, error) {
	if req == nil || strings.TrimSpace(req.Model) == "" || len(req.Messages) == 0 {
		return nil, &ModelError{Code: "ValidationError"}
	}
	payload := chatRequest{
		Model: req.Model, Temperature: req.Temperature, TopP: req.TopP,
		MaxOutputTokens: req.MaxOutputTokens, Stop: req.Stop, Stream: stream,
	}
	if stream {
		payload.StreamOptions = map[string]bool{"include_usage": true}
	}
	if err := encodeChatHistory(&payload, req); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, &ModelError{Code: "ValidationError"}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, &ModelError{Code: "ValidationError"}
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, modelIOError(err, "ProviderConnectionError")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, chatStatusError(response)
	}
	return response, nil
}

func chatStatusError(response *http.Response) error {
	switch response.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &ModelError{Code: "AuthenticationError"}
	case http.StatusTooManyRequests:
		value := response.Header.Get("Retry-After")
		var delay time.Duration
		if seconds, err := strconv.ParseUint(value, 10, 31); err == nil {
			delay = time.Duration(seconds) * time.Second
		} else if when, err := http.ParseTime(value); err == nil {
			delay = max(0, time.Until(when))
		}
		return &ModelError{Code: "RateLimitError", RetryAfter: delay}
	default:
		return &ModelError{Code: "ProviderHTTPError"}
	}
}

func modelIOError(err error, fallback string) error {
	switch {
	case errors.Is(err, context.Canceled):
		return &ModelError{Code: "Cancelled"}
	case errors.Is(err, context.DeadlineExceeded):
		return &ModelError{Code: "Timeout"}
	default:
		return &ModelError{Code: fallback}
	}
}

func protocolError() error { return &ModelError{Code: "ProviderProtocolError"} }

func (c *ChatCompletions) Generate(ctx context.Context, req *ModelRequest) (*ModelResponse, error) {
	response, err := c.request(ctx, req, false)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, modelIOError(err, "ProviderProtocolError")
	}
	var result chatResponse
	if len(body) > maxResponseBytes || json.Unmarshal(body, &result) != nil ||
		hasChatError(result) || len(result.Choices) != 1 {
		return nil, protocolError()
	}
	choice := result.Choices[0]
	if choice.Index != 0 || choice.Message.Role != "assistant" || choice.FinishReason == "" {
		return nil, protocolError()
	}
	calls, err := completeChatCalls(choice.Message.ToolCalls, offeredChatTools(req))
	if err != nil {
		return nil, err
	}
	finish, err := chatFinish(choice.FinishReason, choice.Message.Refusal != "", len(calls))
	if err != nil {
		return nil, err
	}
	return &ModelResponse{
		Message:      Message{Role: "assistant", ToolCalls: calls, Content: []ContentBlock{{Type: "text", Text: choice.Message.Content + choice.Message.Refusal}}},
		FinishReason: finish, Usage: result.usage(),
	}, nil
}

func hasChatError(response chatResponse) bool {
	return len(response.Error) > 0 && string(response.Error) != "null"
}

func chatFinish(reason string, refused bool, calls int) (string, error) {
	if (reason == "tool_calls") != (calls > 0) || (refused && calls > 0) {
		return "", protocolError()
	}
	switch reason {
	case "stop", "length", "content_filter", "tool_calls":
	case "function_call":
		return "", &ModelError{Code: "CapabilityError"}
	default:
		return "", protocolError()
	}
	if refused {
		return "refusal", nil
	}
	return reason, nil
}

func (c *ChatCompletions) Stream(ctx context.Context, req *ModelRequest) (<-chan ModelEvent, error) {
	streamContext, cancel := context.WithTimeout(ctx, modelTimeout)
	response, err := c.request(streamContext, req, true)
	if err != nil {
		cancel()
		return nil, err
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if mediaType != "text/event-stream" {
		cancel()
		response.Body.Close()
		return nil, protocolError()
	}

	offered := offeredChatTools(req)
	events := make(chan ModelEvent, 1)
	go func() {
		defer close(events)
		defer cancel()
		defer response.Body.Close()
		id := rand.Text()
		var sequence uint64
		send := func(event ModelEvent) bool {
			sequence++
			event.Sequence, event.MessageID = sequence, id
			// An available slot also lets a reader observe cancellation after the HTTP body stops.
			select {
			case events <- event:
				return true
			default:
			}
			select {
			case events <- event:
				return true
			case <-streamContext.Done():
				return false
			}
		}
		if err := readChatStream(response.Body, offered, send); err != nil {
			if streamContext.Err() != nil {
				err = modelIOError(streamContext.Err(), "ProviderProtocolError")
			}
			send(ModelEvent{Type: "error", Error: err})
		}
	}()
	return events, nil
}

func readChatStream(body io.Reader, offered map[string]bool, send func(ModelEvent) bool) error {
	const maxEventBytes = 256 << 10
	scanner := bufio.NewScanner(io.LimitReader(body, maxResponseBytes))
	scanner.Buffer(make([]byte, 4096), maxEventBytes)
	var data strings.Builder
	var finish string
	var refused bool
	calls := make(map[int]*pendingChatCall)
	if !send(ModelEvent{Type: "message_start"}) {
		return context.Canceled
	}

	for scanner.Scan() {
		line := scanner.Text()
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len()+len(value)+1 > maxEventBytes {
				return protocolError()
			}
			if data.Len() > 0 {
				data.WriteByte('\n')
			}
			data.WriteString(strings.TrimPrefix(value, " "))
			continue
		}
		if line != "" || data.Len() == 0 {
			continue
		}
		payload := data.String()
		data.Reset()
		if payload == "[DONE]" {
			if finish == "" {
				return protocolError()
			}
			reason, err := chatFinish(finish, refused, len(calls))
			if err != nil {
				return err
			}
			// Validate the whole batch before emitting any completed call.
			completed := make([]chatToolCall, len(calls))
			for index, call := range calls {
				if index >= len(completed) {
					return protocolError()
				}
				completed[index] = call.wire
				completed[index].Function.Arguments = call.arguments.String()
			}
			normalized, err := completeChatCalls(completed, offered)
			if err != nil {
				return err
			}
			for _, call := range normalized {
				if !send(ModelEvent{Type: "tool_call_end", CallID: call.ID, ToolName: call.Name, ToolCall: &call}) {
					return context.Canceled
				}
			}
			send(ModelEvent{Type: "message_end", FinishReason: reason})
			return nil
		}
		var chunk chatResponse
		if json.Unmarshal([]byte(payload), &chunk) != nil || hasChatError(chunk) || len(chunk.Choices) > 1 {
			return protocolError()
		}
		if len(chunk.Choices) == 1 {
			choice := chunk.Choices[0]
			if choice.Index != 0 || finish != "" {
				return protocolError()
			}
			refused = refused || choice.Delta.Refusal != ""
			if delta := choice.Delta.Content + choice.Delta.Refusal; delta != "" && !send(ModelEvent{Type: "text_delta", Text: delta}) {
				return context.Canceled
			}
			for _, delta := range choice.Delta.ToolCalls {
				if err := appendChatToolDelta(calls, delta, offered, send); err != nil {
					return err
				}
			}
			finish = choice.FinishReason
		}
		if usage := chunk.usage(); usage != nil && !send(ModelEvent{Type: "usage", Usage: usage}) {
			return context.Canceled
		}
	}
	return modelIOError(scanner.Err(), "ProviderProtocolError")
}
