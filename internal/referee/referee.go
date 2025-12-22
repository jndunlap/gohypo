package referee

import (
	"fmt"
	"strings"

	"gohypo/domain/stats"
	"gohypo/models"
)

// Type aliases to use models types (avoids import cycles)
type RefereeResult = models.RefereeResult
type TriGateResult = models.TriGateResult

// DiscoveryEvidence represents pre-computed statistical evidence from the discovery phase
type DiscoveryEvidence struct {
	CauseKey         string  // Variable name for the cause
	EffectKey        string  // Variable name for the effect
	TestType         string  // Type of statistical test performed
	PValue           float64 // Raw p-value from discovery
	QValue           float64 // FDR-corrected q-value
	SampleSize       int     // Sample size used in discovery
	TotalComparisons int     // Total comparisons made (for FDR context)
	FDRMethod        string  // FDR correction method used
}

// Referee is the contract all tools must satisfy
type Referee interface {
	// Execute performs statistical testing from scratch (legacy method - deprecated)
	// WARNING: This method ignores discovery evidence and may violate statistical rigor
	Execute(x, y []float64, metadata map[string]interface{}) RefereeResult

	// AuditEvidence performs evidence auditing with Q-value continuity (preferred method)
	// This method receives pre-computed discovery evidence and performs validation
	// rather than re-computing statistics from raw data
	AuditEvidence(discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) RefereeResult
}

// DefaultAuditEvidence provides a fallback implementation for referees without specific audit logic
func DefaultAuditEvidence(gateName string, discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) models.RefereeResult {
	// Extract discovery evidence
	var evidence DiscoveryEvidence
	if ev, ok := discoveryEvidence.(DiscoveryEvidence); ok {
		evidence = ev
	} else {
		return RefereeResult{
			GateName:      gateName,
			Passed:        false,
			FailureReason: "Invalid discovery evidence format",
		}
	}

	// Basic E-value conversion from q-value
	eValue := 1.0 / evidence.QValue

	// Conservative threshold for unspecified referees
	passed := evidence.QValue <= 0.01

	failureReason := ""
	if !passed {
		failureReason = fmt.Sprintf("Evidence audit failed (q=%.4f). Referee %s requires q≤0.01 for validation.", evidence.QValue, gateName)
	}

	return RefereeResult{
		GateName:      gateName,
		Passed:        passed,
		Statistic:     eValue,
		PValue:        evidence.QValue,
		EValue:        eValue,
		StandardUsed:  fmt.Sprintf("Evidence audit (q ≤ 0.01) with E-value calibration"),
		FailureReason: failureReason,
	}
}

// GetRefereeByName acts as the factory for the Deca-Gate
func GetRefereeByName(name string) Referee {
	switch name {
	case "Permutation_Shuffling":
		return &Shredder{}
	case "Transfer_Entropy":
		return &TransferEntropy{}
	case "Chow_Stability_Test":
		return &ChowTest{}
	case "Conditional_Mutual_Information":
		return &ConditionalMI{}
	case "Monotonicity_Stress_Test":
		return &MonotonicityTest{}
	case "Leave_One_Out_CV":
		return &LeaveOneOutCV{}
	case "Persistent_Homology":
		return &PersistentHomology{}
	case "Lempel_Ziv_Complexity":
		return &LempelZivComplexity{}
	case "Synthetic_Intervention":
		return &SyntheticIntervention{}
	case "Wavelet_Coherence":
		return &WaveletCoherence{}
	default:
		return nil
	}
}

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

