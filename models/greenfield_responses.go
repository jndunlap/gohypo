package models

// GreenfieldResearchOutput - Exact match for your JSON schema
type GreenfieldResearchOutput struct {
	IndustryContext   string                      `json:"industry_context" description:"Two-sentence semantic summary of industry domain and primary friction points"`
	ResearchDirectives []ResearchDirectiveResponse `json:"research_directives" description:"Array of research directives from the LLM"`
}

type ResearchDirectiveResponse struct {
	ID                 string             `json:"id" description:"Unique directive identifier (e.g., HYP-001)"`
	BusinessHypothesis string             `json:"business_hypothesis" description:"The simple story of the impact"`
	ScienceHypothesis  string             `json:"science_hypothesis" description:"The technical pattern in the data"`
	NullCase           string             `json:"null_case" description:"Quantitative description of a failed/random result"`
	OpportunityAnalysis OpportunityAnalysis `json:"opportunity_analysis" description:"Business impact and strategic value assessment"`
	ValidationMethods  []ValidationMethod `json:"validation_methods" description:"Array of validation methods"`
	RefereeGates       RefereeGates       `json:"referee_gates" description:"Pass/fail validation thresholds"`
	// Legacy fields for backward compatibility
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
	ConfidenceTarget   float64 `json:"confidence_target" description:"Target confidence level (e.g., 0.99)"`
	StabilityThreshold float64 `json:"stability_threshold" description:"Minimum required stability threshold"`
	// Legacy fields for backward compatibility
	PValueThreshold float64 `json:"p_value_threshold,omitempty" description:"Legacy field"`
	StabilityScore  float64 `json:"stability_score,omitempty" description:"Legacy field"`
	PermutationRuns int     `json:"permutation_runs,omitempty" description:"Legacy field"`
}
