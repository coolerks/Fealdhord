package ai

import (
	"context"
	"unicode/utf8"
)

type Budget struct {
	Context       int  `json:"context"`
	InputLimit    int  `json:"inputLimit"`
	OutputReserve int  `json:"outputReserve"`
	Estimated     bool `json:"estimated"`
}

func ContextBudget(metadata Metadata) Budget {
	window := metadata.Context
	estimated := window == 0
	if estimated {
		window = 32768
	}
	output := min(8192, max(1, window/4))
	if metadata.OutputLimit > 0 {
		output = min(max(1, window/4), metadata.OutputLimit)
	}
	margin := min(512, max(1, window/20))
	input := window - output - margin
	if metadata.InputLimit > 0 {
		input = min(input, metadata.InputLimit)
	}
	return Budget{Context: window, InputLimit: input, OutputReserve: output, Estimated: estimated}
}

// EstimateTokens bounds mixed-script text more conservatively than a byte/4 heuristic.
func EstimateTokens(text string) int {
	var ascii, multibyte int
	for _, r := range text {
		if r < utf8.RuneSelf {
			ascii++
		} else {
			multibyte++
		}
	}
	return (ascii+2)/3 + multibyte*2
}

func EstimateRequest(request ModelRequest) int {
	count := EstimateTokens(request.System)
	for _, message := range request.Messages {
		count += 8
		for _, content := range message.Content {
			count += EstimateTokens(content.Text)
		}
		for _, call := range message.ToolCalls {
			count += 16 + EstimateTokens(call.ID) + EstimateTokens(call.Name) + EstimateTokens(call.ArgumentsJSON)
		}
		count += EstimateTokens(message.ToolCallID)
		if message.ReasoningContent != nil {
			count += EstimateTokens(*message.ReasoningContent)
		}
	}
	for _, tool := range request.Tools {
		count += 16 + EstimateTokens(tool.Name) + EstimateTokens(tool.Description) + EstimateTokens(string(tool.InputSchema))
	}
	return count
}

// Fit removes whole old turns while preserving the latest user message and its tool continuations.
func (budget Budget) Fit(request *ModelRequest) error {
	if budget.Context == 0 {
		return nil
	}
	if request.MaxOutputTokens == nil || *request.MaxOutputTokens > budget.OutputReserve {
		request.MaxOutputTokens = &budget.OutputReserve
	}
	for i := 1; EstimateRequest(*request) > budget.InputLimit; {
		for i < len(request.Messages) && request.Messages[i].Role != "user" {
			i++
		}
		if i == len(request.Messages) {
			return &ModelError{Code: "ContextWindowExceeded"}
		}
		request.Messages = request.Messages[i:]
		i = 1
	}
	return nil
}

// FitForModel checks the exact assembled input on providers with a token-count endpoint.
// Other providers retain the local estimate and use the same whole-turn trimming.
func FitForModel(ctx context.Context, model Model, request *ModelRequest) error {
	budget := request.Budget
	if err := budget.Fit(request); err != nil {
		return err
	}
	counter, ok := model.(interface {
		CountInput(context.Context, *ModelRequest) (int, error)
	})
	if budget.Context == 0 || !ok || EstimateRequest(*request) < budget.InputLimit*3/4 {
		return ctx.Err()
	}
	measured, err := counter.CountInput(ctx, request)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || measured <= budget.InputLimit {
		return nil
	}
	estimate := EstimateRequest(*request)
	if estimate == 0 {
		return &ModelError{Code: "ContextWindowExceeded"}
	}
	budget.InputLimit = int(float64(budget.InputLimit) * float64(estimate) / float64(measured))
	if err := budget.Fit(request); err != nil {
		return err
	}
	verified, err := counter.CountInput(ctx, request)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err == nil && verified > request.Budget.InputLimit {
		return &ModelError{Code: "ContextWindowExceeded"}
	}
	return nil
}
