package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"gohypo/adapters/excel"
	"gohypo/domain/dataset"
	"gohypo/models"
)

// ForensicScout extracts industry context from raw data sample
type ForensicScout struct {
	client         *StructuredClient[ScoutResponse]
	contextCache   map[string]*ScoutResponse        // Cache structured industry context per Excel file
	lowTokenClient *StructuredClient[ScoutResponse] // Client with low token limits for simple responses

	// Specialized clients for different analysis types
	schemaClient   *StructuredClient[dataset.SchemaCompatibilityResult]
	semanticClient *StructuredClient[dataset.SemanticSimilarityResult]
	keyClient      *StructuredClient[dataset.KeyRelationshipResult]
	temporalClient *StructuredClient[dataset.TemporalPatternResult]
	profileClient  *StructuredClient[dataset.DatasetProfileResult]
	mergeClient    *StructuredClient[dataset.MergeReasoningResult]
}

// ScoutResponse is the LLM response for domain identification
type ScoutResponse struct {
	Domain      string `json:"domain" description:"Business/industry domain"`
	DatasetName string `json:"dataset_name" description:"Descriptive dataset name"`
	Filename    string `json:"filename" description:"Clean filename in snake_case with extension"`
	Description string `json:"description" description:"Detailed 2-3 sentence description"`
}

// NewForensicScout creates a new Forensic Scout
func NewForensicScout(config *models.AIConfig) *ForensicScout {
	// Use improved mock client that provides intelligent responses
	mockClient := &mockLLMClient{}

	// Create a low-token config for simple domain identification
	lowTokenConfig := *config      // copy config
	lowTokenConfig.MaxTokens = 100 // Very low token limit for simple JSON response

	return &ForensicScout{
		client:         NewStructuredClient[ScoutResponse](mockClient, nil, config.PromptsDir, config.SystemContext),
		lowTokenClient: NewStructuredClient[ScoutResponse](mockClient, nil, config.PromptsDir, config.SystemContext),
		contextCache:   make(map[string]*ScoutResponse),

		// Initialize specialized analysis clients
		schemaClient:   NewStructuredClient[dataset.SchemaCompatibilityResult](mockClient, nil, config.PromptsDir, config.SystemContext),
		semanticClient: NewStructuredClient[dataset.SemanticSimilarityResult](mockClient, nil, config.PromptsDir, config.SystemContext),
		keyClient:      NewStructuredClient[dataset.KeyRelationshipResult](mockClient, nil, config.PromptsDir, config.SystemContext),
		temporalClient: NewStructuredClient[dataset.TemporalPatternResult](mockClient, nil, config.PromptsDir, config.SystemContext),
		profileClient:  NewStructuredClient[dataset.DatasetProfileResult](mockClient, nil, config.PromptsDir, config.SystemContext),
		mergeClient:    NewStructuredClient[dataset.MergeReasoningResult](mockClient, nil, config.PromptsDir, config.SystemContext),
	}
}

