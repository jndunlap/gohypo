package analysis

import (
	"time"
)

// ConfidenceLevel represents statistical confidence assessment
type ConfidenceLevel string

const (
	ConfidenceVeryStrong ConfidenceLevel = "very_strong"
	ConfidenceStrong     ConfidenceLevel = "strong"
	ConfidenceModerate   ConfidenceLevel = "moderate"
	ConfidenceWeak       ConfidenceLevel = "weak"
	ConfidenceNegligible ConfidenceLevel = "negligible"
)

// PracticalSignificance represents practical importance of effect size
type PracticalSignificance string

const (
	SignificanceLarge     PracticalSignificance = "large"
	SignificanceMedium    PracticalSignificance = "medium"
	SignificanceSmall     PracticalSignificance = "small"
	SignificanceNegligible PracticalSignificance = "negligible"
)

// AssociationResult represents univariate association analysis
type AssociationResult struct {
	EvidenceID            string               `json:"evidence_id"`
	Feature               string               `json:"feature"`
	Outcome               string               `json:"outcome"`
	ScreeningScore        float64              `json:"screening_score"`
	RawEffect             float64              `json:"raw_effect"`
	Direction             int                  `json:"direction"`
	ConfidenceInterval    [2]float64           `json:"confidence_interval"`
	PValue                float64              `json:"p_value"`
	PValueAdj             float64              `json:"p_value_adj"`
	Method                string               `json:"method"`
	EffectFamily          string               `json:"effect_family"`
	RelationshipForm      string               `json:"relationship_form"`
	ClaimTemplate         string               `json:"claim_template"`
	Coverage              float64              `json:"coverage"`
	NEffective            int                  `json:"n_effective"`
	AssumptionsChecked    bool                 `json:"assumptions_checked"`
	Details               map[string]interface{} `json:"details"`

	// LLM interpretation fields
	StatisticalHypothesis   string               `json:"statistical_hypothesis"`
	ConfidenceLevel         ConfidenceLevel      `json:"confidence_level"`
	PracticalSignificance   PracticalSignificance `json:"practical_significance"`

	// Business-friendly names
	BusinessFeatureName   string `json:"business_feature_name"`
	BusinessOutcomeName   string `json:"business_outcome_name"`
}

// BreakpointResult represents breakpoint/threshold detection
type BreakpointResult struct {
	EvidenceID            string               `json:"evidence_id"`
	Feature               string               `json:"feature"`
	Outcome               string               `json:"outcome"`
	Threshold             float64              `json:"threshold"`
	EffectBelow           float64              `json:"effect_below"`
	EffectAbove           float64              `json:"effect_above"`
	Delta                 float64              `json:"delta"`
	ConfidenceBand        [2]float64           `json:"confidence_band"`
	ThresholdCI           *[2]float64          `json:"threshold_ci,omitempty"`
	PValue                float64              `json:"p_value"`
	PValueAdj             float64              `json:"p_value_adj"`
	Method                string               `json:"method"`
	RelationshipForm      string               `json:"relationship_form"`
	ClaimTemplate         string               `json:"claim_template"`
	Coverage              float64              `json:"coverage"`
	NEffective            int                  `json:"n_effective"`
	AssumptionsChecked    bool                 `json:"assumptions_checked"`
	Details               map[string]interface{} `json:"details"`

	// LLM interpretation fields
	StatisticalHypothesis   string               `json:"statistical_hypothesis"`
	ConfidenceLevel         ConfidenceLevel      `json:"confidence_level"`
	PracticalSignificance   PracticalSignificance `json:"practical_significance"`

	// Business-friendly names
	BusinessFeatureName   string `json:"business_feature_name"`
	BusinessOutcomeName   string `json:"business_outcome_name"`
}

