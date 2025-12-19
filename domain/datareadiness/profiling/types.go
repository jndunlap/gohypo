package profiling

import (
	"time"

	"gohypo/domain/datareadiness/ingestion"
	"gohypo/domain/stats/brief"
)

// FieldProfile contains the complete statistical profile of a field_key
// This maintains backward compatibility while the system migrates to StatisticalBrief
type FieldProfile struct {
	FieldKey       string            `json:"field_key"`
	Source         string            `json:"source"`
	SampleSize     int               `json:"sample_size"`
	InferredType   InferredType      `json:"inferred_type"`
	TypeConfidence float64           `json:"type_confidence"`
	Cardinality    CardinalityStats  `json:"cardinality"`
	MissingStats   MissingStats      `json:"missing_stats"`
	TypeSpecific   TypeSpecificStats `json:"type_specific"`
	TemporalStats  TemporalStats     `json:"temporal_stats"`
	QualityScore   float64           `json:"quality_score"`
	ComputedAt     time.Time         `json:"computed_at"`
}

// NewFieldProfile creates a new field profile
func NewFieldProfile(fieldKey, source string, sampleSize int) *FieldProfile {
	return &FieldProfile{
		FieldKey:   fieldKey,
		Source:     source,
		SampleSize: sampleSize,
		ComputedAt: time.Now(),
	}
}

// FromStatisticalBrief converts a StatisticalBrief to FieldProfile for backward compatibility
func FromStatisticalBrief(sb *brief.StatisticalBrief) *FieldProfile {
	fp := &FieldProfile{
		FieldKey:   sb.FieldKey,
		Source:     sb.Source,
		SampleSize: sb.SampleSize,
		ComputedAt: sb.ComputedAt,
	}

	// Map basic stats
	fp.MissingStats = MissingStats{
		MissingCount: int(sb.Quality.MissingRatio * float64(sb.SampleSize)),
		MissingRate:  sb.Quality.MissingRatio,
	}

	fp.Cardinality = CardinalityStats{
		UniqueCount: int(float64(sb.SampleSize) * (1.0 - sb.Quality.SparsityRatio)),
		UniqueRatio: 1.0 - sb.Quality.SparsityRatio,
	}

	if sb.Categorical != nil {
		fp.Cardinality.Entropy = sb.Categorical.Entropy
		fp.InferredType = TypeCategorical
		fp.TypeConfidence = 0.9
	} else {
		fp.InferredType = TypeNumeric
		fp.TypeConfidence = 0.9
	}

	// Map temporal stats
	if sb.Temporal != nil {
		fp.TemporalStats = TemporalStats{
			HasTemporalUpdates: true,
			UpdateFrequency:    1.0, // Placeholder
			StabilityScore:     sb.Temporal.StabilityScore,
		}
	}

	// Map type-specific stats
	if sb.Categorical != nil && sb.Categorical.IsCategorical {
		fp.TypeSpecific.CategoricalStats = &CategoricalStats{
			Mode:          sb.Categorical.Mode,
			ModeFrequency: sb.Categorical.ModeFrequency,
			GiniIndex:     sb.Categorical.GiniIndex,
		}
	} else {
		fp.TypeSpecific.NumericStats = &NumericStats{
			Min:       sb.Summary.Min,
			Max:       sb.Summary.Max,
			Mean:      sb.Summary.Mean,
			Median:    sb.Summary.Median,
			StdDev:    sb.Summary.StdDev,
			Skewness:  sb.Distribution.Skewness,
			Kurtosis:  sb.Distribution.Kurtosis,
			ZeroCount: int(sb.Quality.SparsityRatio * float64(sb.SampleSize)),
		}
	}

	// Compute quality score
	fp.QualityScore = fp.ComputeQualityScore()

	return fp
}

