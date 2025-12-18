package ai

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"gohypo/domain/greenfield"
	"gohypo/models"

	"github.com/joho/godotenv"
)

// TestLiveGreenfieldResearch performs a live fire test with OpenAI
//
// This test implements the "20/10" Greenfield Research Flow philosophy:
// - High-Fidelity Handover: Metadata includes temporal latency, interaction variables, and semantic noise
// - Anti-Hallucination: Requires redundant validation strategies (at least 2 instruments)
// - Symmetry Requirement: Each hypothesis must define its null case
// - Engine Evolution: LLM must propose specific instruments, not generic statistical methods
// - Specificity Check: Validates that responses are mathematically precise (e.g., "36-hour decay constant")
//
// The test metadata is designed to force sophisticated analysis:
// - Temporal System: avg_days_between_events, last_event_timestamp (tests for decay patterns)
// - Interaction Variables: discount_redemption_count vs lifetime_value (tests for cannibalization)
// - Semantic Noise: app_opens vs actual_transactions (tests for proxy relationships)
//
// Expected output: Highly specific research directives with redundant validation strategies
func TestLiveGreenfieldResearch(t *testing.T) {
	// Load environment variables from .env file (relative to test file location)
	if err := godotenv.Load("../.env"); err != nil {
		// Try alternative path if running from different directory
		_ = godotenv.Load(".env")
	}

	// Skip if no API key is available
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping live test: OPENAI_API_KEY not set")
	}

	// Create AI config
	config := &models.AIConfig{
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:   os.Getenv("LLM_MODEL"),
		SystemContext: "You are a Lead Research Architect for the GoHypo Discovery Engine.",
		MaxTokens:     8000,            // Increased for reasoning models that need more tokens
		Temperature:   1.0,             // Default temperature for this model (only supports 1.0)
		PromptsDir:    getPromptsDir(), // Try multiple paths to find prompts directory
	}

	// Fallback to default model if not set
	if config.OpenAIModel == "" {
		config.OpenAIModel = "gpt-4"
	}

	// Create structured client
	client := NewStructuredClient[models.GreenfieldResearchOutput](config, config.PromptsDir)

	// High-fidelity metadata with temporal latency, interaction variables, and semantic noise
	// Designed to force the LLM to propose sophisticated statistical instruments
	fieldMetadata := []greenfield.FieldMetadata{
		// The Monetary System - with potential cannibalization
		{
			Name:         "lifetime_value",
			SemanticType: "numeric",
			DataType:     "float",
			Description:  "Total customer lifetime value in dollars",
		},
		{
			Name:         "discount_redemption_count",
			SemanticType: "numeric",
			DataType:     "int",
			Description:  "Number of discount codes redeemed (potential cannibalization with lifetime_value)",
		},
		{
			Name:         "avg_order_value",
			SemanticType: "numeric",
			DataType:     "float",
			Description:  "Average order value over customer lifetime",
		},

		// The Engagement System - with semantic noise/proxy relationships
		{
			Name:         "app_opens",
			SemanticType: "numeric",
			DataType:     "int",
			Description:  "Total number of app opens (proxy for engagement, but may not correlate with actual transactions)",
		},
		{
			Name:         "actual_transactions",
			SemanticType: "numeric",
			DataType:     "int",
			Description:  "Actual completed transactions (tests if app_opens is a true proxy or just noise)",
		},
		{
			Name:         "session_duration_seconds",
			SemanticType: "numeric",
			DataType:     "int",
			Description:  "Average session duration in seconds",
		},

		// The Temporal System - with latency and decay patterns
		{
			Name:         "avg_days_between_events",
			SemanticType: "numeric",
			DataType:     "float",
			Description:  "Average days between customer events (tests for velocity and decay patterns)",
		},
		{
			Name:         "last_event_timestamp",
			SemanticType: "timestamp",
			DataType:     "datetime",
			Description:  "Timestamp of most recent customer event (tests for recency decay)",
		},
		{
			Name:         "first_purchase_date",
			SemanticType: "timestamp",
			DataType:     "datetime",
			Description:  "Date of first purchase (allows cohort analysis and temporal segmentation)",
		},

		// The Clustering System - for behavioral segmentation
		{
			Name:         "product_category_preference",
			SemanticType: "categorical",
			DataType:     "string",
			Description:  "Most frequently purchased product category",
		},
		{
			Name:         "purchase_time_of_day",
			SemanticType: "numeric",
			DataType:     "int",
			Description:  "Hour of day when purchases typically occur (0-23)",
		},
		{
			Name:         "geographic_region",
			SemanticType: "categorical",
			DataType:     "string",
			Description:  "Customer geographic region",
		},

		// The Mediation System - potential confounding
		{
			Name:         "marketing_channel",
			SemanticType: "categorical",
			DataType:     "string",
			Description:  "Primary marketing acquisition channel (may mediate relationships between engagement and value)",
		},
		{
			Name:         "customer_segment",
			SemanticType: "categorical",
			DataType:     "string",
			Description:  "Assigned customer segment (may be a proxy for unobserved characteristics)",
		},
	}

	// Convert to JSON string for prompt replacement
	fieldJSON, err := json.MarshalIndent(fieldMetadata, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal field metadata: %v", err)
	}

	// Prepare prompt replacements
	replacements := map[string]string{
		"FIELD_METADATA_JSON": string(fieldJSON),
	}

	// Make the live call with context timeout (increased for reasoning models)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	t.Log("Making live call to OpenAI for greenfield research...")
	response, err := client.GetJsonResponseFromPromptWithContext(ctx, "greenfield_research", replacements)
	if err != nil {
		t.Fatalf("Live call failed: %v", err)
	}

	// Print the response as JSON for debugging
	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		t.Logf("Failed to marshal response: %v", err)
	} else {
		t.Logf("Raw JSON Response:\n%s", string(responseJSON))
	}

	// Validate the response
	if response == nil {
		t.Fatal("Response is nil")
	}

	if len(response.ResearchDirectives) == 0 {
		t.Fatal("No research directives generated")
	}

	t.Logf("✅ Generated %d research directives", len(response.ResearchDirectives))

	// Print the results
	for i, directive := range response.ResearchDirectives {
		t.Logf("Directive %d:", i+1)
		t.Logf("  ID: %s", directive.ID)
		t.Logf("  Business Hypothesis: %s", directive.BusinessHypothesis)
		t.Logf("  Science Hypothesis: %s", directive.ScienceHypothesis)
		t.Logf("  Null Case: %s", directive.NullCase)
		t.Logf("  Validation Methods (%d):", len(directive.ValidationMethods))
		for j, method := range directive.ValidationMethods {
			t.Logf("    %d. %s (%s): %s", j+1, method.MethodName, method.Type, method.ExecutionPlan[:min(100, len(method.ExecutionPlan))]+"...")
		}
		t.Logf("  Referee Gates:")
		t.Logf("    Confidence Target: %.3f", directive.RefereeGates.ConfidenceTarget)
		t.Logf("    Stability Threshold: %.2f", directive.RefereeGates.StabilityThreshold)
		t.Logf("")
	}

	// Additional validation - "20/10" specificity checks
	for i, directive := range response.ResearchDirectives {
		if directive.ID == "" {
			t.Errorf("Directive %d: ID is empty", i+1)
		}
		if directive.BusinessHypothesis == "" {
			t.Errorf("Directive %d: Business hypothesis is empty", i+1)
		}
		if directive.ScienceHypothesis == "" {
			t.Errorf("Directive %d: Science hypothesis is empty", i+1)
		}
		if directive.NullCase == "" {
			t.Errorf("Directive %d: Null case is empty", i+1)
		}

		// Check for null case content (symmetry requirement)
		if len(directive.NullCase) < 20 {
			t.Logf("⚠️  Directive %d: Null case description seems too brief (%d chars)", i+1, len(directive.NullCase))
		}

		// Validate validation methods (anti-hallucination requirement)
		if len(directive.ValidationMethods) != 3 {
			t.Errorf("Directive %d: Must have exactly 3 validation methods (found %d)", i+1, len(directive.ValidationMethods))
		}

		// Check each method has required fields and follows 2-sentence rule
		expectedTypes := []string{"Detector", "Scanner", "Referee"}
		for j, method := range directive.ValidationMethods {
			if method.Type == "" || method.MethodName == "" || method.ExecutionPlan == "" {
				t.Errorf("Directive %d: Validation method %d missing required fields", i+1, j+1)
			}
			if j < len(expectedTypes) && method.Type != expectedTypes[j] {
				t.Logf("⚠️  Directive %d: Method %d type is '%s', expected '%s'", i+1, j+1, method.Type, expectedTypes[j])
			}
			// Check for 2-sentence structure (sentence 1 + sentence 2)
			sentenceCount := strings.Count(method.ExecutionPlan, ".") - strings.Count(method.ExecutionPlan, "...")
			if sentenceCount < 2 {
				t.Logf("⚠️  Directive %d: Method %d execution plan may not follow 2-sentence rule (%d sentences detected)", i+1, j+1, sentenceCount)
			}
		}

		// Check for specificity - reject generic answers in method names and execution plans
		genericTerms := []string{"correlation", "regression", "linear", "simple", "basic"}
		isGeneric := false
		allInstruments := ""
		for _, method := range directive.ValidationMethods {
			allInstruments += method.MethodName + " " + method.ExecutionPlan + " "
		}
		for _, term := range genericTerms {
			if strings.Contains(strings.ToLower(allInstruments), term) {
				isGeneric = true
				break
			}
		}
		if isGeneric {
			t.Logf("⚠️  Directive %d: Instruments may be too generic - look for more specific mathematical descriptions", i+1)
		}

		// Check for mathematical specificity (window sizes, decay constants, etc.)
		specificityIndicators := []string{"window", "decay", "constant", "threshold", "quantile", "cluster", "segment", "bootstrap", "permutation"}
		hasSpecificity := false
		// Reuse allInstruments from above
		for _, indicator := range specificityIndicators {
			if strings.Contains(strings.ToLower(allInstruments), indicator) {
				hasSpecificity = true
				break
			}
		}
		if !hasSpecificity {
			t.Logf("⚠️  Directive %d: Instruments lack mathematical specificity (no windows, decay constants, thresholds, etc.)", i+1)
		}

		// Validate referee gates
		if directive.RefereeGates.ConfidenceTarget <= 0 || directive.RefereeGates.ConfidenceTarget > 1 {
			t.Errorf("Directive %d: Invalid confidence target: %.3f (must be between 0 and 1)", i+1, directive.RefereeGates.ConfidenceTarget)
		}
		if directive.RefereeGates.StabilityThreshold < 0 || directive.RefereeGates.StabilityThreshold > 1 {
			t.Errorf("Directive %d: Invalid stability threshold: %.3f (must be between 0 and 1)", i+1, directive.RefereeGates.StabilityThreshold)
		}
	}

	t.Logf("\n🎯 SPECIFICITY CHECK:")
	t.Logf("Look for instruments like:")
	t.Logf("  ✅ 'Segmented Bootstrapped Polynomial Fit with 36-hour Decay Constant'")
	t.Logf("  ✅ 'Mutual Information with Temporal Window Sliding'")
	t.Logf("  ✅ 'K-Means Clustering with 5 Quantile Thresholds'")
	t.Logf("  ❌ NOT: 'Linear Regression' or 'Correlation Analysis'")
}

