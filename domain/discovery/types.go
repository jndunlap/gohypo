package discovery

import (
	"gohypo/domain/core"
)

// ============================================================================
// DISCOVERY BRIEF - LLM Context Generation
// ============================================================================

// DiscoveryBrief encapsulates all statistical senses for LLM reasoning
// This is the "narrative bridge" between Go statistical engine and LLM prompts
type DiscoveryBrief struct {
	// Core metadata
	SnapshotID   core.SnapshotID  `json:"snapshot_id"`
	RunID        core.RunID       `json:"run_id"`
	VariableKey  core.VariableKey `json:"variable_key"`
	DiscoveredAt core.Timestamp   `json:"discovered_at"`

	// Statistical senses (Five Mathematical Senses Engine)
	MutualInformation MutualInformationSense `json:"mutual_information"`
	WelchsTTest       WelchsTTestSense       `json:"welchs_t_test"`
	ChiSquare         ChiSquareSense         `json:"chi_square"`
	Spearman          SpearmanSense          `json:"spearman"`
	CrossCorrelation  CrossCorrelationSense  `json:"cross_correlation"`

	// Behavioral narratives (pattern recognition seeds)
	SilenceAcceleration SilenceAcceleration `json:"silence_acceleration"`
	BlastRadius         BlastRadius         `json:"blast_radius"`
	TwinSegments        TwinSegments        `json:"twin_segments"`

	// Confidence and risk assessment
	ConfidenceScore float64       `json:"confidence_score"` // 0.0-1.0 overall confidence
	RiskAssessment  RiskLevel     `json:"risk_assessment"`  // Low, Medium, High
	WarningFlags    []WarningFlag `json:"warning_flags,omitempty"`

	// Context for LLM generation
	LLMContext LLMContext `json:"llm_context"`
}

// ============================================================================
// FIVE STATISTICAL SENSES
// ============================================================================

// MutualInformationSense detects non-linear relationships that Pearson misses
type MutualInformationSense struct {
	MIValue       float64 `json:"mi_value"`      // Mutual information value (bits)
	NormalizedMI  float64 `json:"normalized_mi"` // 0.0-1.0 normalized MI
	PValue        float64 `json:"p_value"`       // Statistical significance
	SampleSize    int     `json:"sample_size"`
	EntropyX      float64 `json:"entropy_x"`                // Entropy of variable X
	EntropyY      float64 `json:"entropy_y"`                // Entropy of variable Y
	ConditionalMI float64 `json:"conditional_mi,omitempty"` // Given confounders
}

// WelchsTTestSense identifies behavioral differences between groups
type WelchsTTestSense struct {
	TStatistic      float64 `json:"t_statistic"`
	DegreesFreedom  float64 `json:"degrees_freedom"`
	PValue          float64 `json:"p_value"`
	EffectSize      float64 `json:"effect_size"` // Cohen's d
	Group1Mean      float64 `json:"group_1_mean"`
	Group1Size      int     `json:"group_1_size"`
	Group2Mean      float64 `json:"group_2_mean"`
	Group2Size      int     `json:"group_2_size"`
	SampleSize      int     `json:"sample_size"`     // Total sample size (group1 + group2)
	Heteroscedastic bool    `json:"heteroscedastic"` // Different variances detected
}

// ChiSquareSense finds categorical distribution anomalies
type ChiSquareSense struct {
	ChiSquareStatistic  float64            `json:"chi_square_statistic"`
	DegreesFreedom      int                `json:"degrees_freedom"`
	PValue              float64            `json:"p_value"`
	CramersV            float64            `json:"cramers_v"` // Effect size 0.0-1.0
	ExpectedFrequencies map[string]float64 `json:"expected_frequencies"`
	ObservedFrequencies map[string]int     `json:"observed_frequencies"`
	Residuals           map[string]float64 `json:"residuals"` // Standardized residuals
}

// SpearmanSense handles rank-order relationships robust to outliers
type SpearmanSense struct {
	Correlation     float64 `json:"correlation"` // Spearman's rho (-1.0 to 1.0)
	PValue          float64 `json:"p_value"`
	SampleSize      int     `json:"sample_size"`
	ConcordantPairs int     `json:"concordant_pairs"`
	DiscordantPairs int     `json:"discordant_pairs"`
	TiesX           int     `json:"ties_x"` // Number of ties in X
	TiesY           int     `json:"ties_y"` // Number of ties in Y
}

// CrossCorrelationSense discovers temporal/causal dependencies with lag
type CrossCorrelationSense struct {
	MaxCorrelation    float64          `json:"max_correlation"`    // Peak correlation coefficient
	OptimalLag        int              `json:"optimal_lag"`        // Lag with maximum correlation
	LagRange          int              `json:"lag_range"`          // Range of lags tested
	PValue            float64          `json:"p_value"`            // Significance of max correlation
	Direction         string           `json:"direction"`          // "leads", "lags", "simultaneous"
	CrossCorrelations []LagCorrelation `json:"cross_correlations"` // All lag correlations
}

