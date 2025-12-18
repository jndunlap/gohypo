package datareadiness

import (
	"context"
	"fmt"
	"math"
	"strings"

	"gohypo/adapters/datareadiness/coercer"
	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
	"gohypo/domain/datareadiness/profiling"
)

// ProfilerAdapter implements ProfilerPort for data profiling
type ProfilerAdapter struct {
	coercer *coercer.TypeCoercer
}

// NewProfilerAdapter creates a new profiler adapter with coercer integration
func NewProfilerAdapter(coercer *coercer.TypeCoercer) *ProfilerAdapter {
	return &ProfilerAdapter{
		coercer: coercer,
	}
}

// ProfileSource analyzes all fields from a source
func (p *ProfilerAdapter) ProfileSource(ctx context.Context, sourceName string, events []ingestion.CanonicalEvent, config profiling.ProfilingConfig) (*profiling.ProfilingResult, error) {
	if len(events) == 0 {
		return &profiling.ProfilingResult{
			SourceName:  sourceName,
			Profiles:    []profiling.FieldProfile{},
			TotalFields: 0,
			DurationMs:  0,
			Errors:      []string{"no events to profile"},
		}, nil
	}

	// Extract field names from first event's raw payload
	firstEvent := events[0]
	if firstEvent.RawPayload == nil {
		return nil, fmt.Errorf("events must have raw payload data")
	}

	fieldNames := make([]string, 0)
	for k := range firstEvent.RawPayload {
		fieldNames = append(fieldNames, k)
	}

	// Sample events for profiling
	sampleSize := config.SampleSize
	if sampleSize > len(events) {
		sampleSize = len(events)
	}

	sampleEvents := events[:sampleSize]

	// Convert to interface{} for profiling
	sampleData := make([]interface{}, len(sampleEvents))
	for i, event := range sampleEvents {
		sampleData[i] = event.RawPayload
	}

	// Profile each field
	profiles := make([]profiling.FieldProfile, len(fieldNames))
	for i, fieldName := range fieldNames {
		profile := p.profileField(fieldName, sourceName, sampleData, config)
		profiles[i] = profile
	}

	return &profiling.ProfilingResult{
		SourceName:  sourceName,
		Profiles:    profiles,
		TotalFields: len(fieldNames),
		DurationMs:  0, // TODO: measure actual duration
		Errors:      []string{},
	}, nil
}

// profileField analyzes a single field across all events
func (p *ProfilerAdapter) profileField(fieldName, sourceName string, events []interface{}, config profiling.ProfilingConfig) profiling.FieldProfile {
	totalCount := len(events)
	missingCount := 0
	var values []interface{}

	// Collect non-nil values
	for _, event := range events {
		if eventMap, ok := event.(map[string]interface{}); ok {
			if value, exists := eventMap[fieldName]; exists && value != nil {
				values = append(values, value)
			} else {
				missingCount++
			}
		}
	}

	// Infer type from values with confidence scoring
	inferredType, typeConfidence := p.inferTypeWithConfidence(values, config)

	// Compute cardinality stats
	cardinalityStats := p.computeCardinalityStats(values)

	// Compute missing stats
	missingStats := p.computeMissingStats(missingCount, totalCount, events, fieldName)

	// Create profile
	profile := profiling.FieldProfile{
		FieldKey:       fieldName,
		Source:         sourceName,
		SampleSize:     totalCount,
		InferredType:   inferredType,
		TypeConfidence: typeConfidence,
		Cardinality:    cardinalityStats,
		MissingStats:   missingStats,
		QualityScore:   p.computeQualityScore(missingCount, totalCount, len(values)),
		ComputedAt:     core.Now(),
	}

	// Add type-specific stats
	switch inferredType {
	case profiling.TypeNumeric:
		profile.TypeSpecific.NumericStats = p.computeNumericStats(values)
	case profiling.TypeCategorical:
		profile.TypeSpecific.CategoricalStats = p.computeCategoricalStats(values)
	case profiling.TypeText:
		profile.TypeSpecific.TextStats = p.computeTextStats(values)
	}

	return profile
}

// inferType determines the most likely data type using coercer-based parsing
func (p *ProfilerAdapter) inferType(values []interface{}) profiling.InferredType {
	config := profiling.DefaultProfilingConfig()
	inferredType, _ := p.inferTypeWithConfidence(values, config)
	return inferredType
}

