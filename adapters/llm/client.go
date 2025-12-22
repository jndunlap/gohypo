package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gohypo/ports"
)

// newLLMClient creates an LLM client based on config
func newLLMClient(config Config) (ports.LLMClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("missing OpenAI API key")
	}

	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIClient{
		APIKey:      config.APIKey,
		BaseURL:     baseURL,
		Timeout:     config.Timeout,
		Temperature: config.Temperature,
	}, nil
}

// NewClient creates a new LLM client with the given configuration
func NewClient(config Config) (ports.LLMClient, error) {
	return newLLMClient(config)
}

// MockLLMClient is a mock LLM client for testing
type MockLLMClient struct {
	Response string // Set this for testing
	Error    error  // Set this to simulate errors
}

func (m *MockLLMClient) ChatCompletion(ctx context.Context, model string, prompt string, maxTokens int) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	if m.Response != "" {
		return m.Response, nil
	}
	// Default mock response
	return `[
		{
			"cause_key": "inspection_count",
			"effect_key": "severity_score",
			"mechanism_category": "direct_causal",
			"confounder_keys": ["facility_size", "regulatory_region"],
			"rationale": "Increased inspection frequency likely improves compliance through deterrence effects and early violation detection.",
			"suggested_rigor": "standard",
			"supporting_artifacts": ["relationship:pearson:family_001:inspection_count:severity_score"]
		}
	]`, nil
}

func (m *MockLLMClient) ChatCompletionWithUsage(ctx context.Context, model string, prompt string, maxTokens int) (*ports.LLMResponse, error) {
	content, err := m.ChatCompletion(ctx, model, prompt, maxTokens)
	if err != nil {
		return nil, err
	}

	// Mock usage data
	return &ports.LLMResponse{
		Content: content,
		Usage: &ports.UsageData{
			PromptTokens:     50,
			CompletionTokens: 150,
			TotalTokens:      200,
			Model:            model,
			Provider:         "mock",
		},
	}, nil
}

// OpenAIClient implements LLMClient for OpenAI
type OpenAIClient struct {
	APIKey      string
	BaseURL     string
	Timeout     time.Duration
	Temperature float64
}

func (c *OpenAIClient) ChatCompletion(ctx context.Context, model string, prompt string, maxTokens int) (string, error) {
	if strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("missing model")
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	// Chat Completions API (kept minimal: one system + one user message)
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model       string  `json:"model"`
		Messages    []msg   `json:"messages"`
		Temperature float64 `json:"temperature,omitempty"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
	}
	body := reqBody{
		Model: model,
		Messages: []msg{
			{Role: "system", Content: "You are a careful assistant. Output exactly what the user asks for."},
			{Role: "user", Content: prompt},
		},
		Temperature: c.Temperature,
		MaxTokens:   maxTokens,
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	respRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai http %d: %s", resp.StatusCode, string(respRaw))
	}

	type choice struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	type usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	type respBody struct {
		Choices []choice `json:"choices"`
		Usage   usage    `json:"usage"`
		Model   string   `json:"model"`
	}
	var decoded respBody
	if err := json.Unmarshal(respRaw, &decoded); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("openai response missing choices")
	}
	return decoded.Choices[0].Message.Content, nil
}

func (c *OpenAIClient) ChatCompletionWithUsage(ctx context.Context, model string, prompt string, maxTokens int) (*ports.LLMResponse, error) {
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("missing model")
	}
	if maxTokens <= 0 {
		maxTokens = 1024
	}

	// Chat Completions API (kept minimal: one system + one user message)
	type msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type reqBody struct {
		Model          string             `json:"model"`
		Messages       []msg              `json:"messages"`
		Temperature    float64            `json:"temperature,omitempty"`
		MaxTokens      int                `json:"max_tokens,omitempty"`
		ResponseFormat *map[string]string `json:"response_format,omitempty"`
		StreamOptions  *map[string]bool   `json:"stream_options,omitempty"`
	}
	type choice struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	type usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	}
	type respBody struct {
		Choices []choice `json:"choices"`
		Usage   usage    `json:"usage"`
		Model   string   `json:"model"`
	}

	// Force JSON mode for newer models that support it
	var responseFormat *map[string]string
	if strings.Contains(model, "gpt-5.2") && !strings.Contains(model, "gpt-5.2") ||
		strings.Contains(model, "gpt-5.2") {
		responseFormat = &map[string]string{"type": "json_object"}
	}

	// Enable usage tracking in streaming responses
	streamOptions := &map[string]bool{"include_usage": true}

	reqBodyStruct := reqBody{
		Model: model,
		Messages: []msg{
			{Role: "system", Content: "You are a careful assistant. Output exactly what the user asks for."},
			{Role: "user", Content: prompt},
		},
		Temperature:    c.Temperature,
		MaxTokens:      maxTokens,
		ResponseFormat: responseFormat,
		StreamOptions:  streamOptions,
	}

	raw, err := json.Marshal(reqBodyStruct)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	client := &http.Client{Timeout: c.Timeout}
	url := strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}
	defer resp.Body.Close()

	respRaw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai http %d: %s", resp.StatusCode, string(respRaw))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respRaw, &decoded); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return nil, fmt.Errorf("openai response missing choices")
	}

	// Extract usage data
	usageData := &ports.UsageData{
		PromptTokens:     decoded.Usage.PromptTokens,
		CompletionTokens: decoded.Usage.CompletionTokens,
		TotalTokens:      decoded.Usage.TotalTokens,
		Model:            decoded.Model,
		Provider:         "openai",
	}

	return &ports.LLMResponse{
		Content: decoded.Choices[0].Message.Content,
		Usage:   usageData,
	}, nil
}
