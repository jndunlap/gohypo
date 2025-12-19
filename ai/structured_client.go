package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gohypo/models"
	"io"
	"log"
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

// ResponseFormat forces structured output from GPT models
type ResponseFormat struct {
	Type string `json:"type"` // "json_object" for structured output
}

// NewStructuredClient creates a new structured client
func NewStructuredClient[T any](config *models.AIConfig, promptsDir string) *StructuredClient[T] {
	log.Printf("[StructuredClient] Initializing client with model=%s, temp=%.2f, maxTokens=%d, timeout=180s",
		config.OpenAIModel, config.Temperature, config.MaxTokens)

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
	log.Printf("[StructuredClient] Starting JSON response request - provider=%s, model=%s", provider, client.OpenAIClient.Model)

	if provider != "openai" {
		log.Printf("[StructuredClient] ERROR: Unsupported provider: %s", provider)
		return nil, fmt.Errorf("only openai provider supported")
	}

	// Create context with timeout
	timeout := time.Duration(client.OpenAIClient.Timeout) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	log.Printf("[StructuredClient] Request timeout set to %v", timeout)

	// Create the request body for OpenAI chat completions
	type Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type RequestBody struct {
		Model               string         `json:"model"`
		Messages            []Message      `json:"messages"`
		Temperature         float64        `json:"temperature,omitempty"`
		MaxCompletionTokens int            `json:"max_completion_tokens,omitempty"`
		ResponseFormat      ResponseFormat `json:"response_format,omitempty"`
	}

	// Use provided system message or fall back to default
	systemContent := systemMessage
	if systemContent == "" {
		systemContent = client.SystemContext
	}

	// Ensure "JSON" appears in system message for OpenAI JSON mode compatibility
	if strings.Contains(client.OpenAIClient.Model, "gpt-5") && !strings.Contains(strings.ToLower(systemContent), "json") {
		log.Printf("[StructuredClient] Adding JSON mode directive to system message for GPT-5 model")
		systemContent = systemContent + "\n\nIMPORTANT: Respond with valid JSON output."
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

	// Force JSON output for GPT-5.x+ models to prevent conversational drift
	if strings.Contains(client.OpenAIClient.Model, "gpt-5") {
		reqBody.ResponseFormat = ResponseFormat{Type: "json_object"}
		log.Printf("[StructuredClient] JSON response format enforced for GPT-5 model")
	}

	// Log the request details
	promptPreview := prompt
	if len(prompt) > 500 {
		promptPreview = prompt[:500] + "..."
	}
	log.Printf("[StructuredClient] Sending request to %s - promptLength=%d, temp=%.2f",
		client.OpenAIClient.Model, len(prompt), client.OpenAIClient.Temperature)
	log.Printf("[StructuredClient] Prompt preview: %s", promptPreview)

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
		log.Printf("[StructuredClient] ERROR: Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("[StructuredClient] Response body size: %d bytes", len(body))

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
		log.Printf("[StructuredClient] ERROR: Failed to parse OpenAI response envelope: %v", err)
		return nil, fmt.Errorf("failed to parse OpenAI response: %w\nRaw response: %s", err, string(body))
	}

	if len(openaiResp.Choices) == 0 {
		log.Printf("[StructuredClient] ERROR: No choices in OpenAI response")
		return nil, fmt.Errorf("no choices in OpenAI response\nRaw response: %s", string(body))
	}

	// Parse the JSON content into the typed result
	var result T
	content := openaiResp.Choices[0].Message.Content

	log.Printf("[StructuredClient] Raw content length: %d bytes", len(content))

	// Clean up the content (remove markdown code blocks if present)
	content = cleanJSONContent(content)

	log.Printf("[StructuredClient] Cleaned content length: %d bytes", len(content))

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("[StructuredClient] ERROR: Failed to unmarshal JSON content into result type: %v", err)
		log.Printf("[StructuredClient] Cleaned content: %s", content)
		return nil, fmt.Errorf("failed to parse JSON content into result type: %w\nRaw OpenAI response body: %s\nCleaned content: %s", err, string(body), content)
	}

	log.Printf("[StructuredClient] ✓ Successfully parsed JSON response into result type")
	return &result, nil
}

