package synthesizer

import (
	"fmt"
	"strings"

	"gohypo/domain/core"
	"gohypo/domain/datareadiness/profiling"
	"gohypo/domain/dataset"
)

// ContractSynthesizer generates contract drafts from field profiles
type ContractSynthesizer struct {
	config SynthesisConfig
}

// SynthesisConfig defines the synthesis rules
type SynthesisConfig struct {
	DefaultLagDays    int     `json:"default_lag_days"`
	CountThreshold    float64 `json:"count_threshold"`    // Min ratio for count mode
	SumThreshold      float64 `json:"sum_threshold"`      // Min ratio for sum mode
	ExistsThreshold   float64 `json:"exists_threshold"`   // Max ratio for exists mode
	VarianceThreshold float64 `json:"variance_threshold"` // Min variance for latest mode
}

// DefaultSynthesisConfig returns sensible defaults
func DefaultSynthesisConfig() SynthesisConfig {
	return SynthesisConfig{
		DefaultLagDays:    1,
		CountThreshold:    0.1,  // 10% zeros suggests counting
		SumThreshold:      0.1,  // 10% zeros suggests summing
		ExistsThreshold:   0.8,  // 80% missing suggests existence check
		VarianceThreshold: 0.01, // Minimum variance for temporal changes
	}
}

// NewContractSynthesizer creates a synthesizer with config
func NewContractSynthesizer(config SynthesisConfig) *ContractSynthesizer {
	return &ContractSynthesizer{config: config}
}

// SynthesizeContracts generates contract drafts from field profiles
func (s *ContractSynthesizer) SynthesizeContracts(profiles []profiling.FieldProfile) ([]ContractDraft, error) {
	drafts := make([]ContractDraft, 0, len(profiles))

	for _, profile := range profiles {
		if s.shouldSkipProfile(profile) {
			continue
		}

		draft := s.synthesizeContract(profile)
		drafts = append(drafts, draft)
	}

	return drafts, nil
}

// shouldSkipProfile determines if a profile should be skipped
func (s *ContractSynthesizer) shouldSkipProfile(profile profiling.FieldProfile) bool {
	// Skip profiles with too low quality
	if profile.QualityScore < 0.3 {
		return true
	}

	// Skip profiles with extreme missingness
	if profile.MissingStats.MissingRate > 0.95 {
		return true
	}

	// Skip profiles with unknown types
	if profile.InferredType == profiling.TypeUnknown {
		return true
	}

	return false
}

// synthesizeContract creates a contract draft from a profile
func (s *ContractSynthesizer) synthesizeContract(profile profiling.FieldProfile) ContractDraft {
	draft := ContractDraft{
		VariableKey: profile.FieldKey,
		Source:      profile.Source,
		Profile:     profile,
	}

	// Synthesize as-of mode
	draft.AsOfMode = s.synthesizeAsOfMode(profile)
	draft.Reasoning.AsOfMode = s.explainAsOfMode(profile, draft.AsOfMode)

	// Synthesize imputation policy
	draft.ImputationPolicy = s.synthesizeImputation(profile)
	draft.Reasoning.Imputation = s.explainImputation(profile, draft.ImputationPolicy)

	// Synthesize statistical type
	draft.StatisticalType = s.synthesizeStatisticalType(profile)
	draft.Reasoning.StatisticalType = s.explainStatisticalType(profile, draft.StatisticalType)

	// Synthesize categorical encoding for categorical variables
	if draft.StatisticalType == "categorical" {
		draft.CategoricalEncoding = s.synthesizeCategoricalEncoding(profile)
	}

	// Set window days if applicable
	draft.WindowDays = s.synthesizeWindowDays(profile, draft.AsOfMode)

	// Set lag (conservative default)
	draft.LagDays = s.config.DefaultLagDays

	// Determine scalar guarantee
	draft.ScalarGuarantee = s.determineScalarGuarantee(profile, draft.AsOfMode)
	draft.Reasoning.ScalarGuarantee = s.explainScalarGuarantee(profile, draft.AsOfMode, draft.ScalarGuarantee)

	// Calculate overall confidence
	draft.Confidence = s.calculateConfidence(profile, draft)

	return draft
}

