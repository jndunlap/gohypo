package brief

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// StatisticalBrief provides a unified statistical summary that serves multiple use cases:
// - Data ingestion quality assessment
// - Statistical validation metadata
// - LLM hypothesis generation enrichment
type StatisticalBrief struct {
	// Core Identification
	FieldKey   string `json:"field_key"`
	Source     string `json:"source,omitempty"` // For ingestion context
	SampleSize int    `json:"sample_size"`

	// Core Statistics (always computed)
	Summary      SummaryStats      `json:"summary"`
	Distribution DistributionStats `json:"distribution"`
	Quality      QualityStats      `json:"quality"`

	// Conditional Extensions (computed based on use case)
	Categorical *CategoricalStats `json:"categorical,omitempty"` // For categorical fields
	Temporal    *TemporalStats    `json:"temporal,omitempty"`    // For time series analysis
	Validation  *ValidationStats  `json:"validation,omitempty"`  // For referee rule generation

	// Computation Context
	Computation ComputationContext `json:"computation"`
	ComputedAt  time.Time          `json:"computed_at"`
}

// ComputationContext tracks what was computed for what purpose
type ComputationContext struct {
	ForIngestion  bool `json:"for_ingestion"`  // Basic quality check
	ForValidation bool `json:"for_validation"` // Adaptive referee rules
	ForHypothesis bool `json:"for_hypothesis"` // LLM metadata enrichment
}