// GetCategoryForReferee returns the category for a referee name
func GetCategoryForReferee(name string) RefereeCategory {
	// Normalize to lowercase for case-insensitive matching
	normalized := strings.ToLower(strings.TrimSpace(name))

	switch normalized {
	case "permutation_shuffling", "shredder", "statistical_integrity", "permutation_shredder":
		return CategorySHREDDER
	case "transfer_entropy", "directional_causality", "convergent_cross_mapping", "ccm":
		return CategoryDIRECTIONAL
	case "chow_stability_test", "invariance", "structural_stability", "cusum_drift_detection":
		return CategoryINVARIANCE
	case "conditional_mutual_information", "conditional_mi", "cmi", "partial_correlation":
		return CategoryANTI_CONFOUNDER
	case "monotonicity_stress_test", "isotonic_mechanism", "isotonic_mechanism_check", "functional_form_test":
		return CategoryMECHANISM
	case "leave_one_out_cv", "loo_cross_validation", "alpha_decay_test":
		return CategorySENSITIVITY
	case "persistent_homology", "topological_analysis", "topological_data_analysis":
		return CategoryTOPOLOGICAL
	case "algorithmic_complexity", "compression_complexity", "lempel_ziv_complexity":
		return CategoryTHERMODYNAMIC
	case "synthetic_intervention", "g_computation":
		return CategoryCOUNTERFACTUAL
	case "wavelet_coherence", "spectral_analysis":
		return CategorySPECTRAL
	default:
		return ""
	}
}

// EvaluateTriGate evaluates the results of three referees for Tri-Gate validation
func EvaluateTriGate(refereeResults []models.RefereeResult) models.TriGateResult {
	if len(refereeResults) != 3 {
		return models.TriGateResult{
			RefereeResults:   refereeResults,
			OverallPassed:    false,
			Confidence:       0.0,
			NormalizedEValue: 0.0,
			QualityRating:    string(stats.QualityVeryWeak),
			Rationale:        fmt.Sprintf("Invalid number of referees (expected 3, got %d)", len(refereeResults)),
		}
	}

	passedCount := 0
	failedReferees := []string{}

	for _, result := range refereeResults {
		if result.Passed {
			passedCount++
		} else {
			failedReferees = append(failedReferees, result.GateName)
		}
	}

	overallPassed := passedCount == 3
	confidence := float64(passedCount) / 3.0

	var rationale string
	if overallPassed {
		rationale = "All three referees passed validation - hypothesis promoted to Universal Law"
	} else {
		rationale = fmt.Sprintf("Hypothesis failed validation at %d referee(s): %v",
			len(failedReferees), failedReferees)
	}

	// Calculate normalized E-value from confidence (simplified approach)
	// In a full implementation, this would use the E-value calibrator
	normalizedEValue := confidence // For now, use confidence as proxy

	// Determine quality rating based on normalized value
	var qualityRating stats.HypothesisQuality
	switch {
	case normalizedEValue >= 0.8:
		qualityRating = stats.QualityVeryStrong
	case normalizedEValue >= 0.6:
		qualityRating = stats.QualityStrong
	case normalizedEValue >= 0.4:
		qualityRating = stats.QualityModerate
	case normalizedEValue >= 0.2:
		qualityRating = stats.QualityWeak
	default:
		qualityRating = stats.QualityVeryWeak
	}

	return TriGateResult{
		RefereeResults:   refereeResults,
		OverallPassed:    overallPassed,
		Confidence:       confidence,
		NormalizedEValue: normalizedEValue,
		QualityRating:    string(qualityRating),
		Rationale:        rationale,
	}
}

// ValidateData performs basic validation on input data
func ValidateData(x, y []float64) error {
	if len(x) != len(y) {
		return fmt.Errorf("x and y must have same length")
	}
	if len(x) < 10 {
		return fmt.Errorf("insufficient data points (minimum 10 required)")
	}
	return nil
}

// TriGateResult represents the result of running three referees
// TriGateResult uses the type from models package

// RunTriGate executes three referees from different categories
func RunTriGate(x, y []float64, metadata map[string]interface{}, refereeNames []string) models.TriGateResult {
	if len(refereeNames) != 3 {
		return models.TriGateResult{
			OverallPassed: false,
			Confidence:    0.0,
		}
	}

	results := make([]RefereeResult, 3)
	passedCount := 0

	for i, name := range refereeNames {
		referee := GetRefereeByName(name)
		if referee == nil {
			results[i] = RefereeResult{
				GateName:      name,
				Passed:        false,
				FailureReason: "Referee not found",
			}
			continue
		}

		result := referee.Execute(x, y, metadata)
		results[i] = result

		if result.Passed {
			passedCount++
		}
	}

	// Require all three referees to pass for overall success
	overallPassed := passedCount == 3
	confidence := float64(passedCount) / 3.0

	return TriGateResult{
		RefereeResults: results,
		OverallPassed:  overallPassed,
		Confidence:     confidence,
	}
}
