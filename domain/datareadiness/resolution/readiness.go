package resolution

import (
	"fmt"

	"gohypo/domain/datareadiness/profiling"
)

// ReadinessGate defines statistical readiness requirements
type ReadinessGate struct {
	config GateConfig
}

// GateConfig defines the readiness thresholds
type GateConfig struct {
	MaxMissingRate    float64 `json:"max_missing_rate"`   // Variables with higher missing rate are rejected
	MinVariance       float64 `json:"min_variance"`       // Minimum variance for numeric variables
	MaxCardinality    int     `json:"max_cardinality"`    // Maximum categories for categorical variables
	MinQualityScore   float64 `json:"min_quality_score"`  // Minimum quality score
	RequireTimestamps bool    `json:"require_timestamps"` // Require observed_at semantics
	MinSampleSize     int     `json:"min_sample_size"`    // Minimum sample size for reliable stats
}

// DefaultGateConfig returns sensible defaults for readiness gates
func DefaultGateConfig() GateConfig {
	return GateConfig{
		MaxMissingRate:    0.95,  // Reject variables with >95% missing
		MinVariance:       1e-10, // Reject near-constant variables
		MaxCardinality:    1000,  // Cap categorical variables
		MinQualityScore:   0.3,   // Require at least 30% quality
		RequireTimestamps: true,  // Require temporal semantics
		MinSampleSize:     30,    // Minimum sample size for stats
	}
}

// NewReadinessGate creates a gate with config
func NewReadinessGate(config GateConfig) *ReadinessGate {
	return &ReadinessGate{config: config}
}

// EvaluateReadiness evaluates which variables are ready for statistical analysis
func (g *ReadinessGate) EvaluateReadiness(profiles []profiling.FieldProfile) ReadinessResult {
	result := ReadinessResult{
		TotalVariables: len(profiles),
	}

	for _, profile := range profiles {
		evaluation := g.evaluateProfile(profile)

		if evaluation.Ready {
			result.ReadyVariables = append(result.ReadyVariables, evaluation)
		} else {
			result.RejectedVariables = append(result.RejectedVariables, evaluation)
		}
	}

	result.ReadyCount = len(result.ReadyVariables)
	result.RejectedCount = len(result.RejectedVariables)

	return result
}

// evaluateProfile evaluates a single profile against readiness criteria
func (g *ReadinessGate) evaluateProfile(profile profiling.FieldProfile) VariableEvaluation {
	eval := VariableEvaluation{
		VariableKey: profile.FieldKey,
		Source:      profile.Source,
		Profile:     profile,
		Ready:       true,
		Rejections:  make([]RejectionReason, 0),
	}

	// Check sample size
	if profile.SampleSize < g.config.MinSampleSize {
		eval.Rejections = append(eval.Rejections, RejectionReason{
			Rule:     "insufficient_sample_size",
			Message:  fmt.Sprintf("Sample size %d < minimum %d", profile.SampleSize, g.config.MinSampleSize),
			Severity: "error",
		})
		eval.Ready = false
	}

	// Check quality score
	if profile.QualityScore < g.config.MinQualityScore {
		eval.Rejections = append(eval.Rejections, RejectionReason{
			Rule:     "low_quality_score",
			Message:  fmt.Sprintf("Quality score %.2f < minimum %.2f", profile.QualityScore, g.config.MinQualityScore),
			Severity: "error",
		})
		eval.Ready = false
	}

	// Check missing rate
	if profile.MissingStats.MissingRate > g.config.MaxMissingRate {
		eval.Rejections = append(eval.Rejections, RejectionReason{
			Rule: "excessive_missing_rate",
			Message: fmt.Sprintf("Missing rate %.1f%% > maximum %.1f%%",
				profile.MissingStats.MissingRate*100, g.config.MaxMissingRate*100),
			Severity: "error",
		})
		eval.Ready = false
	}

	// Check variance for numeric variables
	if profile.InferredType == profiling.TypeNumeric && profile.TypeSpecific.NumericStats != nil {
		stats := profile.TypeSpecific.NumericStats
		if stats.StdDev < g.config.MinVariance {
			eval.Rejections = append(eval.Rejections, RejectionReason{
				Rule:     "insufficient_variance",
				Message:  fmt.Sprintf("Standard deviation %.2e < minimum %.2e", stats.StdDev, g.config.MinVariance),
				Severity: "warning", // Warning because might be intentional (constants)
			})
			// Don't mark as not ready for low variance - it's a modeling choice
		}
	}

	// Check cardinality for categorical variables
	if profile.InferredType == profiling.TypeCategorical {
		if profile.Cardinality.UniqueCount > g.config.MaxCardinality {
			eval.Rejections = append(eval.Rejections, RejectionReason{
				Rule:     "excessive_cardinality",
				Message:  fmt.Sprintf("Unique values %d > maximum %d", profile.Cardinality.UniqueCount, g.config.MaxCardinality),
				Severity: "error",
			})
			eval.Ready = false
		}
	}

	// Check temporal requirements
	if g.config.RequireTimestamps && !profile.TemporalStats.HasTemporalUpdates {
		eval.Rejections = append(eval.Rejections, RejectionReason{
			Rule:     "missing_temporal_semantics",
			Message:  "Variable lacks observed_at semantics for temporal analysis",
			Severity: "error",
		})
		eval.Ready = false
	}

	// Check for unknown types
	if profile.InferredType == profiling.TypeUnknown {
		eval.Rejections = append(eval.Rejections, RejectionReason{
			Rule:     "unknown_type",
			Message:  "Could not determine variable type from sample data",
			Severity: "error",
		})
		eval.Ready = false
	}

	return eval
}

