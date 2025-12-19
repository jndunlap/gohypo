package referee

import (
	"fmt"
	"strings"
)

// referee_factory.go
// Maps LLM JSON referee selections to dynamic Go implementations
// Ensures all referees use centralized constants for StandardUsed strings

// RefereeConfig holds the configuration for a referee instance
type RefereeConfig struct {
	Name        string
	Category    RefereeCategory
	Description string
}

// GetRefereeFactory returns a configured referee based on LLM selection
func GetRefereeFactory(refereeName string) (Referee, error) {
	switch strings.ToLower(strings.TrimSpace(refereeName)) {

	// SHREDDER Category
	case "permutation_shuffling", "permutation_shredder", "shredder", "statistical_integrity":
		return &Shredder{
			Iterations: SHREDDER_ITERATIONS,
			Alpha:      SHREDDER_P_ALPHA,
		}, nil

	// DIRECTIONAL Category
	case "transfer_entropy", "directional_causality":
		return &TransferEntropy{
			K:       5, // Default kNN neighbors
			TimeLag: CAUSAL_LAG_DEFAULT,
		}, nil

	case "convergent_cross_mapping", "ccm":
		return &ConvergentCrossMapping{}, nil

	// INVARIANCE Category
	case "chow_stability_test", "invariance", "structural_stability":
		return &ChowTest{
			AlphaCritical: CHOW_ALPHA_CRITICAL,
			FCritical:     CHOW_F_CRITICAL,
			TrimFraction:  SUPREMUM_WALD_TRIM,
		}, nil

	case "cusum_drift_detection":
		return &CUSUMDriftDetection{
			ControlLimit: CUSUM_CONTROL_LIMIT,
		}, nil

	// ANTI_CONFOUNDER Category
	case "conditional_mutual_information", "conditional_mi", "cmi":
		return &ConditionalMI{}, nil

	// MECHANISM Category
	case "monotonicity_stress_test", "isotonic_mechanism", "isotonic_mechanism_check":
		return &MonotonicityTest{
			MaxSignFlips:    MECHANISM_SIGN_FLIPS_MAX,
			SpearmanMinimum: SPEARMAN_RHO_MIN,
		}, nil

	// SENSITIVITY Category
	case "leave_one_out_cv", "loo_cross_validation":
		return &LeaveOneOutCV{}, nil

	case "alpha_decay_test":
		return &AlphaDecayTest{
			AlphaStart: ALPHA_DECAY_START,
			AlphaEnd:   ALPHA_DECAY_END,
			MinSamples: SENSITIVITY_MIN_SAMPLES,
		}, nil

	// TOPOLOGICAL Category
	case "persistent_homology", "topological_analysis":
		return &PersistentHomology{}, nil

	// THERMODYNAMIC Category
	case "algorithmic_complexity", "compression_complexity":
		return &AlgorithmicComplexity{}, nil

	// COUNTERFACTUAL Category
	case "synthetic_intervention", "g_computation":
		return &SyntheticIntervention{}, nil

	// SPECTRAL Category
	case "wavelet_coherence", "spectral_analysis":
		return &WaveletCoherence{}, nil

	default:
		return nil, fmt.Errorf("unknown referee: %s", refereeName)
	}
}