// ExtractIndustryContext performs the "Scout" injection:
// 1. Checks for EXCEL_FILE environment variable (legacy support)
// 2. If no env var set, returns nil (expected for uploaded datasets)
// 3. If env var set, extracts headers from Excel file and calls LLM for analysis
// 4. Returns structured industry intelligence for prompt injection and UI display
//
// This method supports both legacy hardcoded Excel files and modern uploaded datasets.
func (fs *ForensicScout) ExtractIndustryContext(ctx context.Context) (*ScoutResponse, error) {
	// Get Excel file path from environment (legacy support)
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		// No hardcoded file configured - return nil to indicate no industry context available
		// This is the expected behavior for uploaded datasets
		return nil, nil
	}

	// Check if we already have cached context for this file
	if cached, exists := fs.contextCache[excelFile]; exists {
		return cached, nil
	}

	// Read Excel data
	reader := excel.NewDataReader(excelFile)
	data, err := reader.ReadData()
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to read Excel data: %v", err)
		return nil, fmt.Errorf("failed to read Excel data for scout: %w", err)
	}

	// Build sample data structure with only headers
	sampleData := map[string]interface{}{
		"headers": data.Headers,
	}

	// Convert to JSON for prompt
	sampleJSON, err := json.MarshalIndent(sampleData, "", "  ")
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to marshal sample data: %v", err)
		return nil, fmt.Errorf("failed to marshal sample data: %w", err)
	}

	// Load scout prompt from external file
	replacements := map[string]string{
		"DATA_SAMPLE": string(sampleJSON),
	}

	scoutPrompt, err := fs.client.PromptManager.RenderPrompt("forensic_scout", replacements)
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to load/render scout prompt: %v", err)
		return nil, fmt.Errorf("failed to load/render scout prompt: %w", err)
	}

	// Call LLM with low token limits (simple domain identification)
	response, err := fs.lowTokenClient.GetJsonResponseWithContext(ctx, "openai", scoutPrompt, "You are a data domain expert specializing in identifying business context from raw data samples. Respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Scout LLM call failed: %v", err)
		return nil, fmt.Errorf("scout LLM call failed: %w", err)
	}

	// Cache the result for future use
	fs.contextCache[excelFile] = response

	return response, nil
}

// AnalyzeFields performs forensic analysis on a list of field names directly
// This allows analysis of uploaded datasets without requiring Excel files
func (fs *ForensicScout) AnalyzeFields(ctx context.Context, fieldNames []string) (*ScoutResponse, error) {
	// Create cache key from sorted field names
	sortedNames := make([]string, len(fieldNames))
	copy(sortedNames, fieldNames)
	// Simple sort for consistent cache key
	for i := 0; i < len(sortedNames)-1; i++ {
		for j := i + 1; j < len(sortedNames); j++ {
			if sortedNames[i] > sortedNames[j] {
				sortedNames[i], sortedNames[j] = sortedNames[j], sortedNames[i]
			}
		}
	}
	cacheKey := "fields:" + fmt.Sprintf("%v", sortedNames)

	// Check cache first
	if cached, exists := fs.contextCache[cacheKey]; exists {
		return cached, nil
	}

	// Create sample data structure with field names
	sampleData := map[string]interface{}{
		"headers": fieldNames,
	}

	// Convert to JSON for prompt
	sampleJSON, err := json.MarshalIndent(sampleData, "", "  ")
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to marshal field data: %v", err)
		return nil, fmt.Errorf("failed to marshal field data: %w", err)
	}

	// Load scout prompt from external file
	replacements := map[string]string{
		"DATA_SAMPLE": string(sampleJSON),
	}

	scoutPrompt, err := fs.client.PromptManager.RenderPrompt("forensic_scout", replacements)
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Failed to load/render scout prompt: %v", err)
		return nil, fmt.Errorf("failed to load/render scout prompt: %w", err)
	}

	// Call LLM with low token limits for domain identification
	response, err := fs.lowTokenClient.GetJsonResponseWithContext(
		ctx,
		"openai",
		scoutPrompt,
		"You are a data domain expert specializing in identifying business context from raw data samples. Respond with valid JSON.",
	)
	if err != nil {
		log.Printf("[ForensicScout] ✗ ERROR: Field analysis LLM call failed: %v", err)
		return nil, fmt.Errorf("field analysis LLM call failed: %w", err)
	}

	log.Printf("[ForensicScout] ✓ Successfully analyzed %d fields - Domain: %s, Dataset: %s",
		len(fieldNames), response.Domain, response.DatasetName)

	// Cache the result for future use
	fs.contextCache[cacheKey] = response

	return response, nil
}

