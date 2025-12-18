package run

import (
	"testing"

	"gohypo/domain/core"
	"gohypo/domain/stage"
)

func TestRunFingerprint_Deterministic(t *testing.T) {
	// A6: Golden test - same inputs produce identical fingerprints
	snapshotID := core.SnapshotID("test-snapshot")
	registryHash := core.RegistryHash("test-registry")
	cohortHash := core.CohortHash("test-cohort")
	stagePlanHash := core.Hash("test-stage-plan")
	seed := int64(42)
	codeVersion := "1.0.0"

	// Generate fingerprint twice with identical inputs
	fp1 := NewRunFingerprint(snapshotID, registryHash, cohortHash, stagePlanHash, seed, codeVersion)
	fp2 := NewRunFingerprint(snapshotID, registryHash, cohortHash, stagePlanHash, seed, codeVersion)

	// Should be identical
	if fp1.Fingerprint != fp2.Fingerprint {
		t.Errorf("Fingerprints not identical: %s vs %s", fp1.Fingerprint, fp2.Fingerprint)
	}

	// Should contain all determinism parameters
	if fp1.SnapshotID != snapshotID {
		t.Errorf("SnapshotID mismatch: %s vs %s", fp1.SnapshotID, snapshotID)
	}
	if fp1.RegistryHash != registryHash {
		t.Errorf("RegistryHash mismatch: %s vs %s", fp1.RegistryHash, registryHash)
	}
	if fp1.CohortHash != cohortHash {
		t.Errorf("CohortHash mismatch: %s vs %s", fp1.CohortHash, cohortHash)
	}
	if fp1.Seed != seed {
		t.Errorf("Seed mismatch: %d vs %d", fp1.Seed, seed)
	}
	if fp1.CodeVersion != codeVersion {
		t.Errorf("CodeVersion mismatch: %s vs %s", fp1.CodeVersion, codeVersion)
	}
}

func TestRunFingerprint_Unique(t *testing.T) {
	// Different inputs should produce different fingerprints
	base := NewRunFingerprint(
		core.SnapshotID("test-snapshot"),
		core.RegistryHash("test-registry"),
		core.CohortHash("test-cohort"),
		core.Hash("test-stage-plan"),
		42,
		"1.0.0",
	)

	// Change each parameter and verify fingerprint changes
	testCases := []struct {
		name string
		fp   RunFingerprint
	}{
		{"different snapshot", NewRunFingerprint(
			core.SnapshotID("different-snapshot"), // changed
			core.RegistryHash("test-registry"),
			core.CohortHash("test-cohort"),
			core.Hash("test-stage-plan"),
			42,
			"1.0.0",
		)},
		{"different registry", NewRunFingerprint(
			core.SnapshotID("test-snapshot"),
			core.RegistryHash("different-registry"), // changed
			core.CohortHash("test-cohort"),
			core.Hash("test-stage-plan"),
			42,
			"1.0.0",
		)},
		{"different seed", NewRunFingerprint(
			core.SnapshotID("test-snapshot"),
			core.RegistryHash("test-registry"),
			core.CohortHash("test-cohort"),
			core.Hash("test-stage-plan"),
			43, // changed
			"1.0.0",
		)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.fp.Fingerprint == base.Fingerprint {
				t.Errorf("Fingerprint should be different for %s", tc.name)
			}
		})
	}
}

func TestRunManifestArtifact_Complete(t *testing.T) {
	// A1: Verify determinism tuple is complete
	runID := core.RunID("test-run")
	snapshotID := core.SnapshotID("test-snapshot")
	snapshotAt := core.NewSnapshotAt(core.Now().Time())
	lag := core.NewLag(24 * 60 * 60 * 1000) // 24 hours in milliseconds
	cutoffAt := core.NewCutoffAt(core.Now().Time())
	registryHash := core.RegistryHash("test-registry")
	cohortHash := core.CohortHash("test-cohort")
	seed := int64(42)
	codeVersion := "1.0.0"

	// Create mock stage plan
	stagePlan := &stage.StagePlan{
		Stages: []stage.StageSpec{
			{Name: "pairwise", Kind: stage.StageKindStats},
		},
	}

	manifest := NewRunManifestArtifact(
		runID, snapshotID, snapshotAt, lag, cutoffAt,
		registryHash, cohortHash, stagePlan, seed, codeVersion,
	)

	// Verify all determinism fields are present
	if manifest.RunID != runID {
		t.Errorf("RunID not set correctly")
	}
	if manifest.SnapshotID != snapshotID {
		t.Errorf("SnapshotID not set correctly")
	}
	if manifest.RegistryHash != registryHash {
		t.Errorf("RegistryHash not set correctly")
	}
	if manifest.CohortHash != cohortHash {
		t.Errorf("CohortHash not set correctly")
	}
	if manifest.Seed != seed {
		t.Errorf("Seed not set correctly")
	}
	if manifest.CodeVersion != codeVersion {
		t.Errorf("CodeVersion not set correctly")
	}

	// Verify fingerprint is computed
	if manifest.Fingerprint.Fingerprint == "" {
		t.Errorf("Fingerprint not computed")
	}

	// Verify validation passes
	if err := manifest.Validate(); err != nil {
		t.Errorf("Manifest validation failed: %v", err)
	}
}
