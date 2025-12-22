package ports

import "context"

// UsageData represents raw usage data from LLM provider APIs
type UsageData struct {
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	Model            string `json:"model"`
	Provider         string `json:"provider"`
}

// LLMResponse represents an enhanced LLM response with usage data
type LLMResponse struct {
	Content string
	Usage   *UsageData
}

// LLMClient interface for LLM providers (enhanced with usage tracking)
type LLMClient interface {
	// Legacy method for backward compatibility
	ChatCompletion(ctx context.Context, model string, prompt string, maxTokens int) (string, error)

	// Enhanced method with usage tracking
	ChatCompletionWithUsage(ctx context.Context, model string, prompt string, maxTokens int) (*LLMResponse, error)
}