// SummaryStats contains basic descriptive statistics
type SummaryStats struct {
	Mean   float64 `json:"mean"`
	StdDev float64 `json:"std_dev"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Median float64 `json:"median"`
	Q25    float64 `json:"q25"`
	Q75    float64 `json:"q75"`
}

// DistributionStats contains distribution shape analysis
type DistributionStats struct {
	Skewness float64 `json:"skewness"`
	Kurtosis float64 `json:"kurtosis"`
	IsNormal bool    `json:"is_normal"`
	ShapiroP float64 `json:"shapiro_p_value,omitempty"`
}

// QualityStats contains data quality metrics
type QualityStats struct {
	MissingRatio     float64 `json:"missing_ratio"`
	SparsityRatio    float64 `json:"sparsity_ratio"`
	NoiseCoefficient float64 `json:"noise_coefficient"`
	OutlierCount     int     `json:"outlier_count"`
}

// CategoricalStats contains categorical field analysis
type CategoricalStats struct {
	IsCategorical bool    `json:"is_categorical"`
	Cardinality   int     `json:"cardinality"`
	Entropy       float64 `json:"entropy"`
	Mode          string  `json:"mode,omitempty"`
	ModeFrequency int     `json:"mode_frequency,omitempty"`
	GiniIndex     float64 `json:"gini_index,omitempty"`
}

// TemporalStats contains temporal pattern analysis
type TemporalStats struct {
	IsStationary    bool    `json:"is_stationary"`
	VariancePValue  float64 `json:"variance_p_value,omitempty"`
	AdfStatistic    float64 `json:"adf_statistic,omitempty"`
	AdfPValue       float64 `json:"adf_p_value,omitempty"`
	SuggestedLags   []int   `json:"suggested_causal_lags,omitempty"`
	AutocorrLag1    float64 `json:"autocorr_lag1,omitempty"`
	UpdateFrequency float64 `json:"update_frequency,omitempty"`
	StabilityScore  float64 `json:"stability_score"`
}

// ValidationStats contains referee rule generation data
type ValidationStats struct {
	RecommendedAlpha  float64 `json:"recommended_alpha"`
	OptimalIterations int     `json:"optimal_iterations"`
	UseNonParametric  bool    `json:"use_nonparametric"`
	RequiresBootstrap bool    `json:"requires_bootstrap"`
	BootstrapSamples  int     `json:"bootstrap_samples,omitempty"`
}

// ComputationRequest defines what should be computed
type ComputationRequest struct {
	ForIngestion  bool `json:"for_ingestion"`
	ForValidation bool `json:"for_validation"`
	ForHypothesis bool `json:"for_hypothesis"`

	// Configuration options
	SampleLimit       int     `json:"sample_limit,omitempty"`       // Max samples to analyze
	SignificanceLevel float64 `json:"significance_level,omitempty"` // For validation stats
}

// ToLLMFormat converts brief to LLM-optimized string format
func (b StatisticalBrief) ToLLMFormat() string {
	distLabel := b.categorizeDistribution()
	stationarityLabel := b.categorizeStationarity()
	qualityLabel := b.categorizeQuality()

	return fmt.Sprintf("Distribution: %s | Stationarity: %s | Quality: %s",
		distLabel, stationarityLabel, qualityLabel)
}

// Helper methods for LLM-friendly categorization
func (b StatisticalBrief) categorizeDistribution() string {
	skewness := b.Distribution.Skewness
	kurtosis := b.Distribution.Kurtosis

	// Skewness interpretation
	var skewLabel string
	switch {
	case math.Abs(skewness) < 0.5:
		skewLabel = "Symmetric"
	case skewness > 0.5:
		skewLabel = "Right-skewed"
	case skewness < -0.5:
		skewLabel = "Left-skewed"
	}

	// Kurtosis interpretation (normal = 3.0)
	var kurtLabel string
	switch {
	case kurtosis < 2.5:
		kurtLabel = "Light tails"
	case kurtosis > 4.0:
		kurtLabel = "Heavy tails"
	default:
		kurtLabel = "Normal tails"
	}

	if skewLabel == "Symmetric" && kurtLabel == "Normal tails" {
		return "Normal-like"
	}
	return fmt.Sprintf("%s (%s)", skewLabel, kurtLabel)
}

func (b StatisticalBrief) categorizeStationarity() string {
	if b.Temporal == nil {
		return "Not analyzed"
	}

	if b.Temporal.IsStationary {
		return "Stationary"
	} else if b.Temporal.AdfPValue > 0.10 {
		return "Non-stationary"
	}
	return "Borderline stationary"
}

func (b StatisticalBrief) categorizeQuality() string {
	var labels []string

	if b.Quality.SparsityRatio > 0.5 {
		labels = append(labels, fmt.Sprintf("Sparse (%.0f%% zeros)", b.Quality.SparsityRatio*100))
	}

	switch {
	case b.Quality.NoiseCoefficient < 0.3:
		labels = append(labels, "Low noise")
	case b.Quality.NoiseCoefficient < 0.7:
		labels = append(labels, "Moderate noise")
	default:
		labels = append(labels, "High noise")
	}

	if b.Quality.MissingRatio > 0.1 {
		labels = append(labels, fmt.Sprintf("Missing (%.1f%%)", b.Quality.MissingRatio*100))
	}

	if len(labels) == 0 {
		return "High quality"
	}
	return strings.Join(labels, ", ")
}

// SenseResult represents the output of a single statistical sense
type SenseResult struct {
	SenseName   string                 `json:"sense_name"`
	EffectSize  float64                `json:"effect_size"`
	PValue      float64                `json:"p_value"`
	Confidence  float64                `json:"confidence"`  // 0-1 confidence score
	Signal      string                 `json:"signal"`      // "weak", "moderate", "strong", "very_strong"
	Description string                 `json:"description"` // Human-readable explanation
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// NewBrief creates a brief with appropriate computation context
func NewBrief(fieldKey string, source string, sampleSize int, request ComputationRequest) *StatisticalBrief {
	return &StatisticalBrief{
		FieldKey:   fieldKey,
		Source:     source,
		SampleSize: sampleSize,
		Computation: ComputationContext{
			ForIngestion:  request.ForIngestion,
			ForValidation: request.ForValidation,
			ForHypothesis: request.ForHypothesis,
		},
		ComputedAt: time.Now(),
	}
}
