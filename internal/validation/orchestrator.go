package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/internal/referee"
	"gohypo/models"
	"gohypo/ports"
)

type ValidationConfig struct {
	// Circuit breaker settings
	MaxComputationalCapacity int
	CapacityTimeout          time.Duration

	// Stability selection settings
	StabilityEnabled         bool
	SubsampleCount          int
	SubsampleFraction       float64
	StabilityThreshold      float64

	// Logical auditor settings
	LogicalAuditorEnabled   bool
	AuditorModel           string

	// Overall validation settings
	ValidationTimeout      time.Duration
}

type ValidationResult struct {
	HypothesisID     string
	Passed           bool
	Confidence       float64
	EValue          float64
	RefereeResults   []referee.RefereeResult
	StabilityResult  *StabilityResult
	AuditorResult    *AuditorResult
	ExecutionTime    time.Duration
	Error            error
}

type AuditorResult struct {
	Decision         string  `json:"decision"`
	ConfidenceScore  float64 `json:"confidence_score"`
	Severity         string  `json:"severity"`
	RecommendedAction string `json:"recommended_action"`
	Reasoning        map[string]string `json:"reasoning"`
	RefereeDirective *RefereeDirective `json:"referee_directive,omitempty"`
}

// AuditorDirective represents the complete output from the Logical Auditor
type AuditorDirective struct {
	// Overall decision
	Decision        string  `json:"decision"`
	ConfidenceScore float64 `json:"confidence_score"`

	// Analysis context
	HypothesisAnalysis HypothesisAnalysis `json:"hypothesis_analysis"`
	DataAssessment     DataAssessment     `json:"data_assessment"`

	// Technical directive
	RefereeDirective RefereeDirective `json:"referee_directive"`

	// Operational metadata
	Severity             string   `json:"severity"`
	RecommendedAction    string   `json:"recommended_action"`
	AlternativeApproaches []string `json:"alternative_approaches,omitempty"`
	ProcessingNotes      string   `json:"processing_notes,omitempty"`
}

// HypothesisAnalysis captures the Auditor's understanding of the hypothesis
type HypothesisAnalysis struct {
	Type               string   `json:"type"` // CAUSAL, ASSOCIATIVE, TEMPORAL, MECHANISTIC, SPATIAL
	DirectionalClaims  bool     `json:"directional_claims"`
	TemporalElements   bool     `json:"temporal_elements"`
	ComplexityLevel    string   `json:"complexity_level"` // SIMPLE, MODERATE, COMPLEX
	KeyTerms          []string `json:"key_terms"` // Words that triggered specific referee selection
	BusinessStake      string   `json:"business_stake"` // EXPLORATORY, TACTICAL, STRATEGIC
}

// DataAssessment captures the Auditor's evaluation of data quality
type DataAssessment struct {
	SampleSize         int     `json:"sample_size"`
	DistributionType   string  `json:"distribution_type"` // NORMAL, SKEWED, HEAVY_TAILED, DISCRETE
	DataStructure      string  `json:"data_structure"` // CROSS_SECTIONAL, TIME_SERIES, PANEL, SPATIAL
	QualityFlags       []string `json:"quality_flags"` // OUTLIERS, MISSING_DATA, MULTICOLLINEARITY, etc.
	AssumptionConcerns []string `json:"assumption_concerns"` // Issues that affect statistical test validity
}

// RefereeDirective contains the specific technical instructions for validation
type RefereeDirective struct {
	SelectedReferees     []SelectedReferee `json:"selected_referees"`
	EnsembleStrategy     string           `json:"ensemble_strategy"`
	ExecutionPriority    string           `json:"execution_priority"` // SEQUENTIAL, PARALLEL, HYBRID
	ExpectedDuration     string           `json:"expected_duration"` // e.g., "3-5 minutes"
	ComputationalBudget  int              `json:"computational_budget"` // Total cost units allowed
	ConfidenceThreshold  float64          `json:"confidence_threshold"`
	FallbackStrategy     string           `json:"fallback_strategy,omitempty"`
}

// SelectedReferee represents one chosen statistical test with full justification
type SelectedReferee struct {
	Name               string            `json:"name"`
	Category           string            `json:"category"`
	Priority           int               `json:"priority"` // 1=MANDATORY, 2=HIGH, 3=MEDIUM, 4=OPTIONAL
	Rationale          string            `json:"rationale"`
	ComputationalCost  int               `json:"computational_cost"` // 1-10 scale
	StatisticalPower   string            `json:"statistical_power"`
	AssumptionChecks   []string          `json:"assumption_checks"` // What data assumptions this test validates
	FailureImplications string           `json:"failure_implications"` // What it means if this test fails
	TriggeredBy        map[string]string `json:"triggered_by,omitempty"` // What hypothesis elements triggered this selection
}

