package models

import (
	"time"

	"github.com/google/uuid"
)

// LLMUsage represents a single LLM API call's token usage
type LLMUsage struct {
	ID               uuid.UUID  `json:"id" db:"id"`
	UserID           uuid.UUID  `json:"user_id" db:"user_id"`
	SessionID        *uuid.UUID `json:"session_id,omitempty" db:"session_id"`
	Provider         string     `json:"provider" db:"provider"`             // 'openai', 'anthropic', etc.
	Model            string     `json:"model" db:"model"`                   // 'gpt-5.2', 'gpt-5.2', etc.
	OperationType    string     `json:"operation_type" db:"operation_type"` // 'hypothesis_generation', 'dataset_analysis', etc.
	PromptTokens     int        `json:"prompt_tokens" db:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens" db:"completion_tokens"`
	TotalTokens      int        `json:"total_tokens" db:"total_tokens"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
}

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

// UserUsageSummary provides aggregated usage statistics for a user
type UserUsageSummary struct {
	UserID                uuid.UUID                `json:"user_id"`
	PeriodStart           time.Time                `json:"period_start"`
	PeriodEnd             time.Time                `json:"period_end"`
	TotalTokens           int                      `json:"total_tokens"`
	TotalPromptTokens     int                      `json:"total_prompt_tokens"`
	TotalCompletionTokens int                      `json:"total_completion_tokens"`
	ByProvider            map[string]ProviderUsage `json:"by_provider"`
	ByModel               map[string]ModelUsage    `json:"by_model"`
	RequestCount          int                      `json:"request_count"`
}

// ProviderUsage represents usage aggregated by provider
type ProviderUsage struct {
	Provider     string `json:"provider"`
	TotalTokens  int    `json:"total_tokens"`
	RequestCount int    `json:"request_count"`
}

// ModelUsage represents usage aggregated by model
type ModelUsage struct {
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	TotalTokens  int    `json:"total_tokens"`
	RequestCount int    `json:"request_count"`
}

// Operation types for categorization
const (
	OpHypothesisGeneration  = "hypothesis_generation"
	OpDatasetDomainAnalysis = "dataset_domain_analysis"
	OpSchemaCompatibility   = "schema_compatibility"
	OpSemanticSimilarity    = "semantic_similarity"
	OpKeyRelationships      = "key_relationships"
	OpTemporalPatterns      = "temporal_patterns"
	OpDatasetProfiling      = "dataset_profiling"
	OpMergeReasoning        = "merge_reasoning"
)
