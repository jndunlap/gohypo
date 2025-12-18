package engine

import (
	"context"
	"math"

	"gohypo/adapters/stats/senses"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/stage"
)

// RelationshipArtifact represents a statistical relationship (internal to engine)
type RelationshipArtifact struct {
	VariableX        core.VariableKey     `json:"variable_x"`
	VariableY        core.VariableKey     `json:"variable_y"`
	TestUsed         string               `json:"test_used"`
	EffectSize       float64              `json:"effect_size"`
	PValue           float64              `json:"p_value"`
	PermutationP     float64              `json:"permutation_p"`
	StabilityScore   float64              `json:"stability_score"`
	PhantomBenchmark float64              `json:"phantom_benchmark"`
	CohortSize       int                  `json:"cohort_size"`
	SenseResults     []senses.SenseResult `json:"sense_results,omitempty"` // All five statistical senses
}

// executeProfileStage profiles the dataset columns
func (e *StatsEngine) executeProfileStage(ctx context.Context, bundle *dataset.MatrixBundle, spec stage.StageSpec) (*stage.StageResult, error) {
	result := &stage.StageResult{
		StageName: spec.Name,
		Success:   true,
		Metrics: stage.StageMetrics{
			ProcessedCount: bundle.ColumnCount(),
		},
	}

	// Profile each column
	artifacts := make([]core.Artifact, 0, bundle.ColumnCount())
	for i := range bundle.ColumnMeta {
		profile := e.profileColumn(bundle, i)
		artifact := core.Artifact{
			ID:        core.NewID(),
			Kind:      core.ArtifactVariableHealth,
			Payload:   profile,
			CreatedAt: core.Now(),
		}
		artifacts = append(artifacts, artifact)
	}

	result.Artifacts = artifacts
	return result, nil
}

// executePairwiseStage runs pairwise statistical tests with all five senses
func (e *StatsEngine) executePairwiseStage(ctx context.Context, bundle *dataset.MatrixBundle, spec stage.StageSpec) (*stage.StageResult, error) {
	tests, ok := spec.Config["tests"].([]string)
	if !ok {
		tests = []string{"pearson"} // default
	}

	// Initialize sense engine for multi-dimensional analysis
	senseEngine := senses.NewSenseEngine()

	result := &stage.StageResult{
		StageName: spec.Name,
		Success:   true,
		Metrics: stage.StageMetrics{
			ProcessedCount: 0,
		},
	}

	artifacts := make([]core.Artifact, 0)

	// Generate all pairwise combinations
	for i := 0; i < bundle.ColumnCount(); i++ {
		for j := i + 1; j < bundle.ColumnCount(); j++ {
			metaX := bundle.ColumnMeta[i]
			metaY := bundle.ColumnMeta[j]

			// Select appropriate test based on types
			testName := e.selectTest(metaX.StatisticalType, metaY.StatisticalType, tests)

			// Run the test
			xData, _ := bundle.GetColumnData(metaX.VariableKey)
			yData, _ := bundle.GetColumnData(metaY.VariableKey)

			// Run traditional test for backward compatibility
			relArtifact := e.runPairwiseTest(testName, metaX.VariableKey, metaY.VariableKey, xData, yData, bundle.RowCount())

			// Run all five statistical senses for comprehensive analysis
			relArtifact.SenseResults = senseEngine.AnalyzeAll(ctx, xData, yData, metaX.VariableKey, metaY.VariableKey)

			// Convert to core.Artifact with enriched sense data
			payload := map[string]interface{}{
				"variable_x":        string(relArtifact.VariableX),
				"variable_y":        string(relArtifact.VariableY),
				"test_used":         relArtifact.TestUsed,
				"effect_size":       relArtifact.EffectSize,
				"p_value":           relArtifact.PValue,
				"permutation_p":     relArtifact.PermutationP,
				"stability_score":   relArtifact.StabilityScore,
				"phantom_benchmark": relArtifact.PhantomBenchmark,
				"cohort_size":       relArtifact.CohortSize,
			}

			// Add sense results to payload
			if len(relArtifact.SenseResults) > 0 {
				senseData := make([]map[string]interface{}, len(relArtifact.SenseResults))
				for k, sense := range relArtifact.SenseResults {
					senseData[k] = map[string]interface{}{
						"sense_name":  sense.SenseName,
						"effect_size": sense.EffectSize,
						"p_value":     sense.PValue,
						"confidence":  sense.Confidence,
						"signal":      sense.Signal,
						"description": sense.Description,
						"metadata":    sense.Metadata,
					}
				}
				payload["sense_results"] = senseData
			}

			artifact := core.Artifact{
				ID:        core.NewID(),
				Kind:      core.ArtifactRelationship,
				Payload:   payload,
				CreatedAt: core.Now(),
			}
			artifacts = append(artifacts, artifact)

			result.Metrics.ProcessedCount++
		}
	}

	result.Artifacts = artifacts
	return result, nil
}

