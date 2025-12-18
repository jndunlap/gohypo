package snapshot

import (
	"time"

	"gohypo/domain/core"
)

// Snapshot represents a point-in-time data view (time machine handle)
type Snapshot struct {
	ID           core.ID
	Dataset      string
	SnapshotAt   core.Timestamp
	Lag          core.Lag
	RegistryHash core.RegistryHash
	CohortHash   core.CohortHash
	Seed         int64
	CreatedAt    core.Timestamp
}

// SnapshotSpec defines parameters for creating a snapshot
type SnapshotSpec struct {
	Dataset      string
	SnapshotAt   time.Time
	LagSeconds   int // lag in seconds for DB storage
	RegistryHash core.RegistryHash
	CohortHash   core.CohortHash
	Seed         int64
}

// NewSnapshot creates a snapshot from spec
func NewSnapshot(spec SnapshotSpec) *Snapshot {
	return &Snapshot{
		ID:           core.NewID(),
		Dataset:      spec.Dataset,
		SnapshotAt:   core.Timestamp(core.NewSnapshotAt(spec.SnapshotAt)),
		Lag:          core.NewLag(time.Duration(spec.LagSeconds) * time.Second),
		RegistryHash: spec.RegistryHash,
		CohortHash:   spec.CohortHash,
		Seed:         spec.Seed,
		CreatedAt:    core.Now(),
	}
}

// GetCutoff calculates the cutoff timestamp (snapshot_at - lag)
func (s *Snapshot) GetCutoff() core.Timestamp {
	return core.Timestamp(core.SnapshotAt(s.SnapshotAt).ApplyLag(s.Lag))
}

// DatasetView represents a filtered dataset with cohort selection
type DatasetView struct {
	ID         core.ID
	Dataset    string
	Filters    map[string]interface{} // cohort selection criteria
	EntityIDs  []core.ID              // resolved cohort entities
	CohortHash core.CohortHash        // hash of selection criteria
	CreatedAt  core.Timestamp
}

// NewDatasetView creates a dataset view with cohort
func NewDatasetView(dataset string, filters map[string]interface{}, entityIDs []core.ID) *DatasetView {
	// Convert []core.ID to []string for ComputeCohortHash
	entityIDStrings := make([]string, len(entityIDs))
	for i, id := range entityIDs {
		entityIDStrings[i] = string(id)
	}

	cohortHash := core.ComputeCohortHash(entityIDStrings, filters)

	return &DatasetView{
		ID:         core.NewID(),
		Dataset:    dataset,
		Filters:    filters,
		EntityIDs:  entityIDs,
		CohortHash: cohortHash,
		CreatedAt:  core.Now(),
	}
}