// cleanJSONContent removes markdown code blocks and cleans JSON content
func cleanJSONContent(content string) string {
	content = strings.TrimSpace(content)
	originalLength := len(content)

	// Remove markdown code blocks with various prefixes
	if strings.HasPrefix(content, "```json") && strings.HasSuffix(content, "```") {
		log.Printf("[StructuredClient] Removing ```json markdown wrapper")
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") && strings.HasSuffix(content, "```") {
		log.Printf("[StructuredClient] Removing ``` markdown wrapper")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	// Remove common AI chatter patterns that might precede JSON
	lines := strings.Split(content, "\n")
	cleanedLines := make([]string, 0, len(lines))
	skippedLines := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines, explanations, or common chatter
		if line == "" ||
			strings.HasPrefix(strings.ToLower(line), "here is") ||
			strings.HasPrefix(strings.ToLower(line), "the json") ||
			strings.HasPrefix(strings.ToLower(line), "output:") ||
			strings.HasPrefix(strings.ToLower(line), "response:") ||
			strings.HasPrefix(strings.ToLower(line), "##") ||
			strings.Contains(strings.ToLower(line), "below is") ||
			strings.Contains(strings.ToLower(line), "following is") {
			skippedLines++
			continue
		}
		cleanedLines = append(cleanedLines, line)
	}

	if skippedLines > 0 {
		log.Printf("[StructuredClient] Filtered out %d lines of AI chatter", skippedLines)
	}

	content = strings.Join(cleanedLines, "\n")
	content = strings.TrimSpace(content)

	// If content starts with a line that looks like chatter, remove it
	if strings.Contains(content, "\n{") {
		parts := strings.SplitN(content, "\n{", 2)
		if len(parts) == 2 && !strings.Contains(parts[0], "{") && !strings.Contains(parts[0], "[") {
			log.Printf("[StructuredClient] Trimming prefix chatter before JSON object")
			content = "{" + parts[1]
		}
	} else if strings.Contains(content, "\n[") {
		parts := strings.SplitN(content, "\n[", 2)
		if len(parts) == 2 && !strings.Contains(parts[0], "{") && !strings.Contains(parts[0], "[") {
			log.Printf("[StructuredClient] Trimming prefix chatter before JSON array")
			content = "[" + parts[1]
		}
	}

	finalLength := len(content)
	if originalLength != finalLength {
		log.Printf("[StructuredClient] Content cleaning reduced size: %d -> %d bytes", originalLength, finalLength)
	}

	return content
}

// GetJsonResponseFromPrompt loads external prompt and gets structured response
func (client *StructuredClient[T]) GetJsonResponseFromPrompt(promptName string, replacements map[string]string) (*T, error) {
	return client.GetJsonResponseFromPromptWithContext(context.Background(), promptName, replacements)
}

// GetJsonResponseFromPromptWithContext loads external prompt and gets structured response with context
func (client *StructuredClient[T]) GetJsonResponseFromPromptWithContext(ctx context.Context, promptName string, replacements map[string]string) (*T, error) {
	log.Printf("[StructuredClient] Loading prompt template: %s", promptName)
	log.Printf("[StructuredClient] Replacements count: %d", len(replacements))

	// Load and render external prompt
	prompt, err := client.PromptManager.RenderPrompt(promptName, replacements)
	if err != nil {
		log.Printf("[StructuredClient] ERROR: Failed to load/render prompt %s: %v", promptName, err)
		return nil, fmt.Errorf("failed to load/render prompt: %w", err)
	}

	log.Printf("[StructuredClient] Rendered prompt length: %d characters", len(prompt))

	// Use OpenAI provider with context, but don't add system message since prompt contains it
	return client.GetJsonResponseWithContext(ctx, "openai", prompt, "")
}