type ValidationOrchestrator struct {
	config             ValidationConfig
	concurrentExecutor *ConcurrentExecutor
	stabilitySelector   *StabilitySelector
	llmClient          ports.LLMClient
	heuristicAuditor   *HeuristicAuditor
	promptManager      *ai.PromptManager
}

func NewValidationOrchestrator(
	config ValidationConfig,
	llmClient ports.LLMClient,
	heuristicAuditor *HeuristicAuditor,
	promptsDir string,
) *ValidationOrchestrator {

	return &ValidationOrchestrator{
		config: config,
		concurrentExecutor: NewConcurrentExecutor(config.MaxComputationalCapacity),
		stabilitySelector: NewStabilitySelector(StabilitySelectionConfig{
			SubsampleCount:    config.SubsampleCount,
			SubsampleFraction: config.SubsampleFraction,
			StabilityThreshold: config.StabilityThreshold,
			RandomSeed:       time.Now().UnixNano(),
		}),
		llmClient:        llmClient,
		heuristicAuditor: heuristicAuditor,
		promptManager:    ai.NewPromptManager(promptsDir),
	}
}

// ValidateHypothesis performs comprehensive validation using all available guardrails
func (vo *ValidationOrchestrator) ValidateHypothesis(
	ctx context.Context,
	hypothesis *models.ResearchDirectiveResponse,
	xData, yData []float64,
	statisticalEvidence map[string]interface{},
) (*ValidationResult, error) {

	startTime := time.Now()
	result := &ValidationResult{
		HypothesisID: hypothesis.ID,
	}

	// Set timeout for entire validation process
	validationCtx, cancel := context.WithTimeout(ctx, vo.config.ValidationTimeout)
	defer cancel()

	// Phase 1: Logical Auditor (if enabled)
	var selectedReferees []string
	if vo.config.LogicalAuditorEnabled {
		auditorResult, err := vo.performLogicalAudit(validationCtx, hypothesis, statisticalEvidence)
		if err != nil {
			result.Error = fmt.Errorf("logical audit failed: %w", err)
			result.ExecutionTime = time.Since(startTime)
			return result, result.Error
		}

		result.AuditorResult = auditorResult

		// If logical auditor rejects with high confidence, stop validation
		if auditorResult.Decision == "REJECT" && auditorResult.ConfidenceScore > 0.8 {
			result.Passed = false
			result.ExecutionTime = time.Since(startTime)
			return result, nil
		}

		// Extract selected referees from auditor directive
		if auditorResult.RefereeDirective != nil {
			selectedReferees = make([]string, len(auditorResult.RefereeDirective.SelectedReferees))
			for i, referee := range auditorResult.RefereeDirective.SelectedReferees {
				selectedReferees[i] = referee.Name
			}
		}
	}

	// Fallback to hypothesis referees if auditor didn't provide selection
	if len(selectedReferees) == 0 {
		selectedReferees = hypothesis.RefereeGates.SelectedReferees
	}

	// Phase 2: Stability Selection (if enabled)
	if vo.config.StabilityEnabled {
		stabilityResult, err := vo.stabilitySelector.ValidateWithStability(
			validationCtx,
			selectedReferees,
			xData, yData,
			core.VariableKey(hypothesis.CauseKey),
			core.VariableKey(hypothesis.EffectKey),
		)

		if err != nil {
			result.Error = fmt.Errorf("stability selection failed: %w", err)
			result.ExecutionTime = time.Since(startTime)
			return result, result.Error
		}

		result.StabilityResult = stabilityResult

		// If stability score is too low, mark as failed
		if stabilityResult.OverallStability < vo.config.StabilityThreshold {
			result.Passed = false
			result.Confidence = stabilityResult.OverallStability
			result.ExecutionTime = time.Since(startTime)
			return result, nil
		}
	}

	// Phase 3: Concurrent Referee Execution with Circuit Breaker
	refereeResults, err := vo.concurrentExecutor.ExecuteReferees(
		validationCtx,
		selectedReferees,
		xData, yData,
	)

	if err != nil {
		result.Error = fmt.Errorf("referee execution failed: %w", err)
		result.ExecutionTime = time.Since(startTime)
		return result, result.Error
	}

	result.RefereeResults = refereeResults

	// Phase 4: Aggregate Results
	result.Passed = vo.aggregateValidationResults(result)
	result.Confidence = vo.calculateOverallConfidence(result)
	result.EValue = vo.calculateEValue(result)
	result.ExecutionTime = time.Since(startTime)

	return result, nil
}