// GetFallbackResponse provides sensible defaults when AI analysis fails
func (fs *ForensicScout) GetFallbackResponse(fieldNames []string) *ScoutResponse {
	// Simple heuristics for fallback naming
	fieldCount := len(fieldNames)

	// Check for common patterns
	hasID := containsSubstring(fieldNames, "id")
	hasName := containsSubstring(fieldNames, "name")
	hasDate := containsSubstring(fieldNames, "date") || containsSubstring(fieldNames, "time")
	hasAmount := containsSubstring(fieldNames, "amount") || containsSubstring(fieldNames, "price") || containsSubstring(fieldNames, "cost")

	if hasID && hasName && hasDate {
		if hasAmount {
			return &ScoutResponse{
				Domain:      "Business Analytics",
				DatasetName: "transaction_records",
			}
		}
		return &ScoutResponse{
			Domain:      "Data Management",
			DatasetName: "entity_records",
		}
	}

	// Generic fallback based on field count
	switch {
	case fieldCount < 5:
		return &ScoutResponse{
			Domain:      "Data Analysis",
			DatasetName: "small_dataset",
		}
	case fieldCount < 20:
		return &ScoutResponse{
			Domain:      "Data Analysis",
			DatasetName: "analysis_dataset",
		}
	default:
		return &ScoutResponse{
			Domain:      "Data Analysis",
			DatasetName: "large_dataset",
		}
	}
}

// AnalyzeSchemaCompatibility analyzes compatibility between two field sets
func (fs *ForensicScout) AnalyzeSchemaCompatibility(ctx context.Context, fields1, fields2 []string) (*dataset.SchemaCompatibilityResult, error) {
	prompt, err := fs.loadPrompt("schema_compatibility.txt")
	if err != nil {
		log.Printf("[ForensicScout] Failed to load schema compatibility prompt: %v", err)
		return fs.getFallbackSchemaCompatibility(fields1, fields2), nil
	}

	// Format the prompt with field sets
	formattedPrompt := strings.ReplaceAll(prompt, "{field_set_1}", formatFieldList(fields1))
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{field_set_2}", formatFieldList(fields2))

	result, err := fs.schemaClient.GetJsonResponseWithContext(ctx, "openai", formattedPrompt, "You are an expert data analyst. Analyze the provided field sets and respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] Schema compatibility analysis failed: %v", err)
		return fs.getFallbackSchemaCompatibility(fields1, fields2), nil
	}

	return result, nil
}

// AnalyzeSemanticSimilarity analyzes semantic relationships between datasets
func (fs *ForensicScout) AnalyzeSemanticSimilarity(ctx context.Context, fields1, fields2 []string) (*dataset.SemanticSimilarityResult, error) {
	prompt, err := fs.loadPrompt("semantic_similarity.txt")
	if err != nil {
		log.Printf("[ForensicScout] Failed to load semantic similarity prompt: %v", err)
		return fs.getFallbackSemanticSimilarity(fields1, fields2), nil
	}

	// Format the prompt with field sets
	formattedPrompt := strings.ReplaceAll(prompt, "{field_set_1}", formatFieldList(fields1))
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{field_set_2}", formatFieldList(fields2))

	result, err := fs.semanticClient.GetJsonResponseWithContext(ctx, "openai", formattedPrompt, "You are an expert data scientist. Analyze semantic relationships and respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] Semantic similarity analysis failed: %v", err)
		return fs.getFallbackSemanticSimilarity(fields1, fields2), nil
	}

	return result, nil
}

// AnalyzeKeyRelationships looks for potential foreign key relationships
func (fs *ForensicScout) AnalyzeKeyRelationships(ctx context.Context, fields1, fields2 []string) (*dataset.KeyRelationshipResult, error) {
	prompt, err := fs.loadPrompt("key_relationships.txt")
	if err != nil {
		log.Printf("[ForensicScout] Failed to load key relationships prompt: %v", err)
		return fs.getFallbackKeyRelationships(fields1, fields2), nil
	}

	// Format the prompt with field sets
	formattedPrompt := strings.ReplaceAll(prompt, "{field_set_1}", formatFieldList(fields1))
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{field_set_2}", formatFieldList(fields2))

	result, err := fs.keyClient.GetJsonResponseWithContext(ctx, "openai", formattedPrompt, "You are a database architect. Analyze key relationships and respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] Key relationships analysis failed: %v", err)
		return fs.getFallbackKeyRelationships(fields1, fields2), nil
	}

	return result, nil
}