// selectTest chooses the appropriate statistical test
func (e *StatsEngine) selectTest(typeX, typeY dataset.StatisticalType, availableTests []string) string {
	// Test selection logic based on variable types
	if typeX == dataset.TypeNumeric && typeY == dataset.TypeNumeric {
		if contains(availableTests, "spearman") {
			return "spearman"
		}
		return "pearson"
	}

	if (typeX == dataset.TypeCategorical || typeX == dataset.TypeBinary) &&
		(typeY == dataset.TypeCategorical || typeY == dataset.TypeBinary) {
		return "chisquare"
	}

	if typeX == dataset.TypeBinary && typeY == dataset.TypeNumeric {
		return "ttest"
	}

	// Default fallback
	return "pearson"
}

// runPairwiseTest executes a specific statistical test
// Uses real sense implementations for accurate results
func (e *StatsEngine) runPairwiseTest(testName string, varX, varY core.VariableKey, x, y []float64, n int) RelationshipArtifact {
	var effectSize, pValue float64
	ctx := context.Background()

	switch testName {
	case "pearson":
		effectSize, pValue = e.pearsonCorrelation(x, y)
	case "spearman":
		spearmanSense := senses.NewSpearmanSense()
		result := spearmanSense.Analyze(ctx, x, y, varX, varY)
		effectSize, pValue = result.EffectSize, result.PValue
	case "chisquare":
		chiSense := senses.NewChiSquareSense()
		result := chiSense.Analyze(ctx, x, y, varX, varY)
		effectSize, pValue = result.EffectSize, result.PValue
	case "ttest":
		tSense := senses.NewWelchTTestSense()
		result := tSense.Analyze(ctx, x, y, varX, varY)
		effectSize, pValue = result.EffectSize, result.PValue
	default:
		effectSize, pValue = 0, 1.0
	}

	return RelationshipArtifact{
		VariableX:      varX,
		VariableY:      varY,
		TestUsed:       testName,
		EffectSize:     effectSize,
		PValue:         pValue,
		StabilityScore: 0, // computed in stability stage
		CohortSize:     n,
	}
}

// pearsonCorrelation calculates Pearson correlation coefficient
func (e *StatsEngine) pearsonCorrelation(x, y []float64) (float64, float64) {
	if len(x) != len(y) || len(x) == 0 {
		return 0, 1.0
	}

	n := float64(len(x))
	sumX, sumY := 0.0, 0.0
	sumXY, sumX2, sumY2 := 0.0, 0.0, 0.0

	for i := 0; i < len(x); i++ {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
		sumY2 += y[i] * y[i]
	}

	numerator := n*sumXY - sumX*sumY
	denominator := math.Sqrt((n*sumX2 - sumX*sumX) * (n*sumY2 - sumY*sumY))

	if denominator == 0 {
		return 0, 1.0
	}

	r := numerator / denominator

	// Simplified p-value calculation (would use proper statistical library)
	t := r * math.Sqrt((n-2)/(1-r*r))
	pValue := 2 * (1 - e.tCDF(math.Abs(t), int(n-2)))

	return r, pValue
}

// Dead code removed - now using real sense implementations from adapters/stats/senses/

func (e *StatsEngine) tCDF(t float64, df int) float64 {
	// Simplified t-distribution CDF approximation
	// For MVP, using a basic approximation
	// In production, would use a proper statistical library like gonum/stat

	if df <= 0 {
		return 0.5
	}

	// For large degrees of freedom, approximate with normal distribution
	if df > 30 {
		// Normal approximation using error function approximation
		// CDF(t) ≈ 0.5 * (1 + erf(t/sqrt(2)))
		// Using approximation: erf(x) ≈ tanh(1.128*x) for small x
		z := t / math.Sqrt2
		if math.Abs(z) < 2.0 {
			erfApprox := math.Tanh(1.128 * z)
			return 0.5 * (1 + erfApprox)
		}
		// For large z, approximate tail
		if z > 0 {
			return 1.0 - (0.5 * math.Exp(-z*z))
		}
		return 0.5 * math.Exp(-z*z)
	}

	// For smaller df, use a simplified approximation
	// Approximate: CDF(t) ≈ 0.5 + (t / (2 * sqrt(df))) for small t
	if math.Abs(t) < 1.0 {
		return 0.5 + (t / (2.0 * math.Sqrt(float64(df))))
	}

	// For larger t values, approximate tail probabilities
	if t > 0 {
		return 1.0 - (0.5 / (1.0 + t*t/float64(df)))
	}
	return 0.5 / (1.0 + t*t/float64(df))
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