// ToStatisticalBrief converts FieldProfile to StatisticalBrief
func (fp *FieldProfile) ToStatisticalBrief() *brief.StatisticalBrief {
	request := brief.ComputationRequest{ForIngestion: true}

	sb := brief.NewBrief(fp.FieldKey, fp.Source, fp.SampleSize, request)

	// Map quality stats
	sb.Quality.MissingRatio = fp.MissingStats.MissingRate
	sb.Quality.SparsityRatio = fp.Cardinality.UniqueRatio
	sb.Quality.NoiseCoefficient = 0.1 // Placeholder

	// Map summary stats
	if fp.TypeSpecific.NumericStats != nil {
		sb.Summary = brief.SummaryStats{
			Mean:   fp.TypeSpecific.NumericStats.Mean,
			StdDev: fp.TypeSpecific.NumericStats.StdDev,
			Min:    fp.TypeSpecific.NumericStats.Min,
			Max:    fp.TypeSpecific.NumericStats.Max,
			Median: fp.TypeSpecific.NumericStats.Median,
			Q25:    0, // Placeholder
			Q75:    0, // Placeholder
		}

		sb.Distribution = brief.DistributionStats{
			Skewness: fp.TypeSpecific.NumericStats.Skewness,
			Kurtosis: fp.TypeSpecific.NumericStats.Kurtosis,
			IsNormal: true, // Placeholder
		}
	}

	// Map categorical stats
	if fp.TypeSpecific.CategoricalStats != nil {
		sb.Categorical = &brief.CategoricalStats{
			IsCategorical: fp.InferredType == TypeCategorical,
			Cardinality:   fp.Cardinality.UniqueCount,
			Entropy:       fp.Cardinality.Entropy,
			Mode:          fp.TypeSpecific.CategoricalStats.Mode,
			ModeFrequency: fp.TypeSpecific.CategoricalStats.ModeFrequency,
			GiniIndex:     fp.TypeSpecific.CategoricalStats.GiniIndex,
		}
	}

	return sb
}

// InferredType represents the automatically detected data type
type InferredType string

const (
	TypeNumeric     InferredType = "numeric"
	TypeCategorical InferredType = "categorical"
	TypeBoolean     InferredType = "boolean"
	TypeTimestamp   InferredType = "timestamp"
	TypeText        InferredType = "text"
	TypeUnknown     InferredType = "unknown"
)

// CardinalityStats describes the uniqueness and distribution
type CardinalityStats struct {
	UniqueCount int          `json:"unique_count"`
	UniqueRatio float64      `json:"unique_ratio"`
	TopValues   []ValueCount `json:"top_values"`
	Entropy     float64      `json:"entropy"` // Shannon entropy
}

// ValueCount represents a value and its frequency
type ValueCount struct {
	Value string  `json:"value"`
	Count int     `json:"count"`
	Ratio float64 `json:"ratio"`
}

// MissingStats tracks missing value patterns
type MissingStats struct {
	MissingCount       int     `json:"missing_count"`
	MissingRate        float64 `json:"missing_rate"`
	ConsecutiveMissing int     `json:"consecutive_missing"` // Max consecutive missing
}

// TypeSpecificStats contains type-dependent statistics
type TypeSpecificStats struct {
	// For numeric types
	NumericStats *NumericStats `json:"numeric_stats,omitempty"`

	// For categorical types
	CategoricalStats *CategoricalStats `json:"categorical_stats,omitempty"`

	// For text types
	TextStats *TextStats `json:"text_stats,omitempty"`
}

// NumericStats contains statistics for numeric fields
type NumericStats struct {
	Min           float64 `json:"min"`
	Max           float64 `json:"max"`
	Mean          float64 `json:"mean"`
	Median        float64 `json:"median"`
	StdDev        float64 `json:"std_dev"`
	Skewness      float64 `json:"skewness"`
	Kurtosis      float64 `json:"kurtosis"`
	ZeroCount     int     `json:"zero_count"`
	NegativeCount int     `json:"negative_count"`
	OutlierCount  int     `json:"outlier_count"` // IQR method
}

// CategoricalStats contains statistics for categorical fields
type CategoricalStats struct {
	Mode           string  `json:"mode"` // Most frequent value
	ModeFrequency  int     `json:"mode_frequency"`
	GiniIndex      float64 `json:"gini_index"`      // Measure of inequality
	RareCategories int     `json:"rare_categories"` // Categories appearing < 1%
}

