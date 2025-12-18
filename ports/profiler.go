package ports

import (
	"context"

	"gohypo/domain/datareadiness/ingestion"
	"gohypo/domain/datareadiness/profiling"
)

// ProfilerPort analyzes data to extract statistical profiles
type ProfilerPort interface {
	ProfileSource(ctx context.Context, sourceName string, events []ingestion.CanonicalEvent, config profiling.ProfilingConfig) (*profiling.ProfilingResult, error)
}
