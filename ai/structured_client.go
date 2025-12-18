package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gohypo/models"
	"io"
	"net/http"
	"strings"
	"time"
)

// StructuredClient provides typed JSON responses from LLM calls
type StructuredClient[T any] struct {
	OpenAIClient  *OpenAIClient
	PromptManager *PromptManager
	SystemContext string
}

// OpenAIClient represents the OpenAI client interface
type OpenAIClient struct {
	APIKey      string
	BaseURL     string
	Timeout     int // in milliseconds
	Temperature float64
	MaxTokens   int
	Model       string
}

// NewStructuredClient creates a new structured client
func NewStructuredClient[T any](config *models.AIConfig, promptsDir string) *StructuredClient[T] {
	return &StructuredClient[T]{
		OpenAIClient: &OpenAIClient{
			APIKey:      config.OpenAIKey,
			BaseURL:     "https://api.openai.com/v1",
			Timeout:     180000, // 180 seconds for reasoning models
			Temperature: config.Temperature,
			MaxTokens:   config.MaxTokens,
			Model:       config.OpenAIModel,
		},
		PromptManager: NewPromptManager(promptsDir),
		SystemContext: config.SystemContext,
	}
}

// GetJsonResponse makes a typed LLM call and parses JSON response
func (client *StructuredClient[T]) GetJsonResponse(provider, prompt string) (*T, error) {
	return client.GetJsonResponseWithContext(context.Background(), provider, prompt, "")
}

// GetJsonResponseWithContext makes a typed LLM call with context support
func (client *StructuredClient[T]) GetJsonResponseWithContext(ctx context.Context, provider, prompt string, systemMessage string) (*T, error) {
	if provider != "openai" {
		return nil, fmt.Errorf("only openai provider supported")
	}

	// Create context with timeout
	timeout := time.Duration(client.OpenAIClient.Timeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create the request body for OpenAI chat completions
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type RequestBody struct {
		Model               string    `json:"model"`
		Messages            []Message `json:"messages"`
		Temperature         float64   `json:"temperature,omitempty"`
		MaxCompletionTokens int       `json:"max_completion_tokens,omitempty"`
	}

	// Use provided system message or fall back to default
	systemContent := systemMessage
	if systemContent == "" {
		systemContent = client.SystemContext
	}

	reqBody := RequestBody{
		Model: client.OpenAIClient.Model,
		Messages: []Message{
			{Role: "system", Content: systemContent},
			{Role: "user", Content: prompt},
		},
		Temperature:         client.OpenAIClient.Temperature,
		MaxCompletionTokens: client.OpenAIClient.MaxTokens,
	}

	// Debug: Log the request being sent
	if len(prompt) > 500 {
		fmt.Printf("[DEBUG] Sending request to %s with prompt (truncated): %s...\n", client.OpenAIClient.Model, prompt[:500])
	} else {
		fmt.Printf("[DEBUG] Sending request to %s with prompt: %s\n", client.OpenAIClient.Model, prompt)
	}

	// Make the HTTP request to OpenAI
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", client.OpenAIClient.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+client.OpenAIClient.APIKey)

	httpClient := &http.Client{Timeout: timeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("request timeout after %v: %w", timeout, err)
		}
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse the OpenAI response
	type OpenAIResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	var openaiResp OpenAIResponse
	if err := json.Unmarshal(body, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI response: %w\nRaw response: %s", err, string(body))
	}

	if len(openaiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response\nRaw response: %s", string(body))
	}

	// Parse the JSON content into the typed result
	var result T
	content := openaiResp.Choices[0].Message.Content

	// Clean up the content (remove markdown code blocks if present)
	content = cleanJSONContent(content)

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON content into result type: %w\nRaw OpenAI response body: %s\nCleaned content: %s", err, string(body), content)
	}

	return &result, nil
}

// cleanJSONContent removes markdown code blocks and cleans JSON content
func cleanJSONContent(content string) string {
	content = strings.TrimSpace(content)

	// Remove markdown code blocks
	if strings.HasPrefix(content, "```json") && strings.HasSuffix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") && strings.HasSuffix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	return content
}

// GetJsonResponseFromPrompt loads external prompt and gets structured response
func (client *StructuredClient[T]) GetJsonResponseFromPrompt(promptName string, replacements map[string]string) (*T, error) {
	return client.GetJsonResponseFromPromptWithContext(context.Background(), promptName, replacements)
}

// GetJsonResponseFromPromptWithContext loads external prompt and gets structured response with context
func (client *StructuredClient[T]) GetJsonResponseFromPromptWithContext(ctx context.Context, promptName string, replacements map[string]string) (*T, error) {
	// Load and render external prompt
	prompt, err := client.PromptManager.RenderPrompt(promptName, replacements)
	if err != nil {
		return nil, fmt.Errorf("failed to load/render prompt: %w", err)
	}

	// Use OpenAI provider with context, but don't add system message since prompt contains it
	return client.GetJsonResponseWithContext(ctx, "openai", prompt, "")
}
