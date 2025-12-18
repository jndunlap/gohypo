package stages

import (
	"strings"
	"testing"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

func TestPairwiseStage_PerformanceGuardrails(t *testing.T) {
	stage := NewPairwiseStage()

	t.Run("too many variables", func(t *testing.T) {
		// A5: Test variable cap (2000 max)
		bundle := createLargeBundle(2001, 10) // 2001 variables

		_, err := stage.Execute(bundle, nil)
		if err == nil {
			t.Error("Expected error for too many variables")
		}
		if !containsString(err.Error(), "too many variables") {
			t.Errorf("Expected 'too many variables' error, got: %v", err)
		}
	})

	t.Run("too many pairs", func(t *testing.T) {
		// A5: Test pair cap (500,000 max)
		// With 1100 variables: pairs = 1100 * 1099 / 2 = 604,450 (over limit)
		bundle := createLargeBundle(1100, 10)

		_, err := stage.Execute(bundle, nil)
		if err == nil {
			t.Error("Expected error for too many pairs")
		}
		if !containsString(err.Error(), "too many variable pairs") {
			t.Errorf("Expected 'too many variable pairs' error, got: %v", err)
		}
	})

	t.Run("within limits", func(t *testing.T) {
		// Should work within limits
		bundle := createLargeBundle(100, 10) // 100 vars = 4,950 pairs < 500k limit

		artifacts, err := stage.Execute(bundle, nil)
		if err != nil {
			t.Errorf("Expected success within limits, got error: %v", err)
		}
		if len(artifacts) == 0 {
			t.Error("Expected artifacts to be generated")
		}
	})
}

// Helper functions

func createLargeBundle(numVars, rowsPerVar int) *dataset.MatrixBundle {
	bundle := dataset.NewMatrixBundle(
		core.SnapshotID("test-snapshot"),
		core.NewID(),
		core.CohortHash("test-cohort"),
		core.NewCutoffAt(core.Now().Time()),
		core.NewLag(24*60*60*1000),
	)

	// Create variable keys
	varKeys := make([]core.VariableKey, numVars)
	for i := 0; i < numVars; i++ {
		varKeys[i] = core.VariableKey(core.NewID())
	}

	// Create data matrix
	data := make([][]float64, rowsPerVar)
	entityIDs := make([]core.ID, rowsPerVar)

	for i := 0; i < rowsPerVar; i++ {
		entityIDs[i] = core.NewID()
		data[i] = make([]float64, numVars)
		for j := 0; j < numVars; j++ {
			data[i][j] = float64(i*numVars + j) // Simple deterministic values
		}
	}

	matrix := dataset.Matrix{
		Data:         data,
		EntityIDs:    entityIDs,
		VariableKeys: varKeys,
	}

	bundle.Matrix = matrix

	// Add minimal metadata
	bundle.ColumnMeta = make([]dataset.ColumnMeta, numVars)
	for i, varKey := range varKeys {
		bundle.ColumnMeta[i] = dataset.ColumnMeta{
			VariableKey:     varKey,
			StatisticalType: dataset.StatisticalType("numeric"),
		}
	}

	return bundle
}

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}
