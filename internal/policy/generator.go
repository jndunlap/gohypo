package policy

import (
	"gohypo/internal/profiling"
)

// RefereeConfig contains dynamic configuration for statistical referees
type RefereeConfig struct {
	// Shredder (Permutation Testing) Configuration
	Shredder struct {
		Iterations int     `json:"iterations"`
		Alpha      float64 `json:"significance_level"`
		EarlyExit  bool    `json:"early_exit_enabled"`
	} `json:"shredder"`

	// Invariance (Structural Stability) Configuration
	Invariance struct {
		CriticalValue float64 `json:"critical_value"`
		AdaptiveAlpha bool    `json:"adaptive_alpha"`
		TrimPercent   float64 `json:"trim_percent"`
	} `json:"invariance"`

	// Directional (Causal Flow) Configuration
	Directional struct {
		MaxLag     int     `json:"max_causal_lag"`
		TestType   string  `json:"test_type"` // "parametric" or "nonparametric"
		MinEntropy float64 `json:"min_transfer_entropy"`
	} `json:"directional"`

	// Sampling Strategy Configuration
	Sampling struct {
		BootstrapSize int    `json:"bootstrap_samples"`
		Strategy      string `json:"sampling_strategy"`
		BlockSize     int    `json:"block_size,omitempty"`
	} `json:"sampling"`

	// General Configuration
	General struct {
		UseNonParametric bool    `json:"use_nonparametric"`
		HardFloor        float64 `json:"hard_significance_floor"`
		ConfidenceLevel  float64 `json:"confidence_level"`
	} `json:"general"`
}

// PolicyGenerator creates adaptive referee configurations based on data characteristics
type PolicyGenerator struct{}

// NewPolicyGenerator creates a new policy generator
func NewPolicyGenerator() *PolicyGenerator {
	return &PolicyGenerator{}
}

// GenerateAdaptivePolicy creates a referee configuration based on topology markers
func (pg *PolicyGenerator) GenerateAdaptivePolicy(markers profiling.TopologyMarkers) RefereeConfig {
	config := RefereeConfig{}

	// Set hard floor for significance to prevent p-hacking
	config.General.HardFloor = 0.001
	config.General.ConfidenceLevel = 0.999

	// Adaptive permutation testing based on data characteristics
	pg.configureShredder(&config, markers)

	// Adaptive invariance testing
	pg.configureInvariance(&config, markers)

	// Adaptive directional/causal testing
	pg.configureDirectional(&config, markers)

	// Adaptive sampling strategy
	pg.configureSampling(&config, markers)

	// General configuration flags
	pg.configureGeneral(&config, markers)

	return config
}

// configureShredder sets up adaptive permutation testing parameters
func (pg *PolicyGenerator) configureShredder(config *RefereeConfig, markers profiling.TopologyMarkers) {
	// Base configuration
	config.Shredder.Iterations = 2500 // Default
	config.Shredder.Alpha = 0.001     // Conservative default
	config.Shredder.EarlyExit = true  // Enable early exit by default

	// Adjust for data quality
	if markers.Quality.NoiseCoefficient > 0.7 {
		// High noise: more permutations for reliable results
		config.Shredder.Iterations = 10000
		config.Shredder.EarlyExit = false // Don't exit early with noisy data
	} else if markers.Quality.SparsityRatio > 0.5 {
		// Sparse data: more conservative threshold
		config.Shredder.Alpha = maxFloat64(config.Shredder.Alpha, 0.005)
		config.Shredder.Iterations = 5000
	}

	// Adjust for distribution characteristics
	if !markers.Distribution.IsNormal {
		// Non-normal data: slightly more conservative
		config.Shredder.Alpha = maxFloat64(config.Shredder.Alpha, 0.005)
	}

	// Adjust for stationarity
	if !markers.Temporal.IsStationary {
		// Non-stationary data: more permutations needed
		config.Shredder.Iterations = maxInt(config.Shredder.Iterations, 5000)
	}

	// Enforce hard floor
	config.Shredder.Alpha = maxFloat64(config.Shredder.Alpha, config.General.HardFloor)
}

