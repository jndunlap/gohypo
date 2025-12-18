package models

import (
	"os"
	"strconv"
)

// AIConfig holds AI service configuration for integration with dunlap/ai package
type AIConfig struct {
	OpenAIKey     string
	OpenAIModel   string
	SystemContext string
	MaxTokens     int
	Temperature   float64
	PromptsDir    string // Directory for external prompt files
}

// DefaultAIConfig returns sensible defaults for AI configuration
func DefaultAIConfig() *AIConfig {
	config := &AIConfig{
		OpenAIKey:     "",
		OpenAIModel:   os.Getenv("LLM_MODEL"),
		SystemContext: "You are a statistical research assistant",
		MaxTokens:     2000, // default
		Temperature:   0.1,  // default
		PromptsDir:    "./prompts",
	}

	// Parse MaxTokens from environment
	if maxTokensStr := os.Getenv("LLM_MAX_TOKENS"); maxTokensStr != "" {
		if maxTokens, err := strconv.Atoi(maxTokensStr); err == nil {
			config.MaxTokens = maxTokens
		}
	}

	// Parse Temperature from environment
	if tempStr := os.Getenv("LLM_TEMPERATURE"); tempStr != "" {
		if temp, err := strconv.ParseFloat(tempStr, 64); err == nil {
			config.Temperature = temp
		}
	}

	return config
}