// LagCorrelation represents correlation at a specific lag
type LagCorrelation struct {
	Lag         int     `json:"lag"`
	Correlation float64 `json:"correlation"`
	PValue      float64 `json:"p_value"`
}

// ============================================================================
// BEHAVIORAL NARRATIVES
// ============================================================================

// SilenceAcceleration detects when variables suddenly stop moving together
type SilenceAcceleration struct {
	Detected         bool    `json:"detected"`
	AccelerationRate float64 `json:"acceleration_rate"`       // Rate of divergence
	SilencePeriod    int     `json:"silence_period"`          // Periods of no relationship
	TriggerEvent     string  `json:"trigger_event,omitempty"` // What might have caused it
	Confidence       float64 `json:"confidence"`              // 0.0-1.0 confidence in detection
}

// BlastRadius measures how much a variable's change affects others
type BlastRadius struct {
	RadiusScore       float64            `json:"radius_score"` // 0.0-1.0 blast radius
	AffectedVariables []core.VariableKey `json:"affected_variables"`
	DominoEffect      bool               `json:"domino_effect"`    // Cascading impacts detected
	CentralityScore   float64            `json:"centrality_score"` // Network centrality
	PathLength        int                `json:"path_length"`      // Longest impact chain
}

// TwinSegments identifies nearly identical behavioral segments
type TwinSegments struct {
	Detected        bool          `json:"detected"`
	SegmentPairs    []SegmentPair `json:"segment_pairs"`
	SimilarityScore float64       `json:"similarity_score"` // 0.0-1.0 average similarity
	RedundancyRisk  string        `json:"redundancy_risk"`  // "low", "medium", "high"
	ConfoundingRisk float64       `json:"confounding_risk"` // Risk of hidden confounding
}

// SegmentPair represents two similar segments
type SegmentPair struct {
	Segment1     core.VariableKey `json:"segment_1"`
	Segment2     core.VariableKey `json:"segment_2"`
	Similarity   float64          `json:"similarity"`    // 0.0-1.0 similarity score
	OverlapCount int              `json:"overlap_count"` // Number of shared relationships
}

// ============================================================================
// CONFIDENCE AND RISK
// ============================================================================

// RiskLevel represents overall risk assessment
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// WarningFlag represents specific concerns
type WarningFlag string

const (
	WarningLowSampleSize        WarningFlag = "low_sample_size"
	WarningHighMissingData      WarningFlag = "high_missing_data"
	WarningOutlierSensitivity   WarningFlag = "outlier_sensitivity"
	WarningNonlinearOnly        WarningFlag = "nonlinear_only"
	WarningTemporalInstability  WarningFlag = "temporal_instability"
	WarningConfoundingSuspected WarningFlag = "confounding_suspected"
)

// ============================================================================
// LLM CONTEXT GENERATION
// ============================================================================

// LLMContext provides structured context for LLM reasoning
type LLMContext struct {
	// Human-readable summaries
	ExecutiveSummary   string   `json:"executive_summary"`
	StatisticalSummary string   `json:"statistical_summary"`
	BehavioralInsights []string `json:"behavioral_insights"`

	// Structured prompts
	HypothesisSeeds []HypothesisSeed `json:"hypothesis_seeds"`
	PromptFragments []PromptFragment `json:"prompt_fragments"`

	// Confidence metadata
	EvidenceStrength   EvidenceStrength `json:"evidence_strength"`
	UncertaintyFactors []string         `json:"uncertainty_factors"`
}

// HypothesisSeed provides creative starting points for LLM
type HypothesisSeed struct {
	Category    string  `json:"category"` // "causal", "confounding", "effect_modification"
	Description string  `json:"description"`
	Priority    float64 `json:"priority"`   // 0.0-1.0 generation priority
	Confidence  float64 `json:"confidence"` // 0.0-1.0 confidence in seed
}

// PromptFragment provides pre-built prompt components
type PromptFragment struct {
	Type        string `json:"type"` // "statistical", "behavioral", "domain"
	Content     string `json:"content"`
	Priority    int    `json:"priority"`              // 1-10 insertion priority
	Conditional string `json:"conditional,omitempty"` // When to include this fragment
}

// EvidenceStrength quantifies the strength of statistical evidence
type EvidenceStrength struct {
	OverallScore     float64            `json:"overall_score"`     // 0.0-1.0 composite score
	SenseScores      map[string]float64 `json:"sense_scores"`      // Score per statistical sense
	ConsistencyScore float64            `json:"consistency_score"` // Agreement between senses
	RobustnessScore  float64            `json:"robustness_score"`  // Resistance to assumptions
}

