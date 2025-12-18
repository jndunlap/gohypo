package ports

import (
	"context"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

// MatrixResolverPort resolves variables to matrices with audit trails
type MatrixResolverPort interface {
	// ResolveMatrix produces a MatrixBundle for the given snapshot and variables
	ResolveMatrix(ctx context.Context, req MatrixResolutionRequest) (*dataset.MatrixBundle, error)
}

// MatrixResolutionRequest defines the parameters for matrix resolution
type MatrixResolutionRequest struct {
	ViewID     core.ID            // dataset view identifier
	SnapshotID core.SnapshotID    // snapshot identifier
	EntityIDs  []core.ID          // entities to include (cohort)
	VarKeys    []core.VariableKey // variables to resolve
}
