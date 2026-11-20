package ai

import "strings"

// SupportsReasoningTools checks whether the current tool runner can continue a turn with explicit effort.
func SupportsReasoningTools(adapter Model, model, effort string) bool {
	if effort == "" {
		return true
	}
	switch client := adapter.(type) {
	case *Messages:
		return false
	case *Responses:
		// Stateless continuation would need to replay the provider's reasoning items.
		return effort == "none"
	case *ChatCompletions:
		// Some providers reject this combination or require reasoning data on tool continuations.
		return effort == "none" || (client.host != "api.openai.com" && client.host != "openrouter.ai" &&
			client.host != "api.deepseek.com" && !strings.Contains(strings.ToLower(model), "deepseek"))
	default:
		return false
	}
}