// AnalyzeTemporalPatterns identifies time-based relationships
func (fs *ForensicScout) AnalyzeTemporalPatterns(ctx context.Context, fields1, fields2 []string) (*dataset.TemporalPatternResult, error) {
	prompt, err := fs.loadPrompt("temporal_patterns.txt")
	if err != nil {
		log.Printf("[ForensicScout] Failed to load temporal patterns prompt: %v", err)
		return fs.getFallbackTemporalPatterns(fields1, fields2), nil
	}

	// Format the prompt with field sets
	formattedPrompt := strings.ReplaceAll(prompt, "{field_set_1}", formatFieldList(fields1))
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{field_set_2}", formatFieldList(fields2))

	result, err := fs.temporalClient.GetJsonResponseWithContext(ctx, "openai", formattedPrompt, "You are a time-series data analyst. Analyze temporal patterns and respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] Temporal patterns analysis failed: %v", err)
		return fs.getFallbackTemporalPatterns(fields1, fields2), nil
	}

	return result, nil
}

// AnalyzeDatasetProfile provides comprehensive dataset profiling
func (fs *ForensicScout) AnalyzeDatasetProfile(ctx context.Context, fieldNames []string) (*dataset.DatasetProfileResult, error) {
	prompt, err := fs.loadPrompt("dataset_profiling.txt")
	if err != nil {
		log.Printf("[ForensicScout] Failed to load dataset profiling prompt: %v", err)
		return fs.getFallbackDatasetProfile(fieldNames), nil
	}

	// Format the prompt with field names
	formattedPrompt := strings.ReplaceAll(prompt, "{field_names}", formatFieldList(fieldNames))
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{sample_values}", "Not available") // Could be enhanced later

	result, err := fs.profileClient.GetJsonResponseWithContext(ctx, "openai", formattedPrompt, "You are a data quality analyst. Profile the dataset and respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] Dataset profiling failed: %v", err)
		return fs.getFallbackDatasetProfile(fieldNames), nil
	}

	return result, nil
}

// AnalyzeMergeReasoning provides intelligent merge strategy recommendations
func (fs *ForensicScout) AnalyzeMergeReasoning(ctx context.Context, datasetSummaries, analyses []string) (*dataset.MergeReasoningResult, error) {
	prompt, err := fs.loadPrompt("merge_reasoning.txt")
	if err != nil {
		log.Printf("[ForensicScout] Failed to load merge reasoning prompt: %v", err)
		return fs.getFallbackMergeReasoning(datasetSummaries), nil
	}

	// Format the prompt with analysis data
	formattedPrompt := strings.ReplaceAll(prompt, "{dataset_summaries}", strings.Join(datasetSummaries, "\n"))
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{schema_analysis}", analyses[0])
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{semantic_analysis}", analyses[1])
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{key_relationships}", analyses[2])
	formattedPrompt = strings.ReplaceAll(formattedPrompt, "{temporal_patterns}", analyses[3])

	result, err := fs.mergeClient.GetJsonResponseWithContext(ctx, "openai", formattedPrompt, "You are a data integration specialist. Recommend merge strategies and respond with valid JSON.")
	if err != nil {
		log.Printf("[ForensicScout] Merge reasoning analysis failed: %v", err)
		return fs.getFallbackMergeReasoning(datasetSummaries), nil
	}

	return result, nil
}