// InteractionResult represents interaction/segment analysis
type InteractionResult struct {
	EvidenceID           string               `json:"evidence_id"`
	Feature              string               `json:"feature"`
	Outcome              string               `json:"outcome"`
	Segmenter            string               `json:"segmenter"`
	GlobalEffect         float64              `json:"global_effect"`
	SegmentEffects       []SegmentEffect      `json:"segment_effects"`
	HeterogeneityP       float64              `json:"heterogeneity_p"`
	HeterogeneityPAdj    float64              `json:"heterogeneity_p_adj"`
	SignFlip             bool                 `json:"sign_flip"`
	RelationshipForm     string               `json:"relationship_form"`
	ClaimTemplate        string               `json:"claim_template"`
	Coverage             float64              `json:"coverage"`
	NEffective           int                  `json:"n_effective"`
	AssumptionsChecked   bool                 `json:"assumptions_checked"`
	Details              map[string]interface{} `json:"details"`

	// LLM interpretation fields
	StatisticalHypothesis   string               `json:"statistical_hypothesis"`
	ConfidenceLevel         ConfidenceLevel      `json:"confidence_level"`
	PracticalSignificance   PracticalSignificance `json:"practical_significance"`

	// Business-friendly names
	BusinessFeatureName   string `json:"business_feature_name"`
	BusinessOutcomeName   string `json:"business_outcome_name"`
	BusinessSegmenterName string `json:"business_segmenter_name"`
}

// SegmentEffect represents effect within a specific segment
type SegmentEffect struct {
	Segment   string  `json:"segment"`
	Effect    float64 `json:"effect"`
	N         int     `json:"n"`
	Direction int     `json:"direction"`
}

// StructuralBreakResult represents temporal structural breaks
type StructuralBreakResult struct {
	EvidenceID            string               `json:"evidence_id"`
	TimeVariable          string               `json:"time_variable"`
	Feature               string               `json:"feature"`
	Outcome               string               `json:"outcome"`
	BreakPoint            float64              `json:"break_point"`
	EffectBefore          float64              `json:"effect_before"`
	EffectAfter           float64              `json:"effect_after"`
	ChowFStatistic        float64              `json:"chow_f_statistic"`
	PValue                float64              `json:"p_value"`
	PValueAdj             float64              `json:"p_value_adj"`
	Method                string               `json:"method"`
	RelationshipForm      string               `json:"relationship_form"`
	ClaimTemplate         string               `json:"claim_template"`
	Coverage              float64              `json:"coverage"`
	NEffective            int                  `json:"n_effective"`
	AssumptionsChecked    bool                 `json:"assumptions_checked"`
	Details               map[string]interface{} `json:"details"`

	// LLM interpretation fields
	StatisticalHypothesis   string               `json:"statistical_hypothesis"`
	ConfidenceLevel         ConfidenceLevel      `json:"confidence_level"`
	PracticalSignificance   PracticalSignificance `json:"practical_significance"`

	// Business-friendly names
	BusinessFeatureName   string `json:"business_feature_name"`
	BusinessOutcomeName   string `json:"business_outcome_name"`
}

// TransferEntropyResult represents causal direction analysis
type TransferEntropyResult struct {
	EvidenceID            string               `json:"evidence_id"`
	SourceVariable        string               `json:"source_variable"`
	TargetVariable        string               `json:"target_variable"`
	TransferEntropy       float64              `json:"transfer_entropy"`
	PValue                float64              `json:"p_value"`
	PValueAdj             float64              `json:"p_value_adj"`
	Method                string               `json:"method"`
	RelationshipForm      string               `json:"relationship_form"`
	ClaimTemplate         string               `json:"claim_template"`
	Coverage              float64              `json:"coverage"`
	NEffective            int                  `json:"n_effective"`
	AssumptionsChecked    bool                 `json:"assumptions_checked"`
	Details               map[string]interface{} `json:"details"`

	// LLM interpretation fields
	StatisticalHypothesis   string               `json:"statistical_hypothesis"`
	ConfidenceLevel         ConfidenceLevel      `json:"confidence_level"`
	PracticalSignificance   PracticalSignificance `json:"practical_significance"`

	// Business-friendly names
	BusinessSourceName    string `json:"business_source_name"`
	BusinessTargetName    string `json:"business_target_name"`
}

// HysteresisResult represents system memory/path dependence
type HysteresisResult struct {
	EvidenceID            string               `json:"evidence_id"`
	Feature               string               `json:"feature"`
	Outcome               string               `json:"outcome"`
	HysteresisStrength    float64              `json:"hysteresis_strength"`
	ScarTissueEffect      float64              `json:"scar_tissue_effect"`
	PValue                float64              `json:"p_value"`
	PValueAdj             float64              `json:"p_value_adj"`
	Method                string               `json:"method"`
	RelationshipForm      string               `json:"relationship_form"`
	ClaimTemplate         string               `json:"claim_template"`
	Coverage              float64              `json:"coverage"`
	NEffective            int                  `json:"n_effective"`
	AssumptionsChecked    bool                 `json:"assumptions_checked"`
	Details               map[string]interface{} `json:"details"`

	// LLM interpretation fields
	StatisticalHypothesis   string               `json:"statistical_hypothesis"`
	ConfidenceLevel         ConfidenceLevel      `json:"confidence_level"`
	PracticalSignificance   PracticalSignificance `json:"practical_significance"`

	// Business-friendly names
	BusinessFeatureName   string `json:"business_feature_name"`
	BusinessOutcomeName   string `json:"business_outcome_name"`
}

