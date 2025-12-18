package stages

import (
	"fmt"
	"math"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/stats"
)

// PairwiseStage performs statistical tests between variable pairs
type PairwiseStage struct{}

// NewPairwiseStage creates a new pairwise stage
func NewPairwiseStage() *PairwiseStage {
	return &PairwiseStage{}
}

// Execute performs pairwise statistical tests on all variable pairs
func (p *PairwiseStage) Execute(bundle *dataset.MatrixBundle, stageConfig map[string]interface{}) ([]interface{}, error) {
	artifacts := make([]interface{}, 0)

	variables := bundle.Matrix.VariableKeys
	data := bundle.Matrix.Data

	// A5: Performance guardrails - explicit caps
	const (
		MaxVariables = 2000   // Maximum variables to analyze
		MaxPairs     = 500000 // Maximum variable pairs (to prevent O(n²) explosion)
		MaxRuntimeMs = 300000 // Maximum runtime: 5 minutes
	)

	numVars := len(variables)
	if numVars > MaxVariables {
		return nil, fmt.Errorf("too many variables: %d > %d", numVars, MaxVariables)
	}

	totalPairs := numVars * (numVars - 1) / 2 // Upper triangle only
	if totalPairs > MaxPairs {
		return nil, fmt.Errorf("too many variable pairs: %d > %d", totalPairs, MaxPairs)
	}

	// A5: Runtime monitoring
	startTime := time.Now()

	// Compute family ID for FDR correction
	familyID := stats.ComputeFamilyID(
		bundle.SnapshotID,
		bundle.CohortHash,
		"pairwise",                         // stage name
		stats.TestPearson,                  // test type (TODO: make configurable)
		core.RegistryHash("test-registry"), // TODO: get from bundle
		core.Hash("test-stage-plan"),       // TODO: get from stage config
	)

	// Perform pairwise tests for i < j (upper triangle only)
	for i := 0; i < len(variables)-1; i++ {
		// A5: Periodic runtime check (every 100 variable pairs)
		if i%100 == 0 && time.Since(startTime).Milliseconds() > MaxRuntimeMs {
			return nil, fmt.Errorf("pairwise stage exceeded maximum runtime: %d ms", MaxRuntimeMs)
		}

		for j := i + 1; j < len(variables); j++ {
			var1 := variables[i]
			var2 := variables[j]

			// Extract columns
			col1 := p.extractColumn(data, i)
			col2 := p.extractColumn(data, j)

			// Perform appropriate statistical test
			relationship := p.analyzeRelationship(var1, var2, col1, col2, familyID)

			if relationship != nil {
				artifacts = append(artifacts, relationship)
			}
		}
	}

	// AC2: Apply BH FDR correction to all relationship artifacts
	p.applyFDRCorrection(artifacts)

	// Create FDR family artifact with correction method
	fdrFamily := stats.NewFDRFamilyArtifact(
		stats.FamilyKey{
			SnapshotID:    bundle.SnapshotID,
			CohortHash:    bundle.CohortHash,
			StageName:     "pairwise",
			TestType:      stats.TestPearson,
			RegistryHash:  core.RegistryHash("test-registry"),
			StagePlanHash: core.Hash("test-stage-plan"),
		},
		len(artifacts),
		"BH", // Benjamini-Hochberg FDR correction applied
	)
	artifacts = append(artifacts, fdrFamily)

	return artifacts, nil
}

// extractColumn extracts a column from the data matrix
func (p *PairwiseStage) extractColumn(data [][]float64, colIndex int) []float64 {
	if len(data) == 0 {
		return nil
	}
	if colIndex >= len(data[0]) {
		return nil
	}

	column := make([]float64, len(data))
	for i, row := range data {
		if colIndex < len(row) {
			column[i] = row[colIndex]
		} else {
			// Pad with NaN if row is too short
			column[i] = math.NaN()
		}
	}
	return column
}

