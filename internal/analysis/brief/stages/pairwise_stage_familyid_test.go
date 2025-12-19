package stages

import (
	"testing"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/stats"
)

func TestPairwiseStage_SetsFamilyIDAndTestType(t *testing.T) {
	stage := NewPairwiseStage()

	bundle := dataset.NewMatrixBundle(
		core.SnapshotID("test-snapshot"),
		core.NewID(),
		core.CohortHash("test-cohort"),
		core.NewCutoffAt(core.Now().Time()),
		core.NewLag(0),
	)

	// 10 rows, 3 vars
	vars := []core.VariableKey{"x", "y", "z"}
	rows := 10
	bundle.Matrix = dataset.Matrix{
		EntityIDs:    make([]core.ID, rows),
		VariableKeys: vars,
		Data:         make([][]float64, rows),
	}
	for i := 0; i < rows; i++ {
		bundle.Matrix.EntityIDs[i] = core.NewID()
		bundle.Matrix.Data[i] = []float64{float64(i), float64(i) * 2, float64(i) + 7}
	}
	bundle.ColumnMeta = make([]dataset.ColumnMeta, len(vars))
	for i, v := range vars {
		bundle.ColumnMeta[i] = dataset.ColumnMeta{VariableKey: v, StatisticalType: dataset.TypeNumeric}
	}

	artifacts, err := stage.Execute(bundle, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	foundRel := 0
	for _, a := range artifacts {
		rel, ok := a.(*RelationshipResult)
		if !ok {
			continue
		}
		foundRel++
		if rel.Key.TestType != stats.TestPearson {
			t.Fatalf("expected TestType=%s, got %s", stats.TestPearson, rel.Key.TestType)
		}
		if rel.Key.FamilyID == "" {
			t.Fatalf("expected FamilyID to be set, got empty")
		}
	}

	if foundRel == 0 {
		t.Fatalf("expected at least one RelationshipResult, got 0")
	}
}
