package ports

import (
	"context"
	"math/rand"
)

// RNGPort provides seeded random number generation for deterministic operations
type RNGPort interface {
	// SeededStream creates a deterministic random number generator for a named operation
	SeededStream(ctx context.Context, name string, seed int64) (*rand.Rand, error)

	// Stream creates a deterministic RNG stream for a specific stage/relationship
	// This ensures permutation/stability stages produce identical results for the same run
	Stream(ctx context.Context, runID, stageName, relationshipKey string, baseSeed int64) (*rand.Rand, error)

	// ValidateSeed ensures the seed produces expected deterministic results
	ValidateSeed(ctx context.Context, name string, seed int64, expected []float64) error
}
