package postgres

import (
	"context"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/ports"
)

// RegistryAdapter implements RegistryPort for PostgreSQL
type RegistryAdapter struct {
	// TODO: implement
}

// GetContract retrieves a variable contract by key
func (r *RegistryAdapter) GetContract(ctx context.Context, varKey string) (*dataset.VariableContract, error) {
	// TODO: implement database lookup
	// For now, return a stub contract
	return &dataset.VariableContract{
		VarKey:           core.VariableKey(varKey),
		AsOfMode:         dataset.AsOfLatestValue,
		StatisticalType:  dataset.TypeNumeric,
		WindowDays:       nil,
		ImputationPolicy: "zero_fill",
		ScalarGuarantee:  true,
	}, nil
}

// Ensure RegistryAdapter implements RegistryPort
var _ ports.RegistryPort = (*RegistryAdapter)(nil)
