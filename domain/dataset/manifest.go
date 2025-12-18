package dataset

import (
	"encoding/json"
	"fmt"
	"sort"

	"gohypo/domain/core"
)

// SnapshotManifest captures the complete specification of a data snapshot
type SnapshotManifest struct {
	SnapshotID    core.SnapshotID `json:"snapshot_id"`
	SnapshotAt    core.SnapshotAt `json:"snapshot_at"`
	Lag           core.Lag        `json:"lag"`
	CutoffAt      core.CutoffAt   `json:"cutoff_at"`
	CohortSize    int             `json:"cohort_size"`
	EntityIDsHash core.Hash       `json:"entity_ids_hash"`
	ViewID        core.ID         `json:"view_id"`
	CohortHash    core.CohortHash `json:"cohort_hash"`
	CreatedAt     core.Timestamp  `json:"created_at"`
}

// NewSnapshotManifest creates a manifest for a snapshot resolution
func NewSnapshotManifest(snapshotID core.SnapshotID, snapshotAt core.SnapshotAt, lag core.Lag, entityIDs []core.ID, viewID core.ID, cohortHash core.CohortHash) *SnapshotManifest {
	// Compute entity IDs hash for audit trail
	entityHash := computeEntityIDsHash(entityIDs)

	return &SnapshotManifest{
		SnapshotID:    snapshotID,
		SnapshotAt:    snapshotAt,
		Lag:           lag,
		CutoffAt:      snapshotAt.ApplyLag(lag),
		CohortSize:    len(entityIDs),
		EntityIDsHash: entityHash,
		ViewID:        viewID,
		CohortHash:    cohortHash,
		CreatedAt:     core.Now(),
	}
}

// computeEntityIDsHash creates a deterministic hash of entity IDs
func computeEntityIDsHash(entityIDs []core.ID) core.Hash {
	// Sort for deterministic hashing
	ids := make([]string, len(entityIDs))
	for i, id := range entityIDs {
		ids[i] = string(id)
	}
	sort.Strings(ids)

	data, _ := json.Marshal(ids)
	return core.NewHash(data)
}

// ResolverAudit captures how each variable was resolved
type ResolverAudit struct {
	VariableKey       core.VariableKey `json:"variable_key"`
	MaxTimestampUsed  core.Timestamp   `json:"max_timestamp_used"`
	RowCount          int              `json:"row_count"`
	ImputationApplied string           `json:"imputation_applied"` // "none", "zero_fill", "mean_fill", etc.
	ScalarGuarantee   bool             `json:"scalar_guarantee"`
	AsOfMode          AsOfMode         `json:"as_of_mode"`
	WindowDays        *int             `json:"window_days,omitempty"`
	ResolutionErrors  []string         `json:"resolution_errors,omitempty"`
}

// ResolutionFingerprint provides complete determinism proof
type ResolutionFingerprint struct {
	ManifestHash    core.Hash         `json:"manifest_hash"`
	RegistryHash    core.RegistryHash `json:"registry_hash"`
	ResolverVersion string            `json:"resolver_version"` // semantic version
	Seed            int64             `json:"seed"`
	Fingerprint     core.Hash         `json:"fingerprint"` // hash of all above
}

// ComputeFingerprint creates the complete fingerprint for replayability
func (m *SnapshotManifest) ComputeFingerprint(registryHash core.RegistryHash, resolverVersion string, seed int64) *ResolutionFingerprint {
	// Hash the manifest
	manifestData, _ := json.Marshal(m)
	manifestHash := core.NewHash(manifestData)

	// Combine all deterministic inputs
	fingerprintData := fmt.Sprintf("%s|%s|%s|%d",
		manifestHash, registryHash, resolverVersion, seed)

	fingerprint := &ResolutionFingerprint{
		ManifestHash:    manifestHash,
		RegistryHash:    registryHash,
		ResolverVersion: resolverVersion,
		Seed:            seed,
		Fingerprint:     core.NewHash([]byte(fingerprintData)),
	}

	return fingerprint
}

// AuditableResolutionResult combines all outputs of the matrix resolver
type AuditableResolutionResult struct {
	Manifest     *SnapshotManifest      `json:"manifest"`
	MatrixBundle *MatrixBundle          `json:"matrix_bundle"`
	Audits       []ResolverAudit        `json:"audits"`
	Fingerprint  *ResolutionFingerprint `json:"fingerprint"`
}

// ValidateResult checks that the resolution result meets all requirements
func (r *AuditableResolutionResult) ValidateResult() error {
	// Check scalar guarantee for all variables
	for _, audit := range r.Audits {
		if !audit.ScalarGuarantee {
			return core.NewResolutionError(string(audit.VariableKey),
				fmt.Errorf("scalar guarantee failed"))
		}
	}

	// Check that matrix dimensions match manifest
	if r.MatrixBundle.RowCount() != r.Manifest.CohortSize {
		return core.NewValidationError("matrix_bundle",
			fmt.Sprintf("row count mismatch: matrix has %d rows, manifest expects %d",
				r.MatrixBundle.RowCount(), r.Manifest.CohortSize))
	}

	// Check that all variables have audits
	matrixVars := r.MatrixBundle.Matrix.VariableKeys
	if len(matrixVars) != len(r.Audits) {
		return core.NewValidationError("audits",
			fmt.Sprintf("audit count mismatch: %d audits for %d variables",
				len(r.Audits), len(matrixVars)))
	}

	// Verify variable key consistency
	for i, audit := range r.Audits {
		if audit.VariableKey != matrixVars[i] {
			return core.NewValidationError("variable_order",
				fmt.Sprintf("variable order mismatch at position %d: audit has %s, matrix has %s",
					i, audit.VariableKey, matrixVars[i]))
		}
	}

	return nil
}
