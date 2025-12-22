package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"gohypo/domain/stats"
	"gohypo/models"
	"gohypo/ports"
	"strings"
)

// HypothesisRiskProfile represents the comprehensive risk assessment of a hypothesis
type HypothesisRiskProfile struct {
	RiskLevel           stats.HypothesisRiskLevel `json:"risk_level"`
	RequiredTestCount   struct {
		Min int `json:"min"`
		Max int `json:"max"`
	} `json:"required_test_count"`
	CriticalConcerns    []string               `json:"critical_concerns"`
	RecommendedCategories []stats.RefereeCategory `json:"recommended_categories"`
	ConfidenceTarget    float64                `json:"confidence_target"`
	FeasibilityScore    float64                `json:"feasibility_score"` // 0.0-1.0
	Rationale           string                 `json:"rationale"`
	DataTopology        DataTopologyAssessment `json:"data_topology"`
	SemanticComplexity  int                    `json:"semantic_complexity"` // 1-10
	StatisticalFragility float64               `json:"statistical_fragility"` // 0.0-1.0
}

// DataTopologyAssessment captures dataset characteristics for hypothesis evaluation
type DataTopologyAssessment struct {
	SampleSize         int     `json:"sample_size"`
	SparsityRatio      float64 `json:"sparsity_ratio"`        // % missing data
	CardinalityCause   int     `json:"cardinality_cause"`     // Unique values in cause variable
	CardinalityEffect  int     `json:"cardinality_effect"`    // Unique values in effect variable
	SkewnessCause      float64 `json:"skewness_cause"`        // Distribution skewness
	SkewnessEffect     float64 `json:"skewness_effect"`       // Distribution skewness
	TemporalCoverage   float64 `json:"temporal_coverage"`     // % of time period covered
	ConfoundingSignals []string `json:"confounding_signals"`  // Potential hidden variables
}

// DataTopologySnapshot represents the dataset characteristics passed to the analyzer
type DataTopologySnapshot struct {
	SampleSize         int
	SparsityRatio      float64
	CardinalityCause   int
	CardinalityEffect  int
	SkewnessCause      float64
	SkewnessEffect     float64
	TemporalCoverage   float64
	ConfoundingSignals []string
	AvailableFields    []string // For UI feedback
}

// HypothesisAnalysisAgent evaluates hypotheses for risk and feasibility
type HypothesisAnalysisAgent struct {
	llmClient     *StructuredClient[HypothesisRiskProfile]
	promptManager *PromptManager
}

// NewHypothesisAnalysisAgent creates a new hypothesis analysis agent
func NewHypothesisAnalysisAgent(llmClient ports.LLMClient, promptsDir string) *HypothesisAnalysisAgent {
	structuredClient := &StructuredClient[HypothesisRiskProfile]{
		LLMClient:     llmClient,
		PromptManager: NewPromptManager(promptsDir),
		SystemContext: "You are the GoHypo Lead Auditor. Your task is to assign a Risk Profile to a hypothesis based on its phrasing AND the provided Data Topology.",
	}

	return &HypothesisAnalysisAgent{
		llmClient:     structuredClient,
		promptManager: NewPromptManager(promptsDir),
	}
}

// AnalyzeHypothesis performs comprehensive risk assessment of a hypothesis
func (haa *HypothesisAnalysisAgent) AnalyzeHypothesis(
	ctx context.Context,
	hypothesis models.ResearchDirectiveResponse,
	dataSnapshot DataTopologySnapshot,
) (*HypothesisRiskProfile, error) {

	// Build the analysis prompt
	prompt, err := haa.buildAnalysisPrompt(hypothesis, dataSnapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to build analysis prompt: %w", err)
	}

	// Call LLM for analysis
	systemMessage := `You are the GoHypo Lead Auditor. Your task is to assign a Risk Profile to a hypothesis based on its phrasing AND the provided Data Topology.

CRITICAL: Consider BOTH the semantic ambition of the hypothesis AND the statistical feasibility given the data characteristics.

OUTPUT FORMAT: Valid JSON matching the HypothesisRiskProfile structure.`

	result, err := haa.llmClient.GetJsonResponseWithContext(ctx, "openai", prompt, systemMessage)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Validate and enhance the result
	validatedResult := haa.validateAndEnhanceResult(result, dataSnapshot)

	return validatedResult, nil
}

