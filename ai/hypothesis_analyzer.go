package ai

import (
	"context"
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

	template := `ANALYZE HYPOTHESIS RISK WITH SCIENTIFIC RIGOR

HYPOTHESIS: {{.BusinessHypothesis}}
SCIENTIFIC CLAIM: {{.ScienceHypothesis}}
CAUSE VARIABLE: {{.CauseKey}}
EFFECT VARIABLE: {{.EffectKey}}

DATA TOPOLOGY CONTEXT:
- Sample Size: {{.SampleSize}} observations
- Sparsity: {{.SparsityRatio}}% missing data
- Cause Variable Cardinality: {{.CardinalityCause}} unique values
- Effect Variable Cardinality: {{.CardinalityEffect}} unique values
- Distribution Skewness (Cause): {{.SkewnessCause}}
- Distribution Skewness (Effect): {{.SkewnessEffect}}
- Temporal Coverage: {{.TemporalCoverage}}% complete

ASSESSED CONFOUNDING SIGNALS:
{{range .ConfoundingSignals}}- {{.}}
{{end}}

RISK ASSESSMENT FRAMEWORK:

1. SEMANTIC COMPLEXITY (1-10 scale):
   - 1-2: Simple descriptive claims ("X is correlated with Y")
   - 3-5: Moderate causal claims ("X influences Y under certain conditions")
   - 6-8: Complex causal claims ("X influences Y through mediating variables")
   - 9-10: Extraordinary claims ("X reverses established relationships")

2. STATISTICAL FRAGILITY (0.0-1.0 scale):
   - Based on sample size, sparsity, cardinality, and temporal coverage
   - Higher fragility = more validation tests required

3. RISK LEVEL DETERMINATION:
   - LOW: Simple claims with abundant, clean data (>1000 samples, <5% missing, low skew)
   - MEDIUM: Moderate complexity or data challenges
   - HIGH: Complex causality, sparse data, or high confounding potential
   - CRITICAL: Claims requiring extraordinary evidence (counterintuitive, high-stakes)

4. TEST COUNT RECOMMENDATIONS:
   - LOW RISK: 1-3 basic integrity checks
   - MEDIUM RISK: 3-6 comprehensive tests
   - HIGH RISK: 6-9 rigorous validation
   - CRITICAL RISK: 8-10 full statistical battery

5. CATEGORY PRIORITIZATION:
   - Always include SHREDDER for statistical integrity
   - Add INVARIANCE for temporal claims
   - Add ANTI_CONFOUNDER for complex causal claims
   - Add MECHANISM for non-obvious relationships

REQUIRED OUTPUT: Valid JSON with risk assessment, test count range, critical concerns, and rationale.`

	// Prepare template data (placeholder for now)
	_ = struct {
		BusinessHypothesis string
		ScienceHypothesis  string
		CauseKey           string
		EffectKey          string
		SampleSize         int
		SparsityRatio      float64
		CardinalityCause   int
		CardinalityEffect  int
		SkewnessCause      float64
		SkewnessEffect     float64
		TemporalCoverage   float64
		ConfoundingSignals []string
	}{
		BusinessHypothesis: hypothesis.BusinessHypothesis,
		ScienceHypothesis:  hypothesis.ScienceHypothesis,
		CauseKey:           hypothesis.CauseKey,
		EffectKey:          hypothesis.EffectKey,
		SampleSize:         dataSnapshot.SampleSize,
		SparsityRatio:      dataSnapshot.SparsityRatio,
		CardinalityCause:   dataSnapshot.CardinalityCause,
		CardinalityEffect:  dataSnapshot.CardinalityEffect,
		SkewnessCause:      dataSnapshot.SkewnessCause,
		SkewnessEffect:     dataSnapshot.SkewnessEffect,
		TemporalCoverage:   dataSnapshot.TemporalCoverage,
		ConfoundingSignals: dataSnapshot.ConfoundingSignals,
	}

	return template, nil // TODO: Implement proper template rendering
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