// TestPromptLoading verifies that prompts can be loaded correctly
func TestPromptLoading(t *testing.T) {
	// Load environment variables from .env file
	if err := godotenv.Load("../.env"); err != nil {
		_ = godotenv.Load(".env")
	}

	config := &models.AIConfig{
		PromptsDir: getPromptsDir(),
	}

	client := NewStructuredClient[models.GreenfieldResearchOutput](config, config.PromptsDir)

	// Test loading the prompt template
	prompt, err := client.PromptManager.LoadPrompt("greenfield_research")
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	if len(prompt) == 0 {
		t.Error("Prompt is empty")
	}

	if !contains(prompt, "FIELD_METADATA_JSON") {
		t.Error("Prompt template does not contain FIELD_METADATA_JSON placeholder")
	}

	t.Logf("✅ Prompt loaded successfully (%d characters)", len(prompt))
}

// TestPromptRendering verifies that prompt rendering works
func TestPromptRendering(t *testing.T) {
	// Load environment variables from .env file
	if err := godotenv.Load("../.env"); err != nil {
		_ = godotenv.Load(".env")
	}

	config := &models.AIConfig{
		PromptsDir: getPromptsDir(),
	}

	client := NewStructuredClient[models.GreenfieldResearchOutput](config, config.PromptsDir)

	replacements := map[string]string{
		"FIELD_METADATA_JSON": `[{"name": "test", "type": "numeric"}]`,
	}

	rendered, err := client.PromptManager.RenderPrompt("greenfield_research", replacements)
	if err != nil {
		t.Fatalf("Failed to render prompt: %v", err)
	}

	if contains(rendered, "{FIELD_METADATA_JSON}") {
		t.Error("Placeholder was not replaced")
	}

	if !contains(rendered, `[{"name": "test", "type": "numeric"}]`) {
		t.Error("Replacement value not found in rendered prompt")
	}

	t.Logf("✅ Prompt rendered successfully (%d characters)", len(rendered))
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// getPromptsDir finds the prompts directory by trying multiple paths
func getPromptsDir() string {
	// Try paths relative to test file location
	paths := []string{
		"../prompts",    // From ai/ directory
		"./prompts",     // From project root
		"../../prompts", // Alternative
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default fallback
	return "./prompts"
}
