package datareadiness

import (
	"context"
	"fmt"
	"math"
	"strings"

	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
	"gohypo/domain/datareadiness/profiling"
)

// ProfilerAdapter implements ProfilerPort for data profiling
type ProfilerAdapter struct{}

// NewProfilerAdapter creates a new profiler adapter
func NewProfilerAdapter() *ProfilerAdapter {
	return &ProfilerAdapter{}
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
		profile := p.profileField(fieldName, sourceName, sampleData)
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
func (p *ProfilerAdapter) profileField(fieldName, sourceName string, events []interface{}) profiling.FieldProfile {
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

	// Infer type from values
	inferredType := p.inferType(values)

	// Create profile
	profile := profiling.FieldProfile{
		FieldKey:     fieldName,
		Source:       sourceName,
		SampleSize:   totalCount,
		InferredType: inferredType,
		QualityScore: p.computeQualityScore(missingCount, totalCount, len(values)),
		ComputedAt:   core.Now(),
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

// inferType determines the most likely data type from a sample of values
func (p *ProfilerAdapter) inferType(values []interface{}) profiling.InferredType {
	if len(values) == 0 {
		return profiling.TypeUnknown
	}

	numericCount := 0
	textCount := 0
	boolCount := 0

	for _, value := range values {
		switch v := value.(type) {
		case int, int32, int64, float32, float64:
			numericCount++
		case string:
			str := strings.TrimSpace(v)
			if str == "true" || str == "false" {
				boolCount++
			} else {
				textCount++
			}
		case bool:
			boolCount++
		default:
			// Keep as unknown
		}
	}

	total := len(values)
	if float64(numericCount)/float64(total) > 0.8 {
		return profiling.TypeNumeric
	}
	if float64(boolCount)/float64(total) > 0.8 {
		return profiling.TypeBoolean
	}
	if float64(textCount)/float64(total) > 0.5 {
		return profiling.TypeText
	}

	return profiling.TypeCategorical
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