// performLogicalAudit runs the LLM-based logical auditor
func (vo *ValidationOrchestrator) performLogicalAudit(
	ctx context.Context,
	hypothesis *models.ResearchDirectiveResponse,
	statisticalEvidence map[string]interface{},
) (*AuditorResult, error) {

	// Prepare comprehensive context for LLM
	contextData := map[string]interface{}{
		"business_hypothesis":          hypothesis.BusinessHypothesis,
		"science_hypothesis":          hypothesis.ScienceHypothesis,
		"null_case":                   hypothesis.NullCase,
		"statistical_relationship_json": statisticalEvidence,
		"variable_context_json": map[string]interface{}{
			"cause_key":  hypothesis.CauseKey,
			"effect_key": hypothesis.EffectKey,
		},
		"rigor_level":                 "decision-critical", // TODO: Make configurable
		"computational_budget":        "medium",            // TODO: Make configurable
	}

	// Render prompt
	prompt, err := vo.renderLogicalAuditorPrompt(contextData)
	if err != nil {
		return nil, fmt.Errorf("failed to render auditor prompt: %w", err)
	}

		// Call LLM with timeout
	llmCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // 30 second timeout for LLM
	defer cancel()

	response, err := vo.llmClient.ChatCompletion(llmCtx, vo.config.AuditorModel, prompt, 2000)
	if err != nil {
		// LLM failed - use heuristic auditor as fallback
		log.Printf("[ValidationOrchestrator] LLM auditor failed (%v), using heuristic fallback", err)

		if vo.heuristicAuditor == nil {
			return nil, fmt.Errorf("both LLM and heuristic auditors unavailable: %w", err)
		}

		// Extract data for heuristic auditor
		xData, yData, dataErr := vo.extractDataFromEvidence(statisticalEvidence)
		if dataErr != nil {
			return nil, fmt.Errorf("failed to extract data for heuristic auditor: %w", dataErr)
		}

		// Use heuristic auditor
		directive, heuristicErr := vo.heuristicAuditor.GetHeuristicDirective(ctx, hypothesis, xData, yData)
		if heuristicErr != nil {
			return nil, fmt.Errorf("heuristic auditor also failed: %w", heuristicErr)
		}

		// Convert heuristic directive to AuditorDirective format
		auditorDirective := *directive

		// Convert to AuditorResult format for backward compatibility
		result := &AuditorResult{
			Decision:         auditorDirective.Decision,
			ConfidenceScore:  auditorDirective.ConfidenceScore,
			Severity:         auditorDirective.Severity,
			RecommendedAction: auditorDirective.RecommendedAction,
			RefereeDirective: &auditorDirective.RefereeDirective,
		}

		return result, nil
	}

	// Parse LLM response
	var auditorDirective AuditorDirective
	if err := json.Unmarshal([]byte(response), &auditorDirective); err != nil {
		// LLM returned invalid JSON - use heuristic fallback
		log.Printf("[ValidationOrchestrator] LLM returned invalid JSON (%v), using heuristic fallback", err)

		if vo.heuristicAuditor == nil {
			return nil, fmt.Errorf("LLM returned invalid response and no heuristic fallback available: %w", err)
		}

		// Extract data for heuristic auditor
		xData, yData, dataErr := vo.extractDataFromEvidence(statisticalEvidence)
		if dataErr != nil {
			return nil, fmt.Errorf("failed to extract data for heuristic auditor: %w", dataErr)
		}

		// Use heuristic auditor
		directive, heuristicErr := vo.heuristicAuditor.GetHeuristicDirective(ctx, hypothesis, xData, yData)
		if heuristicErr != nil {
			return nil, fmt.Errorf("heuristic auditor also failed: %w", heuristicErr)
		}

		// Convert heuristic directive to AuditorDirective format
		auditorDirective = *directive
	}

	// Convert to AuditorResult format for backward compatibility
	result := &AuditorResult{
		Decision:         auditorDirective.Decision,
		ConfidenceScore:  auditorDirective.ConfidenceScore,
		Severity:         auditorDirective.Severity,
		RecommendedAction: auditorDirective.RecommendedAction,
		RefereeDirective: &auditorDirective.RefereeDirective,
	}

	return result, nil
}