// loadPrompt loads a prompt file from the prompts directory
func (fs *ForensicScout) loadPrompt(filename string) (string, error) {
	content, err := os.ReadFile(fmt.Sprintf("prompts/%s", filename))
	if err != nil {
		return "", fmt.Errorf("failed to read prompt file %s: %w", filename, err)
	}
	return string(content), nil
}

// formatFieldList formats a slice of field names for prompt inclusion
func formatFieldList(fields []string) string {
	if len(fields) == 0 {
		return "No fields available"
	}
	return "- " + strings.Join(fields, "\n- ")
}

// Fallback methods for when AI analysis fails

func (fs *ForensicScout) getFallbackSchemaCompatibility(fields1, fields2 []string) *dataset.SchemaCompatibilityResult {
	commonFields := findCommonFields(fields1, fields2)
	compatibility := float64(len(commonFields)*2) / float64(len(fields1)+len(fields2))

	return &dataset.SchemaCompatibilityResult{
		CompatibilityScore: compatibility,
		CommonFields:       commonFields,
		RelationshipType:   "UNKNOWN",
		MergeStrategy:      "UNION",
		Issues:             []string{"AI analysis unavailable"},
		Confidence:         0.5,
		AnalysisDetails:    "Fallback analysis based on field name matching",
	}
}

func (fs *ForensicScout) getFallbackSemanticSimilarity(fields1, fields2 []string) *dataset.SemanticSimilarityResult {
	return &dataset.SemanticSimilarityResult{
		Dataset1Domain:     "Unknown",
		Dataset2Domain:     "Unknown",
		SemanticSimilarity: 0.5,
		RelationshipType:   "UNKNOWN",
		Confidence:         0.3,
		AnalysisDetails:    "AI analysis unavailable, using basic similarity",
	}
}

func (fs *ForensicScout) getFallbackKeyRelationships(fields1, fields2 []string) *dataset.KeyRelationshipResult {
	return &dataset.KeyRelationshipResult{
		RelationshipType:     "UNKNOWN",
		RelationshipStrength: 0.3,
		Confidence:           0.3,
		AnalysisDetails:      "AI analysis unavailable",
	}
}

func (fs *ForensicScout) getFallbackTemporalPatterns(fields1, fields2 []string) *dataset.TemporalPatternResult {
	return &dataset.TemporalPatternResult{
		TemporalRelationship: "UNKNOWN",
		TemporalStrength:     0.3,
		Confidence:           0.3,
		AnalysisDetails:      "AI analysis unavailable",
	}
}

func (fs *ForensicScout) getFallbackDatasetProfile(fieldNames []string) *dataset.DatasetProfileResult {
	return &dataset.DatasetProfileResult{
		DomainClassification: map[string]interface{}{
			"primary_domain": "Unknown",
		},
		ProfilingConfidence: 0.3,
		AnalysisDetails:     "AI analysis unavailable",
	}
}

func (fs *ForensicScout) getFallbackMergeReasoning(datasetSummaries []string) *dataset.MergeReasoningResult {
	return &dataset.MergeReasoningResult{
		IntegrationStrategy:      "SEPARATE_MAINTENANCE",
		SuccessConfidence:        0.3,
		ImplementationComplexity: 5,
		AnalysisDetails:          "AI analysis unavailable, recommend separate maintenance",
	}
}

// Helper functions

func findCommonFields(fields1, fields2 []string) []string {
	common := make([]string, 0)
	fieldMap := make(map[string]bool)

	for _, field := range fields1 {
		fieldMap[strings.ToLower(field)] = true
	}

	for _, field := range fields2 {
		if fieldMap[strings.ToLower(field)] {
			common = append(common, field)
		}
	}

	return common
}

// containsSubstring checks if any field name contains the given substring (case insensitive)
func containsSubstring(fieldNames []string, substr string) bool {
	for _, name := range fieldNames {
		if strings.Contains(strings.ToLower(name), strings.ToLower(substr)) {
			return true
		}
	}
	return false
}