// ApplyRemediation applies automatic fixes to marginally acceptable variables
func (g *ReadinessGate) ApplyRemediation(evaluation VariableEvaluation) VariableEvaluation {
	remediated := evaluation

	// For categorical variables with high cardinality, suggest bucketing
	if evaluation.Profile.InferredType == profiling.TypeCategorical &&
		evaluation.Profile.Cardinality.UniqueCount > g.config.MaxCardinality/2 {

		remediated.Remediation = append(remediated.Remediation, RemediationAction{
			Action:    "bucket_rare_categories",
			Message:   "Bucket categories appearing < 5% into 'other' category",
			Automated: true,
		})
	}

	// For variables with borderline quality, suggest imputation improvements
	if evaluation.Profile.QualityScore >= g.config.MinQualityScore*0.8 &&
		evaluation.Profile.QualityScore < g.config.MinQualityScore {

		remediated.Remediation = append(remediated.Remediation, RemediationAction{
			Action:    "improve_imputation",
			Message:   "Consider better imputation strategy or data collection",
			Automated: false,
		})
	}

	return remediated
}

// ReadinessResult contains the outcome of readiness evaluation
type ReadinessResult struct {
	TotalVariables    int                  `json:"total_variables"`
	ReadyCount        int                  `json:"ready_count"`
	RejectedCount     int                  `json:"rejected_count"`
	ReadyVariables    []VariableEvaluation `json:"ready_variables"`
	RejectedVariables []VariableEvaluation `json:"rejected_variables"`
}

// VariableEvaluation contains the evaluation of a single variable
type VariableEvaluation struct {
	VariableKey string                 `json:"variable_key"`
	Source      string                 `json:"source"`
	Profile     profiling.FieldProfile `json:"profile"`
	Ready       bool                   `json:"ready"`
	Rejections  []RejectionReason      `json:"rejections,omitempty"`
	Remediation []RemediationAction    `json:"remediation,omitempty"`
}

// RejectionReason explains why a variable was rejected
type RejectionReason struct {
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warning"
}

// RemediationAction suggests how to fix issues
type RemediationAction struct {
	Action    string `json:"action"`
	Message   string `json:"message"`
	Automated bool   `json:"automated"`
}

// Summary provides a human-readable summary of readiness results
func (r *ReadinessResult) Summary() string {
	return fmt.Sprintf("Readiness Evaluation: %d/%d variables ready (%.1f%%)",
		r.ReadyCount, r.TotalVariables, float64(r.ReadyCount)/float64(r.TotalVariables)*100)
}

// GetRejectedReasons returns a summary of rejection reasons
func (r *ReadinessResult) GetRejectedReasons() map[string]int {
	reasons := make(map[string]int)

	for _, rejected := range r.RejectedVariables {
		for _, rejection := range rejected.Rejections {
			reasons[rejection.Rule]++
		}
	}

	return reasons
}
