package referee

import (
	"fmt"
	"strings"
)

// RefereeResult provides PhD-level metadata for Meta-Analysis
type RefereeResult struct {
	GateName      string  `json:"gate_name"`
	Passed        bool    `json:"passed"`
	Statistic     float64 `json:"statistic"`
	PValue        float64 `json:"p_value"`
	StandardUsed  string  `json:"standard_used"`
	FailureReason string  `json:"failure_reason,omitempty"`
}

// Referee is the contract all tools must satisfy
type Referee interface {
	Execute(x, y []float64, metadata map[string]interface{}) RefereeResult
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
func EvaluateTriGate(refereeResults []RefereeResult) TriGateResult {
	if len(refereeResults) != 3 {
		return TriGateResult{
			RefereeResults: refereeResults,
			OverallPassed:  false,
			Confidence:     0.0,
			Rationale:      fmt.Sprintf("Invalid number of referees (expected 3, got %d)", len(refereeResults)),
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

	return TriGateResult{
		RefereeResults: refereeResults,
		OverallPassed:  overallPassed,
		Confidence:     confidence,
		Rationale:      rationale,
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
type TriGateResult struct {
	RefereeResults []RefereeResult `json:"referee_results"`
	OverallPassed  bool            `json:"overall_passed"`
	Confidence     float64         `json:"confidence"`
	Rationale      string          `json:"rationale"`
}

// RunTriGate executes three referees from different categories
func RunTriGate(x, y []float64, metadata map[string]interface{}, refereeNames []string) TriGateResult {
	if len(refereeNames) != 3 {
		return TriGateResult{
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
