package stages

import (
	"context"
	"fmt"
	"math"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/stats"
	brief "gohypo/internal/analysis/brief"
)

// PairwiseStage performs statistical tests between variable pairs using unified brief system
type PairwiseStage struct {
	engine *brief.StatisticalEngine
}

// NewPairwiseStage creates a new pairwise stage with statistical engine
func NewPairwiseStage() *PairwiseStage {
	return &PairwiseStage{
		engine: brief.NewStatisticalEngine(),
	}
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

// analyzeRelationship performs statistical analysis between two variables using unified brief system
func (p *PairwiseStage) analyzeRelationship(var1, var2 core.VariableKey, col1, col2 []float64, familyID core.Hash) *RelationshipResult {
	// Use unified brief system for all statistical analysis
	analysis, err := p.engine.AnalyzeRelationship(context.Background(), col1, col2, "correlation", var1, var2)
	if err != nil {
		// Return skipped result on analysis error
		return &RelationshipResult{
			Key: stats.RelationshipKey{
				VariableX: var1,
				VariableY: var2,
				TestType:  stats.TestPearson,
				FamilyID:  familyID,
			},
			Skipped:    true,
			SkipReason: stats.WarningLowN, // Generic skip reason
		}
	}

	// Convert brief-based analysis to legacy RelationshipResult format
	// This maintains backward compatibility while using the new unified system
	result := &RelationshipResult{
		Key: stats.RelationshipKey{
			VariableX: var1,
			VariableY: var2,
			TestType:  stats.TestPearson,
			FamilyID:  familyID,
		},
		Metrics: stats.CanonicalMetrics{
			EffectSize:       analysis.PrimaryMetrics.EffectSize,
			EffectUnit:       "r", // Pearson correlation coefficient
			PValue:           analysis.PrimaryMetrics.PValue,
			SampleSize:       analysis.SampleSize,
			TotalComparisons: 1,
			FDRMethod:        "none", // No FDR correction for single test
		},
		DataQuality: stats.NewDataQualityFromBrief(analysis.Brief),
		Skipped:     false,
	}

	return result
}
