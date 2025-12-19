package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"gohypo/adapters/excel"
	"gohypo/models"
)

// ForensicScout extracts industry context from raw data sample
type ForensicScout struct {
	client       *StructuredClient[ScoutResponse]
	contextCache map[string]*ScoutResponse // Cache structured industry context per Excel file
}

// ScoutResponse is the LLM response for structured industry intelligence
type ScoutResponse struct {
	Domain     string `json:"domain" description:"Industry sector name"`
	Context    string `json:"context" description:"Business process description"`
	Bottleneck string `json:"bottleneck" description:"Primary problem being solved"`
	Physics    string `json:"physics" description:"Key variable interactions and patterns"`
	Map        string `json:"map" description:"Data complexity assessment"`
}

// NewForensicScout creates a new Forensic Scout
func NewForensicScout(config *models.AIConfig) *ForensicScout {
	log.Printf("[ForensicScout] Initializing Forensic Scout with model=%s", config.OpenAIModel)

	return &ForensicScout{
		client:       NewStructuredClient[ScoutResponse](config, config.PromptsDir),
		contextCache: make(map[string]*ScoutResponse),
	}
}

// ExtractIndustryContext performs the "Scout" injection:
// 1. Extracts headers + top 10 rows from Excel file
// 2. Calls LLM to identify structured industry intelligence
// 3. Returns structured analysis for prompt injection and UI display
func (fs *ForensicScout) ExtractIndustryContext(ctx context.Context) (*ScoutResponse, error) {
	log.Printf("[ForensicScout] ═══ Starting Industry Context Extraction ═══")

	// Get Excel file path from environment
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		log.Printf("[ForensicScout] ⚠ EXCEL_FILE environment variable not set - skipping industry context extraction")
		return nil, nil
	}

	log.Printf("[ForensicScout] Target Excel file: %s", excelFile)

	// Check if we already have cached context for this file
	if cached, exists := fs.contextCache[excelFile]; exists {
		log.Printf("[ForensicScout] ✓ Using cached industry intelligence (cache hit)")
		log.Printf("[ForensicScout] Cached domain: %s", cached.Domain)
		return cached, nil
	}

	log.Printf("[ForensicScout] Cache miss - reading Excel file for fresh analysis...")

	// Read Excel data
	log.Printf("[ForensicScout] Creating Excel data reader...")
	reader := excel.NewDataReader(excelFile)

	log.Printf("[ForensicScout] Reading Excel data...")
	data, err := reader.ReadData()
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to read Excel data: %v", err)
		return nil, fmt.Errorf("failed to read Excel data for scout: %w", err)
	}

	log.Printf("[ForensicScout] ✓ Successfully read Excel data:")
	log.Printf("[ForensicScout]   - Headers: %d", len(data.Headers))
	log.Printf("[ForensicScout]   - Total rows: %d", len(data.Rows))

	// Extract headers + top 10 rows
	sampleRows := 10
	if len(data.Rows) < sampleRows {
		sampleRows = len(data.Rows)
		log.Printf("[ForensicScout] Adjusting sample size to available rows: %d", sampleRows)
	} else {
		log.Printf("[ForensicScout] Extracting top %d rows for analysis", sampleRows)
	}

	// Build sample data structure
	log.Printf("[ForensicScout] Building sample data structure...")
	sampleData := map[string]interface{}{
		"headers":     data.Headers,
		"sample_rows": make([]map[string]string, sampleRows),
	}

	for i := 0; i < sampleRows; i++ {
		sampleData["sample_rows"].([]map[string]string)[i] = data.Rows[i]
	}

	// Convert to JSON for prompt
	log.Printf("[ForensicScout] Marshaling sample data to JSON...")
	sampleJSON, err := json.MarshalIndent(sampleData, "", "  ")
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to marshal sample data: %v", err)
		return nil, fmt.Errorf("failed to marshal sample data: %w", err)
	}

	log.Printf("[ForensicScout] ✓ Sample JSON size: %d bytes", len(sampleJSON))

	// Load scout prompt from external file
	log.Printf("[ForensicScout] Loading forensic_scout prompt template...")
	replacements := map[string]string{
		"DATA_SAMPLE": string(sampleJSON),
	}

	scoutPrompt, err := fs.client.PromptManager.RenderPrompt("forensic_scout", replacements)
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to load/render scout prompt: %v", err)
		return nil, fmt.Errorf("failed to load/render scout prompt: %w", err)
	}

	log.Printf("[ForensicScout] ✓ Prompt rendered successfully (length: %d chars)", len(scoutPrompt))

	// Call LLM
	log.Printf("[ForensicScout] ══════════════════════════════════════")
	log.Printf("[ForensicScout] Calling LLM for industry intelligence extraction...")
	log.Printf("[ForensicScout] ══════════════════════════════════════")

	response, err := fs.client.GetJsonResponseWithContext(ctx, "openai", scoutPrompt, "You are a data domain expert specializing in identifying business context from raw data samples. Respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Scout LLM call failed: %v", err)
		return nil, fmt.Errorf("scout LLM call failed: %w", err)
	}

	log.Printf("[ForensicScout] ══════════════════════════════════════")
	log.Printf("[ForensicScout] ✓ Successfully extracted structured intelligence:")
	log.Printf("[ForensicScout]   - Domain: %s", response.Domain)
	log.Printf("[ForensicScout]   - Context: %s", response.Context)
	log.Printf("[ForensicScout]   - Bottleneck: %s", response.Bottleneck)
	log.Printf("[ForensicScout]   - Physics: %s", response.Physics)
	log.Printf("[ForensicScout]   - Map: %s", response.Map)
	log.Printf("[ForensicScout] ══════════════════════════════════════")

	// Cache the result for future use (but only cache concise responses)
	responseLength := len(response.Domain) + len(response.Context) + len(response.Bottleneck) + len(response.Physics) + len(response.Map)
	if responseLength < 1000 {
		log.Printf("[ForensicScout] Caching concise intelligence for future requests (total length: %d chars)", responseLength)
		fs.contextCache[excelFile] = response
	} else {
		log.Printf("[ForensicScout] ⚠ Response too verbose (%d chars) - not caching to force re-extraction with updated prompt", responseLength)
	}

	log.Printf("[ForensicScout] ═══ Industry Context Extraction Complete ═══")
	return response, nil
}
