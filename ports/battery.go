package ports

import (
	"context"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

// BatteryPort runs hypothesis validation tests
// TODO: Implement when battery stages are ready
type BatteryPort interface {
	ValidateHypothesis(ctx context.Context, hypothesisID core.HypothesisID, matrixBundle *dataset.MatrixBundle) (*ValidationResult, error)
}

// ValidationResult contains the outcome of hypothesis validation
type ValidationResult struct {
	// TODO: define validation result structure
}
