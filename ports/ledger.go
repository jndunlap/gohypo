package ports

import (
	"context"
	"gohypo/domain/core"
	"gohypo/domain/run"
)

// LedgerWriterPort provides append-only write access to artifacts
// This is the ONLY way to write artifacts - prevents read-after-write coupling
type LedgerWriterPort interface {
	StoreArtifact(ctx context.Context, runID string, artifact core.Artifact) error
}

// LedgerReaderPort provides read-only access to stored artifacts
// Use this for queries, replay, and UI/API access
type LedgerReaderPort interface {
	// Artifact queries (read-only)
	ListArtifacts(ctx context.Context, filters ArtifactFilters) ([]core.Artifact, error)
	GetArtifact(ctx context.Context, artifactID core.ArtifactID) (*core.Artifact, error)
	GetArtifactsByRun(ctx context.Context, runID core.RunID) ([]core.Artifact, error)
	GetArtifactsByKind(ctx context.Context, kind core.ArtifactKind, limit int) ([]core.Artifact, error)

	// Run manifest queries
	GetRunManifest(ctx context.Context, runID core.RunID) (*run.RunManifestArtifact, error)
}

// ArtifactFilters for querying artifacts
type ArtifactFilters struct {
	RunID   *core.RunID
	Kind    *core.ArtifactKind
	VarKeys []core.VariableKey
	Limit   int
	Offset  int
}

// LedgerPort combines read and write access (for backwards compatibility during transition)
type LedgerPort interface {
	LedgerWriterPort
	LedgerReaderPort
}
