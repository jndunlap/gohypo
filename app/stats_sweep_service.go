package app

import (
	"context"
	"fmt"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/run"
	"gohypo/domain/stage"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// StatsSweepService handles Layer 0 statistical relationship discovery
type StatsSweepService struct {
	stageRunner *StageRunner
	ledgerPort  ports.LedgerPort
	rngPort     ports.RNGPort
}

// AuditableSweepRequest defines the inputs for a deterministic stats sweep
type AuditableSweepRequest struct {
	MatrixBundle *dataset.MatrixBundle
	RigorProfile stage.RigorProfile
	Seed         int64
	SweepID      core.ID // optional, will be generated if empty
}

// SweepResult contains the complete output of a stats sweep
type SweepResult struct {
	SweepID       core.ID         `json:"sweep_id"`
	Relationships []core.Artifact `json:"relationships"`
	Manifest      core.Artifact   `json:"manifest"`
	Fingerprint   core.Hash       `json:"fingerprint"`
	RuntimeMs     int64           `json:"runtime_ms"`
	Success       bool            `json:"success"`
}

// NewStatsSweepService creates a stats sweep service
func NewStatsSweepService(stageRunner *StageRunner, ledgerPort ports.LedgerPort, rngPort ports.RNGPort) *StatsSweepService {
	return &StatsSweepService{
		stageRunner: stageRunner,
		ledgerPort:  ledgerPort,
		rngPort:     rngPort,
	}
}

// RunAuditableSweep executes Layer 0 relationship discovery with complete audit trail
func (s *StatsSweepService) RunAuditableSweep(ctx context.Context, req AuditableSweepRequest) (*SweepResult, error) {
	startTime := time.Now()

	// Generate sweep ID if not provided
	sweepID := req.SweepID
	if sweepID == "" {
		sweepID = core.NewID()
	}

	// Create the sweep stage plan based on rigor profile
	plan := s.createSweepPlan(req.RigorProfile)

	// Create run manifest (AC1 requirement)
	runID := core.RunID(sweepID)
	manifest := run.NewRunManifestArtifact(
		runID,
		req.MatrixBundle.SnapshotID,
		core.SnapshotAt{}, // TODO: get from bundle
		req.MatrixBundle.Lag,
		req.MatrixBundle.CutoffAt,
		core.RegistryHash("test-registry-hash"), // TODO: compute from actual registry
		req.MatrixBundle.CohortHash,
		plan,
		req.Seed,
		"v0.1.0", // TODO: make configurable
	)

	// Execute the sweep with manifest
	pipelineRequest := stage.PipelineRequest{
		RunID:       sweepID.String(),
		InputBundle: req.MatrixBundle,
		Stages:      plan.Stages,
		Seed:        req.Seed,
	}
	pipelineResult, err := s.stageRunner.ExecutePipeline(ctx, pipelineRequest, manifest)

	if err != nil {
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Extract relationship artifacts (already persisted by StageRunner).
	var relationships []core.Artifact
	for _, result := range pipelineResult.Results {
		for _, artifact := range result.Artifacts {
			if artifact.Kind == core.ArtifactRelationship {
				relationships = append(relationships, artifact)
			}
		}
	}

	// Build a sweep manifest from executed results (and persist as a single audit artifact).
	sweepManifest := stats.NewSweepManifest(
		sweepID,
		req.MatrixBundle.SnapshotID,
		core.RegistryHash("test-registry-hash"), // TODO: compute from actual registry
		req.MatrixBundle.CohortHash,
		core.Hash("test-stage-plan-hash"), // TODO: compute from stage plan
		req.Seed,
	)
	sweepManifest.RuntimeMs = pipelineResult.Overall.TotalDuration

	artifactCounts := make(map[string]int)
	rejectionCounts := make(map[stats.WarningCode]int)
	var pairwiseTotal, pairwiseSuccess, pairwiseSkipped int

	for _, sr := range pipelineResult.Results {
		sweepManifest.TestsExecuted = append(sweepManifest.TestsExecuted, string(sr.StageName))
		for _, a := range sr.Artifacts {
			artifactCounts[string(a.Kind)]++
			switch a.Kind {
			case core.ArtifactRelationship:
				pairwiseSuccess++
			case core.ArtifactSkippedRelationship:
				pairwiseSkipped++
				// Extract reason code for structured counts.
				if p, ok := a.Payload.(stats.SkippedRelationshipArtifact); ok {
					rejectionCounts[p.ReasonCode]++
				} else if p, ok := a.Payload.(map[string]interface{}); ok {
					if rc, ok := p["reason_code"].(string); ok && rc != "" {
						rejectionCounts[stats.WarningCode(rc)]++
					}
				}
			}
		}
	}

	// Pairwise totals: number of pairwise results that were either tested or skipped.
	pairwiseTotal = pairwiseSuccess + pairwiseSkipped
	sweepManifest.TotalComparisons = pairwiseTotal
	sweepManifest.SuccessfulTests = pairwiseSuccess
	sweepManifest.SkippedTests = pairwiseSkipped
	sweepManifest.RejectionCounts = rejectionCounts
	sweepManifest.ArtifactCounts = artifactCounts

	manifestArtifact := core.Artifact{
		ID:        core.NewID(),
		Kind:      core.ArtifactSweepManifest,
		Payload:   *sweepManifest,
		CreatedAt: core.Now(),
	}

	// Compute overall fingerprint
	matrixFingerprint := req.MatrixBundle.Fingerprint // Assume MatrixBundle has fingerprint
	overallFingerprint := s.computeSweepFingerprint(sweepID, relationships, &manifestArtifact, matrixFingerprint)

	// Persist ONLY the sweep manifest here. StageRunner already persisted stage artifacts.
	if err := s.ledgerPort.StoreArtifact(ctx, sweepID.String(), manifestArtifact); err != nil {
		return nil, fmt.Errorf("failed to store sweep manifest artifact: %w", err)
	}

	runtimeMs := time.Since(startTime).Milliseconds()

	result := &SweepResult{
		SweepID:       sweepID,
		Relationships: relationships,
		Manifest:      manifestArtifact,
		Fingerprint:   overallFingerprint,
		RuntimeMs:     runtimeMs,
		Success:       pipelineResult.Success(),
	}

	return result, nil
}

// createSweepPlan creates the stage execution plan based on rigor profile
func (s *StatsSweepService) createSweepPlan(rigor stage.RigorProfile) *stage.StagePlan {
	var stages []stage.StageSpec

	// Add profile stage
	stages = append(stages, stage.StageSpec{
		Name: stage.StageProfile,
		Kind: stage.StageKindStats,
		Config: map[string]interface{}{
			"rigor": string(rigor),
		},
	})

	// Add pairwise stage
	stages = append(stages, stage.StageSpec{
		Name: stage.StagePairwise,
		Kind: stage.StageKindStats,
		Config: map[string]interface{}{
			"rigor": string(rigor),
		},
	})

	return stage.NewStagePlan(stages)
}

// computeSweepFingerprint creates a deterministic fingerprint for the sweep
func (s *StatsSweepService) computeSweepFingerprint(sweepID core.ID, relationships []core.Artifact, manifest *core.Artifact, matrixFingerprint core.Hash) core.Hash {
	// Create deterministic representation
	data := fmt.Sprintf("%s|%s|%d", sweepID, matrixFingerprint, len(relationships))

	// Include relationship fingerprints (simplified)
	for _, rel := range relationships {
		if payload, ok := rel.Payload.(map[string]interface{}); ok {
			data += fmt.Sprintf("|%s-%s-%.6f",
				payload["variable_x"],
				payload["variable_y"],
				payload["effect_size"])
		}
	}

	return core.NewHash([]byte(data))
}

// Legacy compatibility - to be removed
type StatsSweepRequest struct {
	MatrixBundle *dataset.MatrixBundle
}

type StatsSweepResponse struct {
	SweepID       core.ID
	Relationships []core.Artifact
	Manifest      core.Artifact
	Fingerprint   core.Hash
	RuntimeMs     int64
	Success       bool
}

func (s *StatsSweepService) RunStatsSweep(ctx context.Context, req StatsSweepRequest) (*StatsSweepResponse, error) {
	// Convert to new format
	auditableReq := AuditableSweepRequest{
		MatrixBundle: req.MatrixBundle,
		RigorProfile: stage.RigorDecision, // default to full rigor
		Seed:         42,                  // default seed
	}

	result, err := s.RunAuditableSweep(ctx, auditableReq)
	if err != nil {
		return nil, err
	}

	// Convert back to old format
	return &StatsSweepResponse{
		SweepID:       result.SweepID,
		Relationships: result.Relationships,
		Manifest:      result.Manifest,
		Fingerprint:   result.Fingerprint,
		RuntimeMs:     result.RuntimeMs,
		Success:       result.Success,
	}, nil
}
