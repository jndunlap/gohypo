package stats

import (
	"time"
)

// EValue represents an evidence value with confidence bounds
type EValue struct {
	Value        float64   `json:"value"`
	Normalized   float64   `json:"normalized"` // 0.0-1.0 scale for UX
	Confidence   float64   `json:"confidence"` // 0.0 to 1.0
	LowerBound   float64   `json:"lower_bound"`
	UpperBound   float64   `json:"upper_bound"`
	TestType     string    `json:"test_type"`
	CalculatedAt time.Time `json:"calculated_at"`
}

// EvidenceCombination represents combined evidence from multiple tests
type EvidenceCombination struct {
	CombinedEValue     float64           `json:"combined_e_value"`
	NormalizedEValue   float64           `json:"normalized_e_value"` // 0.0-1.0 scale
	QualityRating      HypothesisQuality `json:"quality_rating"`
	TestCount          int               `json:"test_count"`
	CorrelationFactor  float64           `json:"correlation_factor"`
	Confidence         float64           `json:"confidence"`
	EarlyStopEligible  bool              `json:"early_stop_eligible"`
	Verdict            HypothesisVerdict `json:"verdict"`
	IndividualResults  []EValue          `json:"individual_results"`
	SelectionRationale string            `json:"selection_rationale"`
}

// HypothesisVerdict represents the final decision
type HypothesisVerdict string

const (
	VerdictAccepted     HypothesisVerdict = "ACCEPTED"
	VerdictRejected     HypothesisVerdict = "REJECTED"
	VerdictInconclusive HypothesisVerdict = "INCONCLUSIVE"
	VerdictEarlyStop    HypothesisVerdict = "EARLY_STOP"
)

// HypothesisQuality represents evidence strength on 0-1 scale
type HypothesisQuality string

const (
	QualityVeryWeak   HypothesisQuality = "Very Weak"
	QualityWeak       HypothesisQuality = "Weak"
	QualityModerate   HypothesisQuality = "Moderate"
	QualityStrong     HypothesisQuality = "Strong"
	QualityVeryStrong HypothesisQuality = "Very Strong"
)

// HypothesisProfile represents characteristics for test selection
type HypothesisProfile struct {
	DataComplexity  DataComplexityScore
	EffectMagnitude EffectSizeCategory
	SampleSize      SampleSizeCategory
	DomainRisk      DomainRiskLevel
	TemporalNature  TemporalComplexity
	ConfoundingRisk ConfoundingAssessment
	PriorEvidence   []ExistingRelationship
}

// DataComplexityScore represents how complex the data structure is
type DataComplexityScore int

const (
	DataComplexitySimple DataComplexityScore = iota
	DataComplexityModerate
	DataComplexityComplex
)

// EffectSizeCategory represents the magnitude of effects
type EffectSizeCategory int

const (
	EffectSizeSmall EffectSizeCategory = iota
	EffectSizeMedium
	EffectSizeLarge
)

// SampleSizeCategory represents the sample size tier
type SampleSizeCategory int

const (
	SampleSizeSmall  SampleSizeCategory = iota // < 100
	SampleSizeMedium                           // 100-1000
	SampleSizeLarge                            // > 1000
)

// DomainRiskLevel represents the stakes of the domain
type DomainRiskLevel int

const (
	DomainRiskLow DomainRiskLevel = iota
	DomainRiskMedium
	DomainRiskHigh
	DomainRiskCritical
)

// TemporalComplexity represents time-based complexity
type TemporalComplexity int

const (
	TemporalStatic TemporalComplexity = iota
	TemporalSimple
	TemporalComplex
)

// ConfoundingAssessment represents confounding variable risk
type ConfoundingAssessment int

const (
	ConfoundingLow ConfoundingAssessment = iota
	ConfoundingMedium
	ConfoundingHigh
)

// ExistingRelationship represents prior statistical findings
type ExistingRelationship struct {
	CauseKey    string
	EffectKey   string
	Strength    float64
	TestType    string
	ValidatedAt time.Time
}

// SelectedTest represents a dynamically chosen validation test
type SelectedTest struct {
	RefereeName    string
	Category       RefereeCategory
	Priority       TestPriority
	Rationale      string
	ExpectedEValue float64
}

// TestPriority represents selection priority
type TestPriority int

const (
	PriorityLow TestPriority = iota
	PriorityMedium
	PriorityHigh
	PriorityCritical
)

// SelectionRationale explains why specific tests were chosen
type SelectionRationale struct {
	RiskLevel         HypothesisRiskLevel
	CategoryCoverage  map[RefereeCategory]float64
	EfficiencyScore   float64
	ExpectedThreshold float64
	TestCount         int
	MinTests          int
	MaxTests          int
}

// HypothesisRiskLevel represents overall hypothesis risk
type HypothesisRiskLevel int

const (
	RiskLevelLow HypothesisRiskLevel = iota
	RiskLevelMedium
	RiskLevelHigh
	RiskLevelVeryHigh
)

// RefereeCategory represents the 10 categories of statistical validation
type RefereeCategory string

const (
	CategorySHREDDER        RefereeCategory = "SHREDDER"
	CategoryDIRECTIONAL     RefereeCategory = "DIRECTIONAL"
	CategoryINVARIANCE      RefereeCategory = "INVARIANCE"
	CategoryANTI_CONFOUNDER RefereeCategory = "ANTI_CONFOUNDER"
	CategoryMECHANISM       RefereeCategory = "MECHANISM"
	CategorySENSITIVITY     RefereeCategory = "SENSITIVITY"
	CategoryTOPOLOGICAL     RefereeCategory = "TOPOLOGICAL"
	CategoryTHERMODYNAMIC   RefereeCategory = "THERMODYNAMIC"
	CategoryCOUNTERFACTUAL  RefereeCategory = "COUNTERFACTUAL"
	CategorySPECTRAL        RefereeCategory = "SPECTRAL"
)