// EvidenceBrief orchestrates multiple statistical senses into business narratives
type EvidenceBrief struct {
	// Core metadata
	Version            string    `json:"version"`
	Timestamp          time.Time `json:"timestamp"`
	DatasetName        string    `json:"dataset_name"`
	RowCount           int       `json:"row_count"`
	ColumnCount        int       `json:"column_count"`

	// Business column mappings
	BusinessColumnNames map[string]string `json:"business_column_names"`
	OutcomeColumn       string            `json:"outcome_column"`
	AllowedVariables    []string          `json:"allowed_variables"`
	ExcludedVariables   map[string]string `json:"excluded_variables"`

	// Evidence collections
	Associations      []AssociationResult      `json:"associations"`
	Breakpoints       []BreakpointResult       `json:"breakpoints"`
	Interactions      []InteractionResult      `json:"interactions"`
	StructuralBreaks  []StructuralBreakResult  `json:"structural_breaks"`
	TransferEntropies []TransferEntropyResult `json:"transfer_entropies"`
	HysteresisEffects []HysteresisResult       `json:"hysteresis_effects"`

	// LLM context for hypothesis generation
	LLMContext LLMContext `json:"llm_context"`
}

// LLMContext provides comprehensive context for LLM hypothesis generation
type LLMContext struct {
	Purpose                    string                 `json:"purpose"`
	BoardroomHypothesisGeneration BoardroomGuidance    `json:"boardroom_hypothesis_generation"`
	EvidenceInterpretationGuide EvidenceInterpretation `json:"evidence_interpretation_guide"`
	HypothesisTemplates       HypothesisTemplates    `json:"hypothesis_templates"`
	BoardroomNarrativeExamples BoardroomExamples      `json:"boardroom_narrative_examples"`
	EvidenceDrivenConstraints EvidenceConstraints    `json:"evidence_driven_constraints"`
	DatasetContext            DatasetContext         `json:"dataset_context"`
}

// BoardroomGuidance for executive communication
type BoardroomGuidance struct {
	ExecutiveSummary         string `json:"executive_summary"`
	BusinessImpactFocus      string `json:"business_impact_focus"`
	ConfidenceBasedPrioritization string `json:"confidence_based_prioritization"`
	NarrativeStyle           string `json:"narrative_style"`
	EvidenceCitation         string `json:"evidence_citation"`
}

// EvidenceInterpretation guidelines
type EvidenceInterpretation struct {
	ConfidenceLevels      map[string]string `json:"confidence_levels"`
	PracticalSignificance map[string]string `json:"practical_significance"`
	CausalityLanguage     string            `json:"causality_language"`
}

// HypothesisTemplates for different evidence types
type HypothesisTemplates struct {
	SingleEvidence   string `json:"single_evidence"`
	MultiEvidence    string `json:"multi_evidence"`
	SegmentSpecific  string `json:"segment_specific"`
	TemporalBreak    string `json:"temporal_break"`
	SystemMemory     string `json:"system_memory"`
}

// BoardroomExamples for communication style
type BoardroomExamples struct {
	Correlation  string `json:"correlation"`
	Interaction  string `json:"interaction"`
	Temporal     string `json:"temporal"`
	Hysteresis   string `json:"hysteresis"`
}

// EvidenceConstraints for validation
type EvidenceConstraints struct {
	CiteEvidenceIDs      string `json:"cite_evidence_ids"`
	StatisticalGrounding string `json:"statistical_grounding"`
	BusinessNamesOnly    string `json:"business_names_only"`
	EvidenceScopeOnly    string `json:"evidence_scope_only"`
	ActionableFocus      string `json:"actionable_focus"`
}

// DatasetContext for analysis scope
type DatasetContext struct {
	RowUnit     string `json:"row_unit"`
	Population  string `json:"population"`
	TimeScope   string `json:"time_scope"`
}