// synthesizeAsOfMode determines the appropriate as-of mode
func (s *ContractSynthesizer) synthesizeAsOfMode(profile profiling.FieldProfile) string {
	switch profile.InferredType {
	case profiling.TypeNumeric:
		// Check if it looks like a counter (many zeros, increasing trend)
		if s.looksLikeCounter(profile) {
			return "count_over_window"
		}

		// Check if it looks like amounts that should be summed
		if s.looksLikeSummable(profile) {
			return "sum_over_window"
		}

		// Default to latest value for numeric
		return "latest_value_as_of"

	case profiling.TypeBoolean:
		// Booleans are often existence indicators
		if profile.MissingStats.MissingRate > s.config.ExistsThreshold {
			return "exists_as_of"
		}
		return "latest_value_as_of"

	case profiling.TypeCategorical:
		// Categorical variables usually want latest value
		return "latest_value_as_of"

	case profiling.TypeTimestamp:
		// Timestamps want latest
		return "latest_value_as_of"

	default:
		return "latest_value_as_of"
	}
}

// looksLikeCounter analyzes if a numeric field behaves like a counter
func (s *ContractSynthesizer) looksLikeCounter(profile profiling.FieldProfile) bool {
	if profile.TypeSpecific.NumericStats == nil {
		return false
	}

	stats := profile.TypeSpecific.NumericStats

	// Counters typically have:
	// - Many zeros (starting state)
	// - Only non-negative values
	// - Relatively low variance compared to mean

	zeroRatio := float64(stats.ZeroCount) / float64(profile.SampleSize)
	hasNegatives := stats.NegativeCount > 0
	varianceRatio := stats.StdDev / (stats.Mean + 1e-10) // Avoid division by zero

	return zeroRatio >= s.config.CountThreshold &&
		!hasNegatives &&
		varianceRatio < 2.0 // Not too variable
}

// looksLikeSummable analyzes if a numeric field should be summed
func (s *ContractSynthesizer) looksLikeSummable(profile profiling.FieldProfile) bool {
	if profile.TypeSpecific.NumericStats == nil {
		return false
	}

	stats := profile.TypeSpecific.NumericStats

	// Summable amounts typically have:
	// - Some zeros (no activity periods)
	// - Can be negative (debits/credits)
	// - Moderate variance

	zeroRatio := float64(stats.ZeroCount) / float64(profile.SampleSize)

	return zeroRatio >= s.config.SumThreshold &&
		stats.StdDev > s.config.VarianceThreshold
}

// synthesizeImputation determines the imputation policy
func (s *ContractSynthesizer) synthesizeImputation(profile profiling.FieldProfile) string {
	missingRate := profile.MissingStats.MissingRate

	// High missing rate suggests existence semantics
	if missingRate > 0.5 {
		return "missing_indicator"
	}

	// For counters and sums, zero fill makes sense
	asOfMode := s.synthesizeAsOfMode(profile)
	if asOfMode == "count_over_window" || asOfMode == "sum_over_window" {
		return "zero_fill"
	}

	// For state-like variables, forward fill if we have temporal updates
	if profile.TemporalStats.HasTemporalUpdates {
		return "forward_fill"
	}

	// Default to missing indicator
	return "missing_indicator"
}

// synthesizeStatisticalType maps profile type to contract type
func (s *ContractSynthesizer) synthesizeStatisticalType(profile profiling.FieldProfile) string {
	switch profile.InferredType {
	case profiling.TypeNumeric:
		return "numeric"
	case profiling.TypeCategorical:
		return "categorical"
	case profiling.TypeBoolean:
		return "binary"
	case profiling.TypeTimestamp:
		return "timestamp"
	default:
		return "categorical" // Safe default
	}
}

// synthesizeWindowDays sets window for windowed modes
func (s *ContractSynthesizer) synthesizeWindowDays(profile profiling.FieldProfile, asOfMode string) *int {
	if asOfMode == "count_over_window" || asOfMode == "sum_over_window" {
		// Default 30 days for windowed operations
		days := 30
		return &days
	}
	return nil
}

// determineScalarGuarantee checks if the mode guarantees scalar results
func (s *ContractSynthesizer) determineScalarGuarantee(profile profiling.FieldProfile, asOfMode string) bool {
	// All our supported modes guarantee scalar results by construction
	switch asOfMode {
	case "latest_value_as_of", "count_over_window", "sum_over_window", "exists_as_of":
		return true
	default:
		return false
	}
}

