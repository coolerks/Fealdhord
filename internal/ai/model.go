// Package ai defines the single-turn model boundary used by Chat and Runtime.
package ai

import (
	"context"
	"encoding/json"
	"time"
)

type Model interface {
	Generate(context.Context, *ModelRequest) (*ModelResponse, error)
	Stream(context.Context, *ModelRequest) (<-chan ModelEvent, error)
}

type ModelRequest struct {
	Model           string
	System          string
	Messages        []Message
	Tools           []ToolDefinition
	ToolChoice      ToolChoice
	Temperature     *float64
	TopP            *float64
	MaxOutputTokens *int
	Stop            []string
}

type Message struct {
	Role       string
	Content    []ContentBlock
	ToolCalls  []ToolCall
	ToolCallID string
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
	Type string
	Text string
}

type Usage struct {
	InputTokens  *int
	OutputTokens *int
	TotalTokens  *int
}

type ModelResponse struct {
	Message      Message
	FinishReason string
	Usage        *Usage
}

type ModelEvent struct {
	Type           string
	MessageID      string
	Sequence       uint64
	Text           string
	CallID         string
	ToolName       string
	ArgumentsDelta string
	ToolCall       *ToolCall
	Usage          *Usage
	FinishReason   string
	Error          error
}

type ModelError struct {
	Code       string
	RetryAfter time.Duration
}

func (e *ModelError) Error() string { return e.Code }