// extractDataFromEvidence extracts x,y data from statistical evidence for heuristic auditor
func (vo *ValidationOrchestrator) extractDataFromEvidence(evidence map[string]interface{}) ([]float64, []float64, error) {
	// Try to extract data from various possible formats in the evidence
	// This is a simplified implementation - would need to be enhanced based on actual data structure

	xDataInterface, hasX := evidence["x_data"]
	yDataInterface, hasY := evidence["y_data"]

	if !hasX || !hasY {
		// Try alternative keys
		xDataInterface, hasX = evidence["cause_data"]
		yDataInterface, hasY = evidence["effect_data"]
	}

	if !hasX || !hasY {
		return nil, nil, fmt.Errorf("could not find x_data and y_data in statistical evidence")
	}

	// Convert interface{} to []float64
	xData, err := vo.convertToFloat64Slice(xDataInterface)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert x_data: %w", err)
	}

	yData, err := vo.convertToFloat64Slice(yDataInterface)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to convert y_data: %w", err)
	}

	if len(xData) != len(yData) {
		return nil, nil, fmt.Errorf("x_data and y_data have different lengths: %d vs %d", len(xData), len(yData))
	}

	return xData, yData, nil
}

// convertToFloat64Slice converts various data types to []float64
func (vo *ValidationOrchestrator) convertToFloat64Slice(data interface{}) ([]float64, error) {
	switch v := data.(type) {
	case []float64:
		return v, nil
	case []interface{}:
		result := make([]float64, len(v))
		for i, val := range v {
			switch num := val.(type) {
			case float64:
				result[i] = num
			case float32:
				result[i] = float64(num)
			case int:
				result[i] = float64(num)
			case int64:
				result[i] = float64(num)
			default:
				return nil, fmt.Errorf("unsupported number type at index %d: %T", i, val)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported data type: %T", data)
	}
}

// renderLogicalAuditorPrompt creates the logical auditor prompt using the external template
func (vo *ValidationOrchestrator) renderLogicalAuditorPrompt(contextData map[string]interface{}) (string, error) {
	// Convert context data to string replacements for the template
	replacements := make(map[string]string)
	for key, value := range contextData {
		if value == nil {
			replacements[key] = ""
			continue
		}

		switch v := value.(type) {
		case string:
			replacements[key] = v
		default:
			// Convert to JSON string for complex types
			jsonBytes, err := json.Marshal(value)
			if err != nil {
				return "", fmt.Errorf("failed to marshal %s to JSON: %w", key, err)
			}
			replacements[key] = string(jsonBytes)
		}
	}

	// Load and render the logical_auditor prompt
	prompt, err := vo.promptManager.RenderPrompt("logical_auditor", replacements)
	if err != nil {
		return "", fmt.Errorf("failed to render logical_auditor prompt: %w", err)
	}

	return prompt, nil
}

// aggregateValidationResults combines referee results into final decision
func (vo *ValidationOrchestrator) aggregateValidationResults(result *ValidationResult) bool {
	if len(result.RefereeResults) == 0 {
		return false
	}

	// Require at least one referee to pass
	passedCount := 0
	for _, refereeResult := range result.RefereeResults {
		if refereeResult.Passed {
			passedCount++
		}
	}

	// If stability selection was used, factor it in
	if result.StabilityResult != nil {
		// Only pass if both referees AND stability selection agree
		return passedCount > 0 && result.StabilityResult.OverallStability >= vo.config.StabilityThreshold
	}

	return passedCount > 0
}

// calculateOverallConfidence computes confidence score across all validation methods
func (vo *ValidationOrchestrator) calculateOverallConfidence(result *ValidationResult) float64 {
	if len(result.RefereeResults) == 0 {
		return 0.0
	}

	// Base confidence from referee agreement
	passedCount := 0
	for _, refereeResult := range result.RefereeResults {
		if refereeResult.Passed {
			passedCount++
		}
	}
	refereeConfidence := float64(passedCount) / float64(len(result.RefereeResults))

	// Factor in stability if available
	if result.StabilityResult != nil {
		return (refereeConfidence + result.StabilityResult.OverallStability) / 2.0
	}

	// Factor in auditor confidence if available
	if result.AuditorResult != nil && result.AuditorResult.Decision == "APPROVE" {
		return (refereeConfidence + result.AuditorResult.ConfidenceScore) / 2.0
	}

	return refereeConfidence
}

// calculateEValue computes the evidence value for the validation
func (vo *ValidationOrchestrator) calculateEValue(result *ValidationResult) float64 {
	if !result.Passed {
		return 0.0
	}

	// Simple e-value calculation based on confidence
	return result.Confidence * 10.0 // Scale to reasonable e-value range
}

// GetValidationMetrics returns operational metrics for monitoring
func (vo *ValidationOrchestrator) GetValidationMetrics() map[string]interface{} {
	return map[string]interface{}{
		"max_capacity":            vo.config.MaxComputationalCapacity,
		"stability_enabled":       vo.config.StabilityEnabled,
		"logical_auditor_enabled": vo.config.LogicalAuditorEnabled,
		"validation_timeout":      vo.config.ValidationTimeout.String(),
	}
}
