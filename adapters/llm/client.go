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
)

// newLLMClient creates an LLM client based on config
func newLLMClient(config Config) (LLMClient, error) {
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
	type respBody struct {
		Choices []choice `json:"choices"`
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