// GetRefereeConfigs returns all available referee configurations for UI/display
func GetRefereeConfigs() []RefereeConfig {
	return []RefereeConfig{
		{
			Name:        "Permutation_Shredder",
			Category:    CategorySHREDDER,
			Description: fmt.Sprintf("Two-tailed permutation test (N=%d) with p ≤ %.3f", SHREDDER_ITERATIONS, SHREDDER_P_ALPHA),
		},
		{
			Name:        "Chow_Stability_Test",
			Category:    CategoryINVARIANCE,
			Description: fmt.Sprintf("Supremum Wald F < %.2f (Trim: %.0f%%)", CHOW_F_CRITICAL, SUPREMUM_WALD_TRIM*100),
		},
		{
			Name:        "Transfer_Entropy",
			Category:    CategoryDIRECTIONAL,
			Description: fmt.Sprintf("Information transfer ≥ %.2f bits (lag τ=%d)", MIN_TRANSFER_ENTROPY_BITS, CAUSAL_LAG_DEFAULT),
		},
		{
			Name:        "Convergent_Cross_Mapping",
			Category:    CategoryDIRECTIONAL,
			Description: fmt.Sprintf("Manifold reconstruction ρ ≥ %.2f", CCM_CONVERGENCE_RHO),
		},
		{
			Name:        "Conditional_MI",
			Category:    CategoryANTI_CONFOUNDER,
			Description: fmt.Sprintf("Non-parametric CMI with k=%d neighbors", CMI_K_NEIGHBORS),
		},
		{
			Name:        "Isotonic_Mechanism_Check",
			Category:    CategoryMECHANISM,
			Description: fmt.Sprintf("Derivative consistency (≤ %d sign flips, ρ ≥ %.2f)", MECHANISM_SIGN_FLIPS_MAX, SPEARMAN_RHO_MIN),
		},
		{
			Name:        "LOO_Cross_Validation",
			Category:    CategorySENSITIVITY,
			Description: fmt.Sprintf("Log-loss reduction ≥ %.1f%%", LOO_LOGLOSS_DELTA_MIN*100),
		},
		{
			Name:        "Persistent_Homology",
			Category:    CategoryTOPOLOGICAL,
			Description: fmt.Sprintf("Persistence ratio ≥ %.1f", PERSISTENCE_NOISE_RATIO),
		},
		{
			Name:        "Algorithmic_Complexity",
			Category:    CategoryTHERMODYNAMIC,
			Description: fmt.Sprintf("Compression gain ≥ %.0f%%", THERMO_COMPRESSION_GAIN*100),
		},
		{
			Name:        "Synthetic_Intervention",
			Category:    CategoryCOUNTERFACTUAL,
			Description: fmt.Sprintf("G-computation (σ = %.1f)", SYNTHETIC_INTERVENTION_SIGMA),
		},
		{
			Name:        "Wavelet_Coherence",
			Category:    CategorySPECTRAL,
			Description: fmt.Sprintf("Phase stability variance < %.2f", SPECTRAL_PHASE_STABILITY),
		},
	}
}

// ValidateRefereeCompatibility checks if a set of referees provides adequate coverage
func ValidateRefereeCompatibility(refereeNames []string) error {
	// Check for correct number of referees
	if len(refereeNames) != 3 {
		return fmt.Errorf("exactly 3 referees required, got %d", len(refereeNames))
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, name := range refereeNames {
		if seen[name] {
			return fmt.Errorf("duplicate referee: %s", name)
		}
		seen[name] = true
	}

	categories := make(map[RefereeCategory]bool)

	for _, name := range refereeNames {
		if _, err := GetRefereeFactory(name); err != nil {
			return fmt.Errorf("invalid referee %s: %w", name, err)
		} else {
			// Get category from the referee
			category := GetCategoryForReferee(name)
			if category == "" {
				return fmt.Errorf("unknown category for referee: %s", name)
			}
			categories[category] = true
		}
	}

	// Ensure we have 3 different categories (as per prompt requirement)
	if len(categories) < 3 {
		return fmt.Errorf("referees must be from 3 different categories, got %d unique categories", len(categories))
	}

	// Note: We no longer require specific categories (SHREDDER, INVARIANCE) as the prompt
	// allows the LLM to choose any 3 different categories based on the hypothesis needs.
	// This provides flexibility while still ensuring diverse coverage.

	return nil
}

// ValidateStandardUsed checks if a StandardUsed string contains the correct constants
func ValidateStandardUsed(standardUsed, gateName string) bool {
	switch gateName {
	case "Permutation_Shredder":
		expected := fmt.Sprintf("Two-tailed permutation (N=%d) with p ≤ %.3f", SHREDDER_ITERATIONS, SHREDDER_P_ALPHA)
		return strings.Contains(standardUsed, expected)

	case "Chow_Stability_Test":
		expected := fmt.Sprintf("Supremum Wald F < %.2f", CHOW_F_CRITICAL)
		return strings.Contains(standardUsed, expected)

	case "Transfer_Entropy":
		expected := fmt.Sprintf("Information transfer ≥ %.2f bits", MIN_TRANSFER_ENTROPY_BITS)
		return strings.Contains(standardUsed, expected)

	case "Isotonic_Mechanism":
		expected := fmt.Sprintf("Derivative consistency (≤ %d sign flips", MECHANISM_SIGN_FLIPS_MAX)
		return strings.Contains(standardUsed, expected)

	case "LOO_Cross_Validation":
		expected := fmt.Sprintf("Log-loss reduction ≥ %.1f%%", LOO_LOGLOSS_DELTA_MIN*100)
		return strings.Contains(standardUsed, expected)

	case "Persistent_Homology":
		expected := fmt.Sprintf("Persistence ratio ≥ %.1f", PERSISTENCE_NOISE_RATIO)
		return strings.Contains(standardUsed, expected)

	default:
		return true // For unimplemented referees, don't fail validation
	}
}
