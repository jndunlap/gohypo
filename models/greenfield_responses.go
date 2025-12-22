package models

import (
	"fmt"
	"time"
)

// GreenfieldResearchOutput - Exact match for your JSON schema
type GreenfieldResearchOutput struct {
	IndustryContext    string                      `json:"industry_context" description:"Two-sentence semantic summary of industry domain and primary friction points"`
	ResearchDirectives []ResearchDirectiveResponse `json:"research_directives" description:"Array of research directives from the LLM"`
}

type ResearchDirectiveResponse struct {
	ID                  string              `json:"id" description:"Unique directive identifier (e.g., HYP-001)"`
	BusinessHypothesis  string              `json:"business_hypothesis" description:"The simple story of the impact"`
	ScienceHypothesis   string              `json:"science_hypothesis" description:"The technical pattern in the data"`
	NullCase            string              `json:"null_case" description:"Quantitative description of a failed/random result"`
	CauseKey            string              `json:"cause_key" description:"Variable name for the hypothesized cause"`
	EffectKey           string              `json:"effect_key" description:"Variable name for the hypothesized effect"`
	OpportunityAnalysis OpportunityAnalysis `json:"opportunity_analysis" description:"Business impact and strategic value assessment"`
	RefereeGates        RefereeGates        `json:"referee_gates" description:"Structured referee selection and validation"`
	// Legacy fields for backward compatibility
	ValidationMethods  []ValidationMethod `json:"validation_methods,omitempty" description:"Legacy validation methods"`
	Claim              string             `json:"claim,omitempty" description:"Legacy field"`
	LogicType          string             `json:"logic_type,omitempty" description:"Legacy field"`
	ValidationStrategy ValidationStrategy `json:"validation_strategy,omitempty" description:"Legacy field"`
}

type OpportunityAnalysis struct {
	StrategicValue string `json:"strategic_value" description:"The specific advantage gained by acting on this discovery"`
	RiskOfInaction string `json:"risk_of_inaction" description:"The cost of allowing this systemic inefficiency to persist"`
	LeverageScore  string `json:"leverage_score" description:"High/Med/Low based on node centrality and potential impact"`
}

type ValidationMethod struct {
	Type          string `json:"type" description:"Type of validation method (Detector, Scanner, Referee)"`
	MethodName    string `json:"method_name" description:"Specific tool name"`
	ExecutionPlan string `json:"execution_plan" description:"2-sentence execution plan"`
}

type ValidationStrategy struct {
	Detector string `json:"detector" description:"Primary statistical instrument (mutual_information, spearmans_rho)"`
	Scanner  string `json:"scanner" description:"Segmentation logic (quantile_split, k_means)"`
	Proxy    string `json:"proxy" description:"ML referee (shap_values, random_forest_importance)"`
}

type RefereeGates struct {
	SelectedReferees []string `json:"selected_referees" description:"Referees selected for dynamic e-value validation"`
	ConfidenceTarget float64  `json:"confidence_target" description:"Target confidence level (e.g., 0.999)"`
	Rationale        string   `json:"rationale" description:"Explanation of why these 3 referees were selected"`
	// Legacy fields for backward compatibility
	StabilityThreshold float64 `json:"stability_threshold,omitempty" description:"Legacy field"`
	PValueThreshold    float64 `json:"p_value_threshold,omitempty" description:"Legacy field"`
	StabilityScore     float64 `json:"stability_score,omitempty" description:"Legacy field"`
	PermutationRuns    int     `json:"permutation_runs,omitempty" description:"Legacy field"`
}

// Validate ensures the RefereeGates structure contains valid referee selections
func (rg *RefereeGates) Validate() error {
	// Dynamic e-value validation allows any number of referees (including 0)
	// No validation required for referee count

	// Check for duplicates
	seen := make(map[string]bool)
	for _, referee := range rg.SelectedReferees {
		if seen[referee] {
			return fmt.Errorf("duplicate referee selection: %s", referee)
		}
		seen[referee] = true
	}

	// Validate referee names against approved list
	validReferees := []string{
		"Permutation_Shredder",
		"Transfer_Entropy",
		"Convergent_Cross_Mapping",
		"Chow_Stability_Test",
		"Conditional_MI",
		"Isotonic_Mechanism_Check",
		"LOO_Cross_Validation",
		"Persistent_Homology",
		"Algorithmic_Complexity",
		"Wavelet_Coherence",
	}

	for _, selected := range rg.SelectedReferees {
		found := false
		for _, valid := range validReferees {
			if selected == valid {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid referee: %s", selected)
		}
	}

	return nil
}

// RefereeResult represents the result of a single referee validation
type RefereeResult struct {
	GateName      string  `json:"gate_name"`
	Passed        bool    `json:"passed"`
	Statistic     float64 `json:"statistic"`
	PValue        float64 `json:"p_value"`
	EValue        float64 `json:"e_value"` // E-value from evidence auditing
	StandardUsed  string  `json:"standard_used"`
	FailureReason string  `json:"failure_reason,omitempty"`
}

// TriGateResult represents the aggregated result of Tri-Gate validation
type TriGateResult struct {
	RefereeResults   []RefereeResult `json:"referee_results"`
	OverallPassed    bool            `json:"overall_passed"`
	Confidence       float64         `json:"confidence"`
	NormalizedEValue float64         `json:"normalized_e_value"` // 0-1 scale for UX
	QualityRating    string          `json:"quality_rating"`     // Hypothesis quality rating
	Rationale        string          `json:"rationale"`
}

// HypothesisResult represents the complete result of hypothesis validation
type HypothesisResult struct {
	ID                  string                 `json:"id"`
	SessionID           string                 `json:"session_id,omitempty"`   // Added for database storage
	WorkspaceID         string                 `json:"workspace_id,omitempty"` // Added for workspace scoping
	BusinessHypothesis  string                 `json:"business_hypothesis"`
	ScienceHypothesis   string                 `json:"science_hypothesis"`
	NullCase            string                 `json:"null_case"`
	RefereeResults      []RefereeResult        `json:"referee_results"`
	Passed              bool                   `json:"passed"`
	ValidationTimestamp time.Time              `json:"validation_timestamp"`
	StandardsVersion    string                 `json:"standards_version"`
	ExecutionMetadata   map[string]interface{} `json:"execution_metadata"`

	// New research ledger fields
	PhaseEValues     []float64              `json:"phase_e_values"`
	FeasibilityScore float64                `json:"feasibility_score"`
	RiskLevel        string                 `json:"risk_level"`
	DataTopology     map[string]interface{} `json:"data_topology"`
	CurrentEValue    float64                `json:"current_e_value"`
	NormalizedEValue float64                `json:"normalized_e_value"`
	Confidence       float64                `json:"confidence"`
	Status           string                 `json:"status"`

	// Legacy fields for backward compatibility
	Validated bool      `json:"validated,omitempty"`  // maps to Passed
	Rejected  bool      `json:"rejected,omitempty"`   // maps to !Passed
	CreatedAt time.Time `json:"created_at,omitempty"` // maps to ValidationTimestamp
}