// configureInvariance sets up structural stability testing parameters
func (pg *PolicyGenerator) configureInvariance(config *RefereeConfig, markers profiling.TopologyMarkers) {
	config.Invariance.CriticalValue = 0.001 // Conservative F-test threshold
	config.Invariance.AdaptiveAlpha = true
	config.Invariance.TrimPercent = 0.15 // Standard 15% trim

	// Adjust for data characteristics
	if markers.Quality.SparsityRatio > 0.7 {
		// Very sparse data: more aggressive trimming
		config.Invariance.TrimPercent = 0.20
	}

	if !markers.Temporal.IsStationary {
		// Non-stationary data: less conservative threshold (easier to detect breaks)
		config.Invariance.CriticalValue = 0.01
	}
}

// configureDirectional sets up causal direction testing parameters
func (pg *PolicyGenerator) configureDirectional(config *RefereeConfig, markers profiling.TopologyMarkers) {
	config.Directional.MinEntropy = 0.01 // Minimum transfer entropy
	config.Directional.TestType = "parametric"

	// Set max lag based on suggested lags or defaults
	if len(markers.Temporal.SuggestedLags) > 0 {
		maxSuggested := 0
		for _, lag := range markers.Temporal.SuggestedLags {
			if lag > maxSuggested {
				maxSuggested = lag
			}
		}
		config.Directional.MaxLag = maxSuggested + 2 // Add buffer
	} else {
		config.Directional.MaxLag = 5 // Default
	}

	// Cap at reasonable maximum
	if config.Directional.MaxLag > 20 {
		config.Directional.MaxLag = 20
	}

	// Use nonparametric tests for non-normal data
	if !markers.Distribution.IsNormal {
		config.Directional.TestType = "nonparametric"
	}
}

// configureSampling sets up bootstrap and sampling parameters
func (pg *PolicyGenerator) configureSampling(config *RefereeConfig, markers profiling.TopologyMarkers) {
	config.Sampling.BootstrapSize = 1000 // Default
	config.Sampling.Strategy = "independent"

	// Adjust for data dependencies
	if markers.Temporal.AutocorrLag1 > 0.3 {
		// Strong autocorrelation: use block bootstrap
		config.Sampling.Strategy = "block"
		config.Sampling.BlockSize = max(5, int(float64(len(markers.Temporal.SuggestedLags))*1.5))
	}

	// Adjust for data quality
	if markers.Quality.NoiseCoefficient > 0.8 {
		// Very noisy data: more bootstrap samples
		config.Sampling.BootstrapSize = 2000
	}
}

// configureGeneral sets general configuration flags
func (pg *PolicyGenerator) configureGeneral(config *RefereeConfig, markers profiling.TopologyMarkers) {
	// Use nonparametric methods for non-normal distributions
	config.General.UseNonParametric = !markers.Distribution.IsNormal

	// Categorical data considerations
	if markers.Categorical.IsCategorical {
		config.General.UseNonParametric = true
		// Adjust sampling for categorical data
		if config.Sampling.Strategy == "independent" {
			config.Sampling.Strategy = "stratified"
		}
	}
}

// ValidateConfig ensures the generated configuration is reasonable
func (pg *PolicyGenerator) ValidateConfig(config RefereeConfig) error {
	// Validate significance levels
	if config.Shredder.Alpha <= 0 || config.Shredder.Alpha >= 1 {
		return NewPolicyError("invalid shredder alpha", nil)
	}

	// Ensure hard floor is respected
	if config.Shredder.Alpha < config.General.HardFloor {
		return NewPolicyError("shredder alpha violates hard floor", nil)
	}

	// Validate iteration counts
	if config.Shredder.Iterations < 1000 {
		return NewPolicyError("insufficient shredder iterations", nil)
	}

	// Validate lag parameters
	if config.Directional.MaxLag < 1 || config.Directional.MaxLag > 50 {
		return NewPolicyError("invalid directional max lag", nil)
	}

	return nil
}

// PolicyError represents policy generation errors
type PolicyError struct {
	Message string
	Cause   error
}

func (e PolicyError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

func NewPolicyError(message string, cause error) PolicyError {
	return PolicyError{Message: message, Cause: cause}
}

// Utility functions
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
