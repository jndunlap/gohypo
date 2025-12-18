package ports

import (
	"context"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/verdict"
)

// BatteryPort runs hypothesis validation tests
type BatteryPort interface {
	ValidateHypothesis(ctx context.Context, hypothesisID core.HypothesisID, matrixBundle *dataset.MatrixBundle) (*ValidationResult, error)
}

// ValidationResult contains the outcome of hypothesis validation
type ValidationResult struct {
	HypothesisID       core.HypothesisID
	Status             verdict.VerdictStatus
	Reason             verdict.RejectionReason
	PValue             float64
	Confidence         float64
	EffectSize         float64
	NullPercentile     float64
	FalsificationLog   *verdict.FalsificationLog
	NumPermutations    int
	ValidationMetadata map[string]interface{}
}
