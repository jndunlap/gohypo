package run

import (
	"crypto/sha256"
	"fmt"

	"gohypo/domain/core"
)

// Run represents an execution of the analysis pipeline
type Run struct {
	// TODO: implement
}

// RunFingerprint ensures deterministic replay
type RunFingerprint struct {
	SnapshotID    core.SnapshotID   `json:"snapshot_id"`
	RegistryHash  core.RegistryHash `json:"registry_hash"`
	CohortHash    core.CohortHash   `json:"cohort_hash"`
	StagePlanHash core.Hash         `json:"stage_plan_hash"`
	Seed          int64             `json:"seed"`
	CodeVersion   string            `json:"code_version"`
	Fingerprint   core.Hash         `json:"fingerprint"` // Hash of all above
}

// NewRunFingerprint creates a fingerprint from determinism parameters
func NewRunFingerprint(snapshotID core.SnapshotID, registryHash core.RegistryHash,
	cohortHash core.CohortHash, stagePlanHash core.Hash, seed int64, codeVersion string) RunFingerprint {

	fingerprint := computeRunFingerprint(snapshotID, registryHash, cohortHash, stagePlanHash, seed, codeVersion)

	return RunFingerprint{
		SnapshotID:    snapshotID,
		RegistryHash:  registryHash,
		CohortHash:    cohortHash,
		StagePlanHash: stagePlanHash,
		Seed:          seed,
		CodeVersion:   codeVersion,
		Fingerprint:   fingerprint,
	}
}

// computeRunFingerprint generates deterministic hash from all determinism parameters
func computeRunFingerprint(snapshotID core.SnapshotID, registryHash core.RegistryHash,
	cohortHash core.CohortHash, stagePlanHash core.Hash, seed int64, codeVersion string) core.Hash {

	// Create deterministic string representation
	data := fmt.Sprintf("snapshot:%s|registry:%s|cohort:%s|stage_plan:%s|seed:%d|code:%s",
		snapshotID, registryHash, cohortHash, stagePlanHash, seed, codeVersion)

	// Use SHA256 for deterministic hashing
	hash := sha256.Sum256([]byte(data))
	return core.Hash(fmt.Sprintf("%x", hash))
}

// PipelineResult contains the output of a pipeline execution
type PipelineResult struct {
	// TODO: implement
}
