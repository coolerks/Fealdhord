// Package ai defines the single-turn model boundary used by Chat and Runtime.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Model interface {
	Generate(context.Context, *ModelRequest) (*ModelResponse, error)
	Stream(context.Context, *ModelRequest) (<-chan ModelEvent, error)
}

type ModelRequest struct {
	Budget                  Budget
	Model                   string
	ReasoningEffort         string
	IncludeReasoningSummary bool
	System                  string
	Messages                []Message
	Tools                   []ToolDefinition
	ToolChoice              ToolChoice
	Temperature             *float64
	TopP                    *float64
	MaxOutputTokens         *int
	Stop                    []string
}

type Message struct {
	Role             string
	Content          []ContentBlock
	ToolCalls        []ToolCall
	ToolCallID       string
	ReasoningContent *string
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

type ToolChoice struct {
	Mode string // auto, none, required, or function
	Name string
}

type ToolCall struct {
	ID            string
	Name          string
	ArgumentsJSON string
}

type ContentBlock struct {
	Type     string
	Text     string
	MIMEType string
	Data     string
}

type Usage struct {
	InputTokens       *int
	CachedInputTokens *int
	OutputTokens      *int
	TotalTokens       *int
}

type ModelResponse struct {
	Message      Message
	FinishReason string
	Usage        *Usage
}

type ModelEvent struct {
	Type             string
	MessageID        string
	Sequence         uint64
	Text             string
	CallID           string
	ToolName         string
	ToolStatus       string
	ToolOutput       *string
	ArgumentsDelta   string
	ToolCall         *ToolCall
	Usage            *Usage
	FinishReason     string
	ReasoningContent *string
	RetryAttempt     int
	RetryMax         int
	Error            error
}

type ModelError struct {
	Code       string
	RetryAfter time.Duration
	httpStatus int
}

func (e *ModelError) Error() string { return e.Code }

// Retryable reports whether a transient network, stream, timeout, rate-limit, or 5xx server error can be retried.
func (e *ModelError) Retryable() bool {
	if e == nil {
		return false
	}
	switch e.Code {
	case "ProviderConnectionError", "ProviderProtocolError", "RateLimitError", "Timeout":
		return true
	case "ProviderHTTPError":
		return e.httpStatus == 0 || e.httpStatus == 408 || e.httpStatus == 429 || e.httpStatus >= 500
	default:
		return false
	}
}

// IsRetryableError reports whether err is a transient model failure and returns any server-provided Retry-After hint.
func IsRetryableError(err error) (bool, time.Duration) {
	if err == nil {
		return false, 0
	}
	var typed *ModelError
	if errors.As(err, &typed) {
		return typed.Retryable(), typed.RetryAfter
	}
	return false, 0
}