// applyFDRCorrection applies Benjamini-Hochberg FDR correction to relationship artifacts
func (p *PairwiseStage) applyFDRCorrection(artifacts []interface{}) {
	// Collect relationship artifacts for FDR correction
	var relationshipArtifacts []*RelationshipResult
	for _, artifact := range artifacts {
		if rel, ok := artifact.(*RelationshipResult); ok && !rel.Skipped {
			relationshipArtifacts = append(relationshipArtifacts, rel)
		}
	}

	if len(relationshipArtifacts) == 0 {
		return
	}

	m := len(relationshipArtifacts) // total number of tests

	// Sort by p-value ascending
	for i := 0; i < len(relationshipArtifacts)-1; i++ {
		for j := i + 1; j < len(relationshipArtifacts); j++ {
			if relationshipArtifacts[i].Metrics.PValue > relationshipArtifacts[j].Metrics.PValue {
				relationshipArtifacts[i], relationshipArtifacts[j] = relationshipArtifacts[j], relationshipArtifacts[i]
			}
		}
	}

	// Apply BH correction: q_i = p_i * (m / i)
	// where i is the rank (1-based)
	for i, rel := range relationshipArtifacts {
		rank := i + 1 // 1-based rank
		qValue := rel.Metrics.PValue * float64(m) / float64(rank)

		// Clamp q-value to [0, 1]
		if qValue > 1.0 {
			qValue = 1.0
		}

		rel.Metrics.QValue = qValue
		rel.Metrics.TotalComparisons = m
		rel.Metrics.FDRMethod = "BH"
	}
}

// RelationshipResult contains the result of a pairwise statistical test
type RelationshipResult struct {
	Key         stats.RelationshipKey  `json:"key"`
	Metrics     stats.CanonicalMetrics `json:"metrics"`
	DataQuality stats.DataQuality      `json:"data_quality"`
	Skipped     bool                   `json:"skipped"`
	SkipReason  stats.WarningCode      `json:"skip_reason,omitempty"`
}

// analyzeRelationship performs statistical analysis between two variables
func (p *PairwiseStage) analyzeRelationship(var1, var2 core.VariableKey, col1, col2 []float64, familyID core.Hash) *RelationshipResult {
	if len(col1) != len(col2) || len(col1) == 0 {
		return &RelationshipResult{
			Key: stats.RelationshipKey{
				VariableX: var1,
				VariableY: var2,
				TestType:  stats.TestPearson,
				FamilyID:  familyID,
			},
			Skipped:    true,
			SkipReason: stats.WarningLowN,
		}
	}

	// Debug: check column lengths
	_ = len(col1) // For debugging

	// Compute basic data quality for diagnostics (missingness, variance, unique counts).
	nTotal := len(col1)
	missingX := 0
	missingY := 0
	uniqueX := make(map[float64]struct{})
	uniqueY := make(map[float64]struct{})
	validX := make([]float64, 0, nTotal)
	validY := make([]float64, 0, nTotal)
	for i := 0; i < nTotal; i++ {
		if math.IsNaN(col1[i]) {
			missingX++
		} else {
			uniqueX[col1[i]] = struct{}{}
			validX = append(validX, col1[i])
		}
		if math.IsNaN(col2[i]) {
			missingY++
		} else {
			uniqueY[col2[i]] = struct{}{}
			validY = append(validY, col2[i])
		}
	}

	dq := stats.DataQuality{
		MissingRateX: float64(missingX) / float64(nTotal),
		MissingRateY: float64(missingY) / float64(nTotal),
		UniqueCountX: len(uniqueX),
		UniqueCountY: len(uniqueY),
		VarianceX:    p.variance(validX),
		VarianceY:    p.variance(validY),
		CardinalityX: len(uniqueX),
		CardinalityY: len(uniqueY),
	}

	// Skip early if either variable is too missing (matches domain warning semantics).
	if dq.MissingRateX > 0.30 || dq.MissingRateY > 0.30 {
		return &RelationshipResult{
			Key: stats.RelationshipKey{
				VariableX: var1,
				VariableY: var2,
				TestType:  stats.TestPearson,
				FamilyID:  familyID,
			},
			Metrics: stats.CanonicalMetrics{
				EffectSize:       0.0,
				PValue:           1.0,
				SampleSize:       0,
				TotalComparisons: 1,
			},
			DataQuality: dq,
			Skipped:     true,
			SkipReason:  stats.WarningHighMissing,
		}
	}

	// Filter out NaN values and create paired data
	validPairs := make([][2]float64, 0, len(col1))
	for i := 0; i < len(col1); i++ {
		if !math.IsNaN(col1[i]) && !math.IsNaN(col2[i]) {
			validPairs = append(validPairs, [2]float64{col1[i], col2[i]})
		}
	}

	if len(validPairs) < 3 {
		return &RelationshipResult{
			Key: stats.RelationshipKey{
				VariableX: var1,
				VariableY: var2,
				TestType:  stats.TestPearson,
			},
			Metrics: stats.CanonicalMetrics{
				EffectSize:       0.0,
				PValue:           1.0,
				SampleSize:       len(validPairs),
				TotalComparisons: 1,
			},
			DataQuality: dq,
			Skipped:     true,
			SkipReason:  stats.WarningLowN,
		}
	}

	// Check for zero variance
	var1Values := make([]float64, len(validPairs))
	var2Values := make([]float64, len(validPairs))
	for i, pair := range validPairs {
		var1Values[i] = pair[0]
		var2Values[i] = pair[1]
	}

	if p.hasZeroVariance(var1Values) || p.hasZeroVariance(var2Values) {
		return &RelationshipResult{
			Key: stats.RelationshipKey{
				VariableX: var1,
				VariableY: var2,
				TestType:  stats.TestPearson,
				FamilyID:  familyID,
			},
			Metrics: stats.CanonicalMetrics{
				EffectSize:       0.0,
				PValue:           1.0,
				SampleSize:       len(validPairs),
				TotalComparisons: 1,
			},
			DataQuality: dq,
			Skipped:     true,
			SkipReason:  stats.WarningLowVariance,
		}
	}

	// Perform Pearson correlation (simplified - assumes continuous variables)
	corr, pValue := p.pearsonCorrelation(var1Values, var2Values)

	return &RelationshipResult{
		Key: stats.RelationshipKey{
			VariableX: var1,
			VariableY: var2,
			TestType:  stats.TestPearson,
			FamilyID:  familyID,
		},
		Metrics: stats.CanonicalMetrics{
			EffectSize:       corr,
			EffectUnit:       "r", // Pearson correlation coefficient
			PValue:           pValue,
			SampleSize:       len(validPairs),
			TotalComparisons: 1,
			FDRMethod:        "none", // No FDR correction for single test
		},
		DataQuality: dq,
		Skipped:     false,
	}
}