// inferTypeWithConfidence determines type with statistical confidence scoring
func (p *ProfilerAdapter) inferTypeWithConfidence(values []interface{}, config profiling.ProfilingConfig) (profiling.InferredType, float64) {
	if len(values) == 0 {
		return profiling.TypeUnknown, 0.0
	}

	// Use coercer to analyze type distribution
	analysis := p.coercer.AnalyzeTypeDistribution(values)

	// Calculate confidence based on the strength of the signal
	var confidence float64

	// High confidence numeric detection - but check for categorical codes first
	if analysis.NumericRatio >= config.NumericThreshold {
		// Even high-confidence numeric can be categorical if it looks like codes
		if p.looksLikeCategoricalCodes(values, config) {
			confidence = 0.8 // High confidence categorical override
			return profiling.TypeCategorical, confidence
		}
		confidence = math.Min(analysis.NumericRatio, 0.99) // Cap at 0.99 to leave room for uncertainty
		return profiling.TypeNumeric, confidence
	}

	// High confidence boolean detection
	if analysis.BooleanRatio >= config.BooleanThreshold {
		confidence = math.Min(analysis.BooleanRatio, 0.99)
		return profiling.TypeBoolean, confidence
	}

	// Timestamp detection
	if analysis.TimestampRatio >= config.TimestampThreshold {
		confidence = math.Min(analysis.TimestampRatio, 0.95)
		return profiling.TypeTimestamp, confidence
	}

	// Ambiguous numeric case - check for categorical codes
	if analysis.NumericRatio >= config.AmbiguousNumericThreshold {
		if p.looksLikeCategoricalCodes(values, config) {
			// Lower confidence for cardinality-based decisions
			confidence = 0.7
			return profiling.TypeCategorical, confidence
		} else {
			// Moderate confidence numeric
			confidence = analysis.NumericRatio * 0.8 // Scale down for ambiguity
			return profiling.TypeNumeric, confidence
		}
	}

	// Default to categorical with low confidence
	confidence = 0.5
	return profiling.TypeCategorical, confidence
}

// looksLikeCategoricalCodes checks if numeric values behave like categorical codes
func (p *ProfilerAdapter) looksLikeCategoricalCodes(values []interface{}, config profiling.ProfilingConfig) bool {
	var numbers []float64

	// Extract numeric values from the data - handle all numeric types
	for _, val := range values {
		var num float64
		var isNumeric bool

		switch v := val.(type) {
		case string:
			// Use coercer to parse string values
			coercedVal := p.coercer.CoerceValue(v)
			if coercedVal.Type == ingestion.ValueTypeNumeric && coercedVal.NumericVal != nil {
				num = *coercedVal.NumericVal
				isNumeric = true
			}
		case int:
			num = float64(v)
			isNumeric = true
		case int8:
			num = float64(v)
			isNumeric = true
		case int16:
			num = float64(v)
			isNumeric = true
		case int32:
			num = float64(v)
			isNumeric = true
		case int64:
			num = float64(v)
			isNumeric = true
		case uint:
			num = float64(v)
			isNumeric = true
		case uint8:
			num = float64(v)
			isNumeric = true
		case uint16:
			num = float64(v)
			isNumeric = true
		case uint32:
			num = float64(v)
			isNumeric = true
		case uint64:
			num = float64(v)
			isNumeric = true
		case float32:
			num = float64(v)
			isNumeric = true
		case float64:
			num = v
			isNumeric = true
		}

		if isNumeric {
			numbers = append(numbers, num)
		}
	}

	if len(numbers) < 10 {
		return false
	}

	// Categorical codes typically have:
	// - Low unique ratio (few distinct values relative to total)
	// - Mostly integers
	// - Small number of unique values

	unique := make(map[float64]bool)
	integerCount := 0

	for _, num := range numbers {
		unique[num] = true
		if num == float64(int64(num)) {
			integerCount++
		}
	}

	uniqueRatio := float64(len(unique)) / float64(len(numbers))
	integerRatio := float64(integerCount) / float64(len(numbers))

	// Low cardinality + mostly integers suggests categorical codes
	return uniqueRatio < config.CategoricalUniqueRatio && integerRatio > config.CategoricalIntegerRatio
}

// computeQualityScore calculates an overall quality score
func (p *ProfilerAdapter) computeQualityScore(missingCount, totalCount, valueCount int) float64 {
	if totalCount == 0 {
		return 0.0
	}

	completeness := 1.0 - float64(missingCount)/float64(totalCount)

	// Simple quality score based on completeness
	return math.Max(0.0, completeness)
}

// computeNumericStats calculates statistics for numeric fields
func (p *ProfilerAdapter) computeNumericStats(values []interface{}) *profiling.NumericStats {
	if len(values) == 0 {
		return nil
	}

	var sum, min, max float64
	zeroCount := 0
	negativeCount := 0

	min = math.Inf(1)
	max = math.Inf(-1)

	for _, value := range values {
		var num float64
		switch v := value.(type) {
		case int:
			num = float64(v)
		case int32:
			num = float64(v)
		case int64:
			num = float64(v)
		case float32:
			num = float64(v)
		case float64:
			num = v
		default:
			continue
		}

		sum += num
		if num < min {
			min = num
		}
		if num > max {
			max = num
		}
		if num == 0 {
			zeroCount++
		}
		if num < 0 {
			negativeCount++
		}
	}

	count := float64(len(values))
	mean := sum / count

	return &profiling.NumericStats{
		Min:           min,
		Max:           max,
		Mean:          mean,
		ZeroCount:     zeroCount,
		NegativeCount: negativeCount,
	}
}