// calculateConfidence computes overall confidence in the synthesized contract
func (s *ContractSynthesizer) calculateConfidence(profile profiling.FieldProfile, draft ContractDraft) float64 {
	confidence := profile.QualityScore

	// Boost confidence for clear patterns
	if draft.AsOfMode == "count_over_window" && s.looksLikeCounter(profile) {
		confidence *= 1.2
	}

	if draft.AsOfMode == "sum_over_window" && s.looksLikeSummable(profile) {
		confidence *= 1.2
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// Explanation methods provide reasoning for synthesis decisions
func (s *ContractSynthesizer) explainAsOfMode(profile profiling.FieldProfile, mode string) string {
	switch mode {
	case "count_over_window":
		return "Numeric field with many zeros and non-negative values suggests counting semantics"
	case "sum_over_window":
		return "Numeric field with some zeros suggests accumulation/summing semantics"
	case "exists_as_of":
		return fmt.Sprintf("High missing rate (%.1f%%) suggests existence indicator",
			profile.MissingStats.MissingRate*100)
	case "latest_value_as_of":
		return "Default mode for state-like variables that change over time"
	default:
		return "Default mode selection"
	}
}

func (s *ContractSynthesizer) explainImputation(profile profiling.FieldProfile, policy string) string {
	switch policy {
	case "zero_fill":
		return "Appropriate for counters and sums where zero represents no activity"
	case "forward_fill":
		return "Temporal updates detected, forward fill preserves state continuity"
	case "missing_indicator":
		return "Creates explicit missing indicators for statistical modeling"
	default:
		return "Default imputation policy"
	}
}

func (s *ContractSynthesizer) explainStatisticalType(profile profiling.FieldProfile, statType string) string {
	return fmt.Sprintf("Automatically inferred as %s based on %d sample values with %.1f%% success rate",
		statType, profile.SampleSize, profile.QualityScore*100)
}

func (s *ContractSynthesizer) explainScalarGuarantee(profile profiling.FieldProfile, mode string, guaranteed bool) string {
	if guaranteed {
		return fmt.Sprintf("%s mode guarantees exactly one scalar value per entity per snapshot", mode)
	}
	return "Mode does not guarantee scalar results - manual review required"
}

// ContractDraft represents a synthesized contract with reasoning
type ContractDraft struct {
	VariableKey         string                 `json:"variable_key"`
	Source              string                 `json:"source"`
	AsOfMode            string                 `json:"as_of_mode"`
	StatisticalType     string                 `json:"statistical_type"`
	ImputationPolicy    string                 `json:"imputation_policy"`
	WindowDays          *int                   `json:"window_days,omitempty"`
	LagDays             int                    `json:"lag_days"`
	ScalarGuarantee     bool                   `json:"scalar_guarantee"`
	Confidence          float64                `json:"confidence"`
	CategoricalEncoding map[string]float64     `json:"categorical_encoding,omitempty"` // For categorical variables: value -> numeric encoding
	Profile             profiling.FieldProfile `json:"profile"`
	Reasoning           ContractReasoning      `json:"reasoning"`
}

// synthesizeCategoricalEncoding creates an ordinal encoding for categorical variables
func (s *ContractSynthesizer) synthesizeCategoricalEncoding(profile profiling.FieldProfile) map[string]float64 {
	encoding := make(map[string]float64)

	// Use the top values from cardinality analysis to determine encoding
	topValues := profile.Cardinality.TopValues

	// Check for special patterns and use semantic encodings
	if semanticEncoding := s.detectSemanticEncoding(profile.FieldKey, topValues); semanticEncoding != nil {
		encoding = semanticEncoding
	} else if len(topValues) <= 10 {
		// Small cardinality - use frequency-based ordering (most common = 0, then 1, 2, etc.)
		for i, valueCount := range topValues {
			encoding[valueCount.Value] = float64(i)
		}
	} else {
		// High cardinality - use hash-based encoding with smaller range
		for _, valueCount := range topValues {
			// Simple hash with smaller range for high cardinality
			hash := 0
			for _, r := range valueCount.Value {
				hash = hash*31 + int(r)
			}
			encoding[valueCount.Value] = float64(hash % 50) // Smaller range than before
		}
	}

	// Add any remaining values not in top values as unknowns
	if len(topValues) < profile.Cardinality.UniqueCount {
		maxKnown := 0.0
		for _, v := range encoding {
			if v > maxKnown {
				maxKnown = v
			}
		}
		encoding["__unknown__"] = maxKnown + 1.0
	}

	return encoding
}

// detectSemanticEncoding detects special categorical patterns and returns semantic encodings
func (s *ContractSynthesizer) detectSemanticEncoding(fieldKey string, topValues []profiling.ValueCount) map[string]float64 {
	// Extract values
	values := make([]string, len(topValues))
	for i, vc := range topValues {
		values[i] = vc.Value
	}

	// Check for football match result pattern (H, D, A)
	if s.isFootballResultPattern(fieldKey, values) {
		return map[string]float64{
			"H": 1.0,  // Home win
			"D": 0.0,  // Draw
			"A": -1.0, // Away win
		}
	}

	// Check for binary yes/no patterns
	if s.isBinaryPattern(values) {
		return s.createBinaryEncoding(values)
	}

	// Check for ordered categorical patterns (Low, Medium, High, etc.)
	if orderedEncoding := s.detectOrderedPattern(values); orderedEncoding != nil {
		return orderedEncoding
	}

	return nil // No semantic pattern detected
}

// isFootballResultPattern detects football match result variables
func (s *ContractSynthesizer) isFootballResultPattern(fieldKey string, values []string) bool {
	// Check field name patterns
	fieldLower := strings.ToLower(fieldKey)
	if strings.Contains(fieldLower, "result") ||
		strings.Contains(fieldLower, "ftr") ||
		strings.Contains(fieldLower, "outcome") {
		// Check if values match football pattern
		hasH, hasD, hasA := false, false, false
		for _, v := range values {
			switch v {
			case "H":
				hasH = true
			case "D":
				hasD = true
			case "A":
				hasA = true
			}
		}
		return hasH && hasD && hasA
	}
	return false
}

// isBinaryPattern detects binary yes/no patterns
func (s *ContractSynthesizer) isBinaryPattern(values []string) bool {
	if len(values) != 2 {
		return false
	}

	// Check for common binary pairs
	binaryPairs := [][]string{
		{"Y", "N"}, {"YES", "NO"}, {"TRUE", "FALSE"}, {"T", "F"},
		{"1", "0"}, {"ON", "OFF"}, {"ENABLED", "DISABLED"},
	}

	for _, pair := range binaryPairs {
		if (values[0] == pair[0] && values[1] == pair[1]) ||
			(values[0] == pair[1] && values[1] == pair[0]) {
			return true
		}
	}

	return false
}

// createBinaryEncoding creates encoding for binary variables
func (s *ContractSynthesizer) createBinaryEncoding(values []string) map[string]float64 {
	encoding := make(map[string]float64)

	// Assign 1.0 to "positive" values, 0.0 to "negative" values
	for _, value := range values {
		valUpper := strings.ToUpper(value)
		switch valUpper {
		case "Y", "YES", "TRUE", "T", "1", "ON", "ENABLED":
			encoding[value] = 1.0
		default:
			encoding[value] = 0.0
		}
	}

	return encoding
}

// detectOrderedPattern detects ordinal patterns like Low/Medium/High
func (s *ContractSynthesizer) detectOrderedPattern(values []string) map[string]float64 {
	orderedSets := [][]string{
		{"LOW", "MEDIUM", "HIGH"},
		{"SMALL", "MEDIUM", "LARGE"},
		{"BAD", "GOOD", "EXCELLENT"},
		{"POOR", "FAIR", "GOOD"},
	}

	// Convert to uppercase for comparison
	upperValues := make([]string, len(values))
	valueMap := make(map[string]string)
	for i, v := range values {
		upperValues[i] = strings.ToUpper(v)
		valueMap[upperValues[i]] = v
	}

	for _, orderedSet := range orderedSets {
		if s.containsAll(upperValues, orderedSet) {
			encoding := make(map[string]float64)
			for i, upperVal := range orderedSet {
				if originalVal, exists := valueMap[upperVal]; exists {
					encoding[originalVal] = float64(i)
				}
			}
			return encoding
		}
	}

	return nil
}

// containsAll checks if slice contains all elements of subset
func (s *ContractSynthesizer) containsAll(slice, subset []string) bool {
	set := make(map[string]bool)
	for _, item := range slice {
		set[item] = true
	}

	for _, item := range subset {
		if !set[item] {
			return false
		}
	}

	return true
}

// ContractReasoning explains the synthesis decisions
type ContractReasoning struct {
	AsOfMode        string `json:"as_of_mode"`
	Imputation      string `json:"imputation"`
	StatisticalType string `json:"statistical_type"`
	ScalarGuarantee string `json:"scalar_guarantee"`
}

// ToVariableContract converts the draft to a domain contract
func (d *ContractDraft) ToVariableContract() *dataset.VariableContract {
	return &dataset.VariableContract{
		VarKey:              core.VariableKey(d.VariableKey),
		AsOfMode:            dataset.AsOfMode(d.AsOfMode),
		StatisticalType:     dataset.StatisticalType(d.StatisticalType),
		WindowDays:          d.WindowDays,
		ImputationPolicy:    dataset.ImputationPolicy(d.ImputationPolicy),
		ScalarGuarantee:     d.ScalarGuarantee,
		CategoricalEncoding: d.CategoricalEncoding,
	}
}
