package run

import (
	"gohypo/domain/core"
	"gohypo/domain/stage"
)

// RunManifestArtifact represents the complete specification for a run
// This is the "truth source" for replay - must exist before any stage artifacts
type RunManifestArtifact struct {
	RunID         core.RunID         `json:"run_id"`
	SnapshotID    core.SnapshotID    `json:"snapshot_id"`
	SnapshotAt    core.SnapshotAt    `json:"snapshot_at"`
	Lag           core.Lag           `json:"lag"`
	CutoffAt      core.CutoffAt      `json:"cutoff_at"`
	RegistryHash  core.RegistryHash  `json:"registry_hash"`
	CohortHash    core.CohortHash    `json:"cohort_hash"`
	StagePlanHash core.StageListHash `json:"stage_plan_hash"`
	Seed          int64              `json:"seed"`
	CodeVersion   string             `json:"code_version"`
	Fingerprint   RunFingerprint     `json:"fingerprint"` // Determinism fingerprint
	CreatedAt     core.Timestamp     `json:"created_at"`
}

// NewRunManifestArtifact creates a run manifest from a pipeline request
func NewRunManifestArtifact(
	runID core.RunID,
	snapshotID core.SnapshotID,
	snapshotAt core.SnapshotAt,
	lag core.Lag,
	cutoffAt core.CutoffAt,
	registryHash core.RegistryHash,
	cohortHash core.CohortHash,
	stagePlan *stage.StagePlan,
	seed int64,
	codeVersion string,
) *RunManifestArtifact {
	stagePlanHash := stagePlan.Hash()
	fingerprint := NewRunFingerprint(snapshotID, registryHash, cohortHash, core.Hash(stagePlanHash), seed, codeVersion)

	return &RunManifestArtifact{
		RunID:         runID,
		SnapshotID:    snapshotID,
		SnapshotAt:    snapshotAt,
		Lag:           lag,
		CutoffAt:      cutoffAt,
		RegistryHash:  registryHash,
		CohortHash:    cohortHash,
		StagePlanHash: stagePlanHash,
		Seed:          seed,
		CodeVersion:   codeVersion,
		Fingerprint:   fingerprint,
		CreatedAt:     core.Now(),
	}
}

// ToCoreArtifact converts to a core artifact for storage
func (r *RunManifestArtifact) ToCoreArtifact() core.Artifact {
	return core.Artifact{
		ID:        core.NewID(),
		Kind:      core.ArtifactRun,
		Payload:   r,
		CreatedAt: r.CreatedAt,
	}
}

// Validate checks if the manifest is complete
func (r *RunManifestArtifact) Validate() error {
	if core.ID(r.RunID).IsEmpty() {
		return core.NewValidationError("run_manifest", "run_id cannot be empty")
	}
	if core.ID(r.SnapshotID).IsEmpty() {
		return core.NewValidationError("run_manifest", "snapshot_id cannot be empty")
	}
	if r.RegistryHash == "" {
		return core.NewValidationError("run_manifest", "registry_hash cannot be empty")
	}
	if r.CohortHash == "" {
		return core.NewValidationError("run_manifest", "cohort_hash cannot be empty")
	}
	if r.CodeVersion == "" {
		return core.NewValidationError("run_manifest", "code_version cannot be empty")
	}
	return nil
}