// TextStats contains statistics for text fields
type TextStats struct {
	AvgLength       float64  `json:"avg_length"`
	MaxLength       int      `json:"max_length"`
	HasNumbers      bool     `json:"has_numbers"`
	HasSpecialChars bool     `json:"has_special_chars"`
	CommonPatterns  []string `json:"common_patterns"` // Regex patterns detected
}

// TemporalStats describes how the field behaves over time
type TemporalStats struct {
	HasTemporalUpdates bool    `json:"has_temporal_updates"`
	UpdateFrequency    float64 `json:"update_frequency"` // Updates per day
	LastUpdateGap      int64   `json:"last_update_gap"`  // Days since last update
	StabilityScore     float64 `json:"stability_score"`  // 0-1, how stable over time
}

// ProfilingConfig defines the profiling parameters
type ProfilingConfig struct {
	SampleSize                int     `json:"sample_size"`                 // How many events to sample
	TypeThreshold             float64 `json:"type_threshold"`              // % success for type inference
	CategoricalMaxCard        int     `json:"categorical_max_card"`        // Max categories before truncation
	MinQualityScore           float64 `json:"min_quality_score"`           // Minimum acceptable quality
	NumericThreshold          float64 `json:"numeric_threshold"`           // % of values that must parse as numbers (default 0.95)
	BooleanThreshold          float64 `json:"boolean_threshold"`           // % of values that must parse as booleans (default 0.98)
	TimestampThreshold        float64 `json:"timestamp_threshold"`         // % of values that must parse as timestamps (default 0.90)
	AmbiguousNumericThreshold float64 `json:"ambiguous_numeric_threshold"` // % for ambiguous numeric detection (default 0.8)
	CategoricalUniqueRatio    float64 `json:"categorical_unique_ratio"`    // Max unique ratio for categorical codes (default 0.3)
	CategoricalIntegerRatio   float64 `json:"categorical_integer_ratio"`   // Min integer ratio for categorical codes (default 0.8)
}

// DefaultProfilingConfig returns sensible defaults
func DefaultProfilingConfig() ProfilingConfig {
	return ProfilingConfig{
		SampleSize:                10000,
		TypeThreshold:             0.8,  // 80% of values must parse successfully
		CategoricalMaxCard:        100,  // Cap at 100 categories
		MinQualityScore:           0.3,  // Require at least 30% quality
		NumericThreshold:          0.95, // 95% must parse as numbers for high confidence
		BooleanThreshold:          0.98, // 98% must parse as booleans for high confidence
		TimestampThreshold:        0.90, // 90% must parse as timestamps
		AmbiguousNumericThreshold: 0.8,  // 80% for ambiguous numeric detection
		CategoricalUniqueRatio:    0.3,  // Max 30% unique ratio for categorical codes
		CategoricalIntegerRatio:   0.8,  // Min 80% integer ratio for categorical codes
	}
}

// ProfilingResult contains the outcome of profiling a source
type ProfilingResult struct {
	SourceName  string         `json:"source_name"`
	Profiles    []FieldProfile `json:"profiles"`
	TotalFields int            `json:"total_fields"`
	DurationMs  int64          `json:"duration_ms"`
	Errors      []string       `json:"errors"`
}

// FieldProfiler defines the interface for profiling field_keys
type FieldProfiler interface {
	// ProfileField analyzes a field_key across all its events
	ProfileField(fieldKey string, source string, events []ingestion.CanonicalEvent, config ProfilingConfig) FieldProfile

	// ProfileSource analyzes all fields from a source
	ProfileSource(sourceName string, events []ingestion.CanonicalEvent, config ProfilingConfig) ProfilingResult
}

// ComputeQualityScore calculates an overall quality score for a field
func (fp *FieldProfile) ComputeQualityScore() float64 {
	score := 1.0

	// Penalize high missing rates
	score *= (1.0 - fp.MissingStats.MissingRate)

	// Penalize low information content (high uniqueness for categoricals)
	if fp.InferredType == TypeCategorical {
		if fp.Cardinality.UniqueRatio > 0.9 {
			score *= 0.5 // Too many unique values
		}
	}

	// Penalize temporal instability
	score *= fp.TemporalStats.StabilityScore

	// Ensure score is between 0 and 1
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	fp.QualityScore = score
	return score
}
