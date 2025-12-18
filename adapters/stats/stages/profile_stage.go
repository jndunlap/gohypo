package stages

import (
	"math"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

// ProfileStage analyzes individual variables for statistical properties
type ProfileStage struct{}

// NewProfileStage creates a new profile stage
func NewProfileStage() *ProfileStage {
	return &ProfileStage{}
}

// Execute performs profiling analysis on all variables in the matrix bundle
func (p *ProfileStage) Execute(bundle *dataset.MatrixBundle, stageConfig map[string]interface{}) ([]interface{}, error) {
	artifacts := make([]interface{}, 0, len(bundle.Matrix.VariableKeys))

	for i, varKey := range bundle.Matrix.VariableKeys {
		profile := p.profileVariable(varKey, bundle.Matrix.Data, i)

		// Create stats profile artifact
		artifact := map[string]interface{}{
			"variable_key":     string(varKey),
			"missing_rate":     profile.MissingRate,
			"variance":         profile.Variance,
			"cardinality":      profile.Cardinality,
			"zero_variance":    profile.ZeroVariance,
			"high_cardinality": profile.HighCardinality,
			"sample_size":      profile.SampleSize,
		}

		artifacts = append(artifacts, artifact)
	}

	return artifacts, nil
}

// VariableProfile contains statistical profile for a single variable
type VariableProfile struct {
	MissingRate     float64
	Variance        float64
	Cardinality     int
	ZeroVariance    bool
	HighCardinality bool
	SampleSize      int
}

// profileVariable analyzes a single variable column
func (p *ProfileStage) profileVariable(varKey core.VariableKey, data [][]float64, colIndex int) VariableProfile {
	if len(data) == 0 || colIndex >= len(data[0]) {
		return VariableProfile{SampleSize: 0}
	}

	column := make([]float64, len(data))
	validCount := 0
	valueSet := make(map[float64]bool)

	for i, row := range data {
		if colIndex < len(row) {
			val := row[colIndex]
			column[i] = val
			// Consider non-NaN values as valid (simplified - in reality check for proper missing indicators)
			if !math.IsNaN(val) {
				validCount++
				valueSet[val] = true
			}
		}
	}

	sampleSize := len(data)
	missingRate := 1.0 - float64(validCount)/float64(sampleSize)

	// Calculate variance
	variance := p.calculateVariance(column[:validCount])
	zeroVariance := variance < 1e-10 // Very small threshold for zero variance

	// Cardinality analysis
	cardinality := len(valueSet)
	highCardinality := float64(cardinality)/float64(validCount) > 0.9 // >90% unique values

	return VariableProfile{
		MissingRate:     missingRate,
		Variance:        variance,
		Cardinality:     cardinality,
		ZeroVariance:    zeroVariance,
		HighCardinality: highCardinality,
		SampleSize:      sampleSize,
	}
}

// calculateVariance computes sample variance
func (p *ProfileStage) calculateVariance(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}

	// Sample variance (divide by n-1)
	return sumSq / float64(len(values)-1)
}