// computeCategoricalStats calculates statistics for categorical fields
func (p *ProfilerAdapter) computeCategoricalStats(values []interface{}) *profiling.CategoricalStats {
	if len(values) == 0 {
		return nil
	}

	freq := make(map[string]int)
	for _, value := range values {
		if str, ok := value.(string); ok {
			freq[str]++
		}
	}

	if len(freq) == 0 {
		return nil
	}

	// Find mode
	mode := ""
	modeFreq := 0
	for value, count := range freq {
		if count > modeFreq {
			mode = value
			modeFreq = count
		}
	}

	return &profiling.CategoricalStats{
		Mode:          mode,
		ModeFrequency: modeFreq,
	}
}

// computeTextStats calculates statistics for text fields
func (p *ProfilerAdapter) computeTextStats(values []interface{}) *profiling.TextStats {
	if len(values) == 0 {
		return nil
	}

	totalLength := 0
	hasNumbers := false
	hasSpecial := false

	for _, value := range values {
		if str, ok := value.(string); ok {
			totalLength += len(str)
			if strings.ContainsAny(str, "0123456789") {
				hasNumbers = true
			}
			if strings.ContainsAny(str, "!@#$%^&*()_+-=[]{}|;:,.<>?") {
				hasSpecial = true
			}
		}
	}

	avgLength := float64(totalLength) / float64(len(values))

	return &profiling.TextStats{
		AvgLength:       avgLength,
		HasNumbers:      hasNumbers,
		HasSpecialChars: hasSpecial,
	}
}

// computeCardinalityStats calculates cardinality statistics for a field
func (p *ProfilerAdapter) computeCardinalityStats(values []interface{}) profiling.CardinalityStats {
	if len(values) == 0 {
		return profiling.CardinalityStats{
			UniqueCount: 0,
			UniqueRatio: 0.0,
			TopValues:   []profiling.ValueCount{},
			Entropy:     0.0,
		}
	}

	// Count frequency of each value
	freq := make(map[string]int)
	for _, value := range values {
		key := p.valueToString(value)
		freq[key]++
	}

	uniqueCount := len(freq)
	uniqueRatio := float64(uniqueCount) / float64(len(values))

	// Calculate top values (limit to top 10)
	type freqPair struct {
		value string
		count int
	}
	pairs := make([]freqPair, 0, len(freq))
	for value, count := range freq {
		pairs = append(pairs, freqPair{value: value, count: count})
	}

	// Sort by frequency (descending)
	for i := 0; i < len(pairs)-1; i++ {
		for j := i + 1; j < len(pairs); j++ {
			if pairs[j].count > pairs[i].count {
				pairs[i], pairs[j] = pairs[j], pairs[i]
			}
		}
	}

	// Take top 10
	topN := 10
	if len(pairs) < topN {
		topN = len(pairs)
	}
	topValues := make([]profiling.ValueCount, topN)
	for i := 0; i < topN; i++ {
		topValues[i] = profiling.ValueCount{
			Value: pairs[i].value,
			Count: pairs[i].count,
			Ratio: float64(pairs[i].count) / float64(len(values)),
		}
	}

	// Calculate Shannon entropy
	entropy := 0.0
	for _, count := range freq {
		if count > 0 {
			prob := float64(count) / float64(len(values))
			entropy -= prob * math.Log2(prob)
		}
	}

	return profiling.CardinalityStats{
		UniqueCount: uniqueCount,
		UniqueRatio: uniqueRatio,
		TopValues:   topValues,
		Entropy:     entropy,
	}
}

// computeMissingStats calculates missing value statistics
func (p *ProfilerAdapter) computeMissingStats(missingCount, totalCount int, events []interface{}, fieldName string) profiling.MissingStats {
	if totalCount == 0 {
		return profiling.MissingStats{
			MissingCount:       0,
			MissingRate:        0.0,
			ConsecutiveMissing: 0,
		}
	}

	missingRate := float64(missingCount) / float64(totalCount)

	// Calculate consecutive missing values
	maxConsecutive := 0
	currentConsecutive := 0
	for _, event := range events {
		if eventMap, ok := event.(map[string]interface{}); ok {
			if value, exists := eventMap[fieldName]; !exists || value == nil {
				currentConsecutive++
				if currentConsecutive > maxConsecutive {
					maxConsecutive = currentConsecutive
				}
			} else {
				currentConsecutive = 0
			}
		}
	}

	return profiling.MissingStats{
		MissingCount:       missingCount,
		MissingRate:        missingRate,
		ConsecutiveMissing: maxConsecutive,
	}
}

// valueToString converts a value to a string representation for cardinality analysis
func (p *ProfilerAdapter) valueToString(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%.6f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
