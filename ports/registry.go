package ports

import (
	"context"

	"gohypo/domain/dataset"
)

// RegistryPort manages variable contracts
type RegistryPort interface {
	GetContract(ctx context.Context, varKey string) (*dataset.VariableContract, error)
}