// buildAnalysisPrompt constructs the detailed analysis prompt
func (haa *HypothesisAnalysisAgent) buildAnalysisPrompt(
	hypothesis models.ResearchDirectiveResponse,
	dataSnapshot DataTopologySnapshot,
) (string, error) {

	// Build confounding signals JSON
	confoundingJSON := "[]"
	if len(dataSnapshot.ConfoundingSignals) > 0 {
		confoundingBytes, err := json.Marshal(dataSnapshot.ConfoundingSignals)
		if err == nil {
			confoundingJSON = string(confoundingBytes)
		}
	}

	// Prepare template replacements
	replacements := map[string]string{
		"business_hypothesis": hypothesis.BusinessHypothesis,
		"science_hypothesis":  hypothesis.ScienceHypothesis,
		"cause_key":           hypothesis.CauseKey,
		"effect_key":          hypothesis.EffectKey,
		"sample_size":         fmt.Sprintf("%d", dataSnapshot.SampleSize),
		"sparsity_ratio":      fmt.Sprintf("%.1f", dataSnapshot.SparsityRatio),
		"cardinality_cause":   fmt.Sprintf("%d", dataSnapshot.CardinalityCause),
		"cardinality_effect":  fmt.Sprintf("%d", dataSnapshot.CardinalityEffect),
		"skewness_cause":      fmt.Sprintf("%.2f", dataSnapshot.SkewnessCause),
		"skewness_effect":     fmt.Sprintf("%.2f", dataSnapshot.SkewnessEffect),
		"temporal_coverage":   fmt.Sprintf("%.1f", dataSnapshot.TemporalCoverage),
		"confounding_signals": confoundingJSON,
	}

	// Load and render the hypothesis_risk_analysis prompt
	prompt, err := haa.promptManager.RenderPrompt("hypothesis_risk_analysis", replacements)
	if err != nil {
		return "", fmt.Errorf("failed to render hypothesis_risk_analysis prompt: %w", err)
	}

	return prompt, nil
}

// validateAndEnhanceResult ensures the LLM output is reasonable and complete
func (haa *HypothesisAnalysisAgent) validateAndEnhanceResult(
	result *HypothesisRiskProfile,
	dataSnapshot DataTopologySnapshot,
) *HypothesisRiskProfile {

	// Ensure reasonable bounds
	if result.RequiredTestCount.Min < 1 {
		result.RequiredTestCount.Min = 1
	}
	if result.RequiredTestCount.Max > 10 {
		result.RequiredTestCount.Max = 10
	}
	if result.RequiredTestCount.Min > result.RequiredTestCount.Max {
		result.RequiredTestCount.Max = result.RequiredTestCount.Min
	}

	// Ensure feasibility score is reasonable
	if result.FeasibilityScore < 0.0 {
		result.FeasibilityScore = 0.0
	}
	if result.FeasibilityScore > 1.0 {
		result.FeasibilityScore = 1.0
	}

	// Ensure confidence target is reasonable
	if result.ConfidenceTarget < 0.8 {
		result.ConfidenceTarget = 0.8
	}
	if result.ConfidenceTarget > 0.999 {
		result.ConfidenceTarget = 0.999
	}

	// Add data topology to result for transparency
	result.DataTopology = DataTopologyAssessment{
		SampleSize:         dataSnapshot.SampleSize,
		SparsityRatio:      dataSnapshot.SparsityRatio,
		CardinalityCause:   dataSnapshot.CardinalityCause,
		CardinalityEffect:  dataSnapshot.CardinalityEffect,
		SkewnessCause:      dataSnapshot.SkewnessCause,
		SkewnessEffect:     dataSnapshot.SkewnessEffect,
		TemporalCoverage:   dataSnapshot.TemporalCoverage,
		ConfoundingSignals: dataSnapshot.ConfoundingSignals,
	}

	return result
}

// QuickRiskAssessment provides a lightweight risk check for UI feedback
func (haa *HypothesisAnalysisAgent) QuickRiskAssessment(
	hypothesis models.ResearchDirectiveResponse,
	dataSnapshot DataTopologySnapshot,
) (stats.HypothesisRiskLevel, string) {

	// Rule-based quick assessment for UI responsiveness
	riskScore := 0.0
	concerns := []string{}

	// Data quality concerns
	if dataSnapshot.SparsityRatio > 0.2 {
		riskScore += 2.0
		concerns = append(concerns, "High data sparsity")
	}

	if dataSnapshot.SampleSize < 100 {
		riskScore += 3.0
		concerns = append(concerns, "Small sample size")
	}

	// Hypothesis complexity (simple keyword analysis)
	hypothesisText := hypothesis.ScienceHypothesis + " " + hypothesis.BusinessHypothesis
	if containsComplexTerms(hypothesisText) {
		riskScore += 2.0
		concerns = append(concerns, "Complex causal claim")
	}

	// Determine risk level
	var riskLevel stats.HypothesisRiskLevel
	switch {
	case riskScore >= 5.0:
		riskLevel = stats.RiskLevelVeryHigh
	case riskScore >= 3.0:
		riskLevel = stats.RiskLevelHigh
	case riskScore >= 1.0:
		riskLevel = stats.RiskLevelMedium
	default:
		riskLevel = stats.RiskLevelLow
	}

	rationale := fmt.Sprintf("Risk score: %.1f. Concerns: %v", riskScore, concerns)
	return riskLevel, rationale
}

// containsComplexTerms checks for complex causal language
func containsComplexTerms(text string) bool {
	complexTerms := []string{
		"mediates", "interacts", "moderates", "confounds",
		"causes", "influences", "drives", "determines",
		"reverses", "eliminates", "transforms",
	}

	text = strings.ToLower(text)
	for _, term := range complexTerms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}
