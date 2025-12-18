package engine

import (
	"math"

	"gohypo/adapters/stats/senses"
	"gohypo/domain/dataset"
)

// StatsEngine provides statistical computation capabilities
type StatsEngine struct {
	senseEngine *senses.SenseEngine
	// Engine can be extended with configuration, caching, etc.
}

// NewStatsEngine creates a new statistical engine
func NewStatsEngine() *StatsEngine {
	return &StatsEngine{
		senseEngine: senses.NewSenseEngine(),
	}
}

// profileColumn profiles a single column from the matrix bundle
func (e *StatsEngine) profileColumn(bundle *dataset.MatrixBundle, colIndex int) map[string]interface{} {
	if colIndex >= len(bundle.ColumnMeta) {
		return map[string]interface{}{
			"error": "column index out of range",
		}
	}

	meta := bundle.ColumnMeta[colIndex]
	columnData, ok := bundle.GetColumnData(meta.VariableKey)
	if !ok {
		return map[string]interface{}{
			"error": "failed to get column data",
		}
	}

	// Calculate basic statistics
	validCount := 0
	valueSet := make(map[float64]bool)
	sum := 0.0

	for _, val := range columnData {
		if !math.IsNaN(val) {
			validCount++
			valueSet[val] = true
			sum += val
		}
	}

	sampleSize := len(columnData)
	missingRate := 1.0 - float64(validCount)/float64(sampleSize)
	mean := sum / float64(validCount)

	// Calculate variance
	variance := 0.0
	if validCount > 1 {
		sumSq := 0.0
		for _, val := range columnData {
			if !math.IsNaN(val) {
				diff := val - mean
				sumSq += diff * diff
			}
		}
		variance = sumSq / float64(validCount-1)
	}

	cardinality := len(valueSet)
	zeroVariance := variance < 1e-10
	highCardinality := float64(cardinality)/float64(validCount) > 0.9

	return map[string]interface{}{
		"variable_key":     string(meta.VariableKey),
		"statistical_type": string(meta.StatisticalType),
		"missing_rate":     missingRate,
		"variance":         variance,
		"cardinality":      cardinality,
		"zero_variance":    zeroVariance,
		"high_cardinality": highCardinality,
		"sample_size":      sampleSize,
		"mean":             mean,
	}
}