// ============================================================================
// CONSTRUCTORS AND METHODS
// ============================================================================

// NewDiscoveryBrief creates a new discovery brief for a variable
func NewDiscoveryBrief(snapshotID core.SnapshotID, runID core.RunID, variableKey core.VariableKey) *DiscoveryBrief {
	return &DiscoveryBrief{
		SnapshotID:   snapshotID,
		RunID:        runID,
		VariableKey:  variableKey,
		DiscoveredAt: core.Now(),
		LLMContext: LLMContext{
			BehavioralInsights: []string{},
			HypothesisSeeds:    []HypothesisSeed{},
			PromptFragments:    []PromptFragment{},
			UncertaintyFactors: []string{},
		},
	}
}

// CalculateConfidence computes overall confidence score from all senses
func (db *DiscoveryBrief) CalculateConfidence() float64 {
	// Weight different senses based on their reliability and informativeness
	weights := map[string]float64{
		"mutual_information": 0.25,
		"welchs_t_test":      0.20,
		"chi_square":         0.15,
		"spearman":           0.20,
		"cross_correlation":  0.20,
	}

	senseScores := map[string]float64{
		"mutual_information": db.calculateMIScore(),
		"welchs_t_test":      db.calculateTTestScore(),
		"chi_square":         db.calculateChiSquareScore(),
		"spearman":           db.calculateSpearmanScore(),
		"cross_correlation":  db.calculateCrossCorrScore(),
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for sense, score := range senseScores {
		if weight, exists := weights[sense]; exists && score >= 0 {
			weightedSum += score * weight
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		return 0.0
	}

	db.ConfidenceScore = weightedSum / totalWeight
	return db.ConfidenceScore
}

// Helper methods for calculating individual sense scores
func (db *DiscoveryBrief) calculateMIScore() float64 {
	if db.MutualInformation.SampleSize == 0 {
		return -1.0 // Not calculated
	}
	// Score based on normalized MI and significance
	score := db.MutualInformation.NormalizedMI * 0.7
	if db.MutualInformation.PValue < 0.05 {
		score += 0.3
	}
	return min(score, 1.0)
}

func (db *DiscoveryBrief) calculateTTestScore() float64 {
	if db.WelchsTTest.SampleSize == 0 {
		return -1.0
	}
	// Score based on effect size and significance
	effectScore := min(db.WelchsTTest.EffectSize/2.0, 1.0) // Cohen's d > 2.0 is very large
	sigScore := 0.0
	if db.WelchsTTest.PValue < 0.05 {
		sigScore = 1.0
	} else if db.WelchsTTest.PValue < 0.10 {
		sigScore = 0.5
	}
	return (effectScore + sigScore) / 2.0
}

func (db *DiscoveryBrief) calculateChiSquareScore() float64 {
	if db.ChiSquare.DegreesFreedom == 0 {
		return -1.0
	}
	// Score based on Cramer's V and significance
	score := db.ChiSquare.CramersV * 0.7
	if db.ChiSquare.PValue < 0.05 {
		score += 0.3
	}
	return min(score, 1.0)
}

func (db *DiscoveryBrief) calculateSpearmanScore() float64 {
	if db.Spearman.SampleSize == 0 {
		return -1.0
	}
	// Score based on correlation strength and significance
	corrScore := abs(db.Spearman.Correlation)
	sigScore := 0.0
	if db.Spearman.PValue < 0.05 {
		sigScore = 1.0
	} else if db.Spearman.PValue < 0.10 {
		sigScore = 0.5
	}
	return (corrScore + sigScore) / 2.0
}

func (db *DiscoveryBrief) calculateCrossCorrScore() float64 {
	if len(db.CrossCorrelation.CrossCorrelations) == 0 {
		return -1.0
	}
	// Score based on maximum correlation and significance
	corrScore := abs(db.CrossCorrelation.MaxCorrelation)
	sigScore := 0.0
	if db.CrossCorrelation.PValue < 0.05 {
		sigScore = 1.0
	} else if db.CrossCorrelation.PValue < 0.10 {
		sigScore = 0.5
	}
	return (corrScore + sigScore) / 2.0
}

// AssessRisk determines overall risk level based on confidence and warnings
func (db *DiscoveryBrief) AssessRisk() RiskLevel {
	if db.ConfidenceScore < 0.3 {
		return RiskHigh
	} else if db.ConfidenceScore < 0.7 {
		return RiskMedium
	}

	// High confidence but check for warning flags
	if len(db.WarningFlags) > 2 {
		return RiskMedium
	}

	return RiskLow
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