// variance computes sample variance (n-1) for numeric values.
func (p *PairwiseStage) variance(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))
	sumSq := 0.0
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return sumSq / float64(len(values)-1)
}

// hasZeroVariance checks if a variable has essentially zero variance
func (p *PairwiseStage) hasZeroVariance(values []float64) bool {
	if len(values) < 2 {
		return true
	}

	first := values[0]
	for _, v := range values[1:] {
		if math.Abs(v-first) > 1e-10 { // Very small threshold
			return false
		}
	}
	return true
}

// pearsonCorrelation calculates Pearson correlation coefficient and p-value
// This is a simplified implementation - in production you'd use a proper statistical library
func (p *PairwiseStage) pearsonCorrelation(x, y []float64) (correlation, pValue float64) {
	if len(x) != len(y) || len(x) < 2 {
		return 0, 1.0
	}

	n := float64(len(x))

	// Calculate means
	sumX, sumY := 0.0, 0.0
	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
	}
	meanX := sumX / n
	meanY := sumY / n

	// Calculate correlation
	numerator := 0.0
	sumXX := 0.0
	sumYY := 0.0

	for i := 0; i < len(x); i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		numerator += dx * dy
		sumXX += dx * dx
		sumYY += dy * dy
	}

	if sumXX == 0 || sumYY == 0 {
		return 0, 1.0
	}

	correlation = numerator / math.Sqrt(sumXX*sumYY)

	// Clamp to [-1, 1] due to floating point precision
	if correlation > 1.0 {
		correlation = 1.0
	} else if correlation < -1.0 {
		correlation = -1.0
	}

	// Simplified p-value calculation (t-distribution approximation)
	// This is not statistically rigorous - use proper statistical libraries in production
	t := math.Abs(correlation) * math.Sqrt(float64(n-2)/(1-correlation*correlation))
	// Very rough approximation - in reality, use statistical tables or libraries
	if t > 3.0 {
		pValue = 0.001
	} else if t > 2.0 {
		pValue = 0.01
	} else if t > 1.5 {
		pValue = 0.05
	} else {
		pValue = 0.1
	}

	return correlation, pValue
}
