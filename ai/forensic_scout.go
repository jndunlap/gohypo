package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gohypo/adapters/excel"
	"gohypo/models"
)

// ForensicScout extracts industry context from raw data sample
type ForensicScout struct {
	client *StructuredClient[ScoutResponse]
}

// ScoutResponse is the LLM response for industry context
type ScoutResponse struct {
	IndustryContext string `json:"industry_context" description:"Two-sentence semantic summary of industry domain and primary friction points"`
}

// NewForensicScout creates a new Forensic Scout
func NewForensicScout(config *models.AIConfig) *ForensicScout {
	return &ForensicScout{
		client: NewStructuredClient[ScoutResponse](config, config.PromptsDir),
	}
}

// ExtractIndustryContext performs the "Scout" injection:
// 1. Extracts headers + top 10 rows from Excel file
// 2. Calls LLM to identify industry context and semantic physics
// 3. Returns 2-sentence summary for prompt injection
func (fs *ForensicScout) ExtractIndustryContext(ctx context.Context) (string, error) {
	// Get Excel file path from environment
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		fmt.Printf("[ForensicScout] Warning: EXCEL_FILE not set, skipping industry context extraction\n")
		return "", nil
	}

	fmt.Printf("[ForensicScout] Reading Excel file: %s\n", excelFile)

	// Read Excel data
	reader := excel.NewExcelReader(excelFile)
	data, err := reader.ReadData()
	if err != nil {
		return "", fmt.Errorf("failed to read Excel data for scout: %w", err)
	}

	fmt.Printf("[ForensicScout] Read %d headers and %d rows from Excel file\n", len(data.Headers), len(data.Rows))

	// Extract headers + top 10 rows
	sampleRows := 10
	if len(data.Rows) < sampleRows {
		sampleRows = len(data.Rows)
	}

	// Build sample data structure
	sampleData := map[string]interface{}{
		"headers":     data.Headers,
		"sample_rows": make([]map[string]string, sampleRows),
	}

	for i := 0; i < sampleRows; i++ {
		sampleData["sample_rows"].([]map[string]string)[i] = data.Rows[i]
	}

	// Convert to JSON for prompt
	sampleJSON, err := json.MarshalIndent(sampleData, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal sample data: %w", err)
	}

	// Load scout prompt from external file
	replacements := map[string]string{
		"DATA_SAMPLE": string(sampleJSON),
	}

	scoutPrompt, err := fs.client.PromptManager.RenderPrompt("forensic_scout", replacements)
	if err != nil {
		return "", fmt.Errorf("failed to load/render scout prompt: %w", err)
	}

	// Call LLM
	fmt.Printf("[ForensicScout] Calling LLM to extract industry context...\n")
	response, err := fs.client.GetJsonResponseWithContext(ctx, "openai", scoutPrompt, "You are a data domain expert specializing in identifying business context from raw data samples.")
	if err != nil {
		return "", fmt.Errorf("scout LLM call failed: %w", err)
	}

	industryContext := strings.TrimSpace(response.IndustryContext)
	fmt.Printf("[ForensicScout] Extracted industry context (%d chars): %s\n", len(industryContext), industryContext)
	return industryContext, nil
}
