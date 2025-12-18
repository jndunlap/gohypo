package profiling

import (
	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
)

// FieldProfile contains the complete statistical profile of a field_key
type FieldProfile struct {
	FieldKey      string            `json:"field_key"`
	Source        string            `json:"source"`
	SampleSize    int               `json:"sample_size"`
	InferredType  InferredType      `json:"inferred_type"`
	Cardinality   CardinalityStats  `json:"cardinality"`
	MissingStats  MissingStats      `json:"missing_stats"`
	TypeSpecific  TypeSpecificStats `json:"type_specific"`
	TemporalStats TemporalStats     `json:"temporal_stats"`
	QualityScore  float64           `json:"quality_score"` // 0-1, higher is better
	ComputedAt    core.Timestamp    `json:"computed_at"`
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
	SampleSize         int     `json:"sample_size"`          // How many events to sample
	TypeThreshold      float64 `json:"type_threshold"`       // % success for type inference
	CategoricalMaxCard int     `json:"categorical_max_card"` // Max categories before truncation
	MinQualityScore    float64 `json:"min_quality_score"`    // Minimum acceptable quality
}

// DefaultProfilingConfig returns sensible defaults
func DefaultProfilingConfig() ProfilingConfig {
	return ProfilingConfig{
		SampleSize:         10000,
		TypeThreshold:      0.8, // 80% of values must parse successfully
		CategoricalMaxCard: 100, // Cap at 100 categories
		MinQualityScore:    0.3, // Require at least 30% quality
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

	// Penalize poor type inference confidence
	if fp.SampleSize > 100 {
		// Assume some type inference confidence metric
		score *= 0.9 // Placeholder
	}

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
