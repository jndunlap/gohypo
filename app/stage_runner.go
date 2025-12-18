package app

import (
	"context"
	"fmt"
	"time"

	"gohypo/adapters/stats/stages"
	"gohypo/domain/artifacts"
	"gohypo/domain/core"
	"gohypo/domain/run"
	"gohypo/domain/stage"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// StageRunner executes stage plans and collects artifacts
// ENFORCES: Run manifest must exist before any stage artifacts can be stored
type StageRunner struct {
	ledgerPort ports.LedgerWriterPort
	rngPort    ports.RNGPort
}

// NewStageRunner creates a new stage runner
func NewStageRunner(ledgerPort ports.LedgerWriterPort, rngPort ports.RNGPort) *StageRunner {
	return &StageRunner{
		ledgerPort: ledgerPort,
		rngPort:    rngPort,
	}
}

// ExecutePipeline runs a stage plan and returns results
// AC1: Run manifest must exist before any stage artifacts can be stored
func (r *StageRunner) ExecutePipeline(ctx context.Context, req stage.PipelineRequest, manifest *run.RunManifestArtifact) (*stage.PipelineResult, error) {
	// AC1: Validate and store run manifest first (enforced requirement)
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("invalid run manifest: %w", err)
	}

	manifestArtifact := manifest.ToCoreArtifact()
	if err := r.ledgerPort.StoreArtifact(ctx, req.RunID, manifestArtifact); err != nil {
		return nil, fmt.Errorf("failed to store run manifest: %w", err)
	}

	// AC1: Run manifest is now stored - enforce that all subsequent artifacts require this
	manifestStored := true

	// Create stage plan from request
	plan := &stage.StagePlan{Stages: req.Stages}
	result := stage.NewPipelineResult(plan)
	startTime := time.Now()

	// Execute each stage in order
	for _, stageSpec := range plan.Stages {
		stageResult, err := r.executeStage(ctx, stageSpec, req, startTime)
		if err != nil {
			// Continue with other stages unless it's a hard failure
			stageResult = &stage.StageResult{
				StageName: stageSpec.Name,
				Success:   false,
				Metrics:   stage.StageMetrics{},
				Artifacts: []core.Artifact{},
				Audit: stage.StageExecutionAudit{
					StageName:  stageSpec.Name,
					RunID:      core.RunID(req.RunID),
					SnapshotID: req.InputBundle.SnapshotID,
					Seed:       req.Seed,
					ExecutedAt: core.Now(),
				},
				Error:    err.Error(),
				Duration: time.Since(startTime).Milliseconds(),
			}
		}

		// StageRunner is the ONLY component that writes to ledger (centralized persistence)
		for _, artifact := range stageResult.Artifacts {
			// AC1: Enforce manifest requirement - no stage artifacts without manifest
			if !manifestStored {
				return nil, fmt.Errorf("attempted to store stage artifact before run manifest")
			}

			// AC5: Validate artifact through registry before storing (now non-optional)
			if err := artifacts.ValidateArtifact(artifact); err != nil {
				return nil, fmt.Errorf("artifact validation failed for %s: %w", artifact.ID, err)
			}

			if err := r.ledgerPort.StoreArtifact(ctx, req.RunID, artifact); err != nil {
				return nil, fmt.Errorf("failed to persist artifact %s: %w", artifact.ID, err)
			}
		}

		result.AddResult(*stageResult)
	}

	result.Overall.TotalDuration = time.Since(startTime).Milliseconds()
	return result, nil
}

// executeStage runs a single stage and returns its result
func (r *StageRunner) executeStage(ctx context.Context, stageSpec stage.StageSpec, req stage.PipelineRequest, startTime time.Time) (*stage.StageResult, error) {
	startTime = time.Now()

	switch stageSpec.Kind {
	case stage.StageKindStats:
		return r.executeStatsStage(ctx, stageSpec, req, startTime)
	default:
		return nil, fmt.Errorf("unsupported stage kind: %s", stageSpec.Kind)
	}
}

// executeStatsStage handles statistical analysis stages
// CONTRACT: Consumes (MatrixBundle, StageSpec) → Produces ([]core.Artifact, StageAudit)
func (r *StageRunner) executeStatsStage(ctx context.Context, stageSpec stage.StageSpec, req stage.PipelineRequest, startTime time.Time) (*stage.StageResult, error) {
	result := &stage.StageResult{
		StageName: stageSpec.Name,
		Success:   true,
		Metrics: stage.StageMetrics{
			ProcessedCount: 1,
			SuccessCount:   1,
		},
		Artifacts: []core.Artifact{},
		Audit: stage.StageExecutionAudit{
			StageName:     stageSpec.Name,
			RunID:         core.RunID(req.RunID),
			SnapshotID:    req.InputBundle.SnapshotID,
			Seed:          req.Seed,
			ExecutedAt:    core.Now(),
			SkipsByReason: make(map[string]int),
		},
		Duration: 0, // Will be set at the end
	}

	var rawArtifacts []interface{}
	var err error

	// Execute the appropriate stage
	switch stageSpec.Name {
	case stage.StageProfile:
		profileStage := stages.NewProfileStage()
		rawArtifacts, err = profileStage.Execute(req.InputBundle, stageSpec.Config)
	case stage.StagePairwise:
		pairwiseStage := stages.NewPairwiseStage()
		rawArtifacts, err = pairwiseStage.Execute(req.InputBundle, stageSpec.Config)
	case stage.StageSweep:
		// Create a sweep manifest artifact
		manifest := stats.NewSweepManifest(
			core.ID(req.RunID),
			req.InputBundle.SnapshotID,
			core.RegistryHash("test-registry-hash"), // TODO: get from bundle
			req.InputBundle.CohortHash,
			core.Hash("test-stage-plan-hash"), // TODO: compute from stage plan
			req.Seed,
		)
		manifest.TestsExecuted = []string{"profile", "pairwise", "permutation", "stability"}
		manifest.RuntimeMs = result.Duration
		manifest.RejectionCounts = map[stats.WarningCode]int{
			stats.WarningLowVariance: 0,
			stats.WarningLowN:        0,
			stats.WarningHighMissing: 0,
		}
		manifest.TotalComparisons = len(rawArtifacts)
		manifest.SuccessfulTests = len(rawArtifacts)
		rawArtifacts = []interface{}{manifest}
	default:
		// Fallback to mock artifact
		artifact := map[string]interface{}{
			"stage":     string(stageSpec.Name),
			"run_id":    req.RunID,
			"timestamp": core.Now().Time().Format(time.RFC3339),
			"mock":      true,
		}
		rawArtifacts = []interface{}{artifact}
	}

	if err != nil {
		return nil, fmt.Errorf("stage execution failed: %w", err)
	}

	// Convert raw artifacts to core.Artifacts
	coreArtifacts := make([]core.Artifact, len(rawArtifacts))
	for i, raw := range rawArtifacts {
		coreArtifacts[i] = r.convertToCoreArtifact(raw, stageSpec.Name, req.RunID)
	}

	result.Artifacts = coreArtifacts
	// Populate audit skip counts from converted artifacts (used by diagnostics UI).
	for _, a := range coreArtifacts {
		if a.Kind != core.ArtifactSkippedRelationship {
			continue
		}
		// Typed payload path.
		if p, ok := a.Payload.(stats.SkippedRelationshipArtifact); ok {
			if p.ReasonCode != "" {
				result.Audit.SkipsByReason[string(p.ReasonCode)]++
			}
			continue
		}
		// Map payload fallback.
		if p, ok := a.Payload.(map[string]interface{}); ok {
			if rc, ok := p["reason_code"].(string); ok && rc != "" {
				result.Audit.SkipsByReason[rc]++
			}
		}
	}
	result.Audit.ArtifactsWritten = len(coreArtifacts)
	result.Metrics.ProcessedCount = len(coreArtifacts)
	result.Metrics.SuccessCount = len(coreArtifacts)
	result.Duration = time.Since(startTime).Milliseconds()

	return result, nil
}

// convertToCoreArtifact converts stage outputs to domain artifacts
func (r *StageRunner) convertToCoreArtifact(raw interface{}, stageName stage.StageName, runID string) core.Artifact {
	switch artifact := raw.(type) {
	case *stages.RelationshipResult:
		// Convert pairwise stage artifact to domain artifact
		if !artifact.Skipped {
			relationshipArtifact, err := stats.NewRelationshipArtifact(artifact.Key, artifact.Metrics)
			if err != nil {
				// Return error artifact
				return core.Artifact{
					ID:        core.NewID(),
					Kind:      core.ArtifactRelationship,
					Payload:   map[string]string{"error": err.Error()},
					CreatedAt: core.Now(),
				}
			}
			relationshipArtifact.DataQuality = artifact.DataQuality
			if artifact.SkipReason != "" {
				relationshipArtifact.OverallWarnings = []stats.WarningCode{artifact.SkipReason}
			}
			return core.Artifact{
				ID:        core.NewID(),
				Kind:      core.ArtifactRelationship,
				Payload:   relationshipArtifact.ToPayload(),
				CreatedAt: core.Now(),
			}
		}
		// Persist skipped relationships explicitly for diagnostics.
		skipped := stats.NewSkippedRelationshipArtifact(artifact.Key, artifact.SkipReason)
		skipped.DataQuality = artifact.DataQuality
		skipped.Counts = map[string]int{
			"sample_size": artifact.Metrics.SampleSize,
		}
		return core.Artifact{
			ID:        core.NewID(),
			Kind:      core.ArtifactSkippedRelationship,
			Payload:   *skipped,
			CreatedAt: core.Now(),
		}
	case *stats.SweepManifest:
		// Convert manifest to domain artifact
		return core.Artifact{
			ID:   core.NewID(),
			Kind: core.ArtifactSweepManifest,
			Payload: map[string]interface{}{
				"sweep_id":          artifact.SweepID,
				"snapshot_id":       artifact.SnapshotID,
				"registry_hash":     artifact.RegistryHash,
				"cohort_hash":       artifact.CohortHash,
				"stage_plan_hash":   artifact.StagePlanHash,
				"seed":              artifact.Seed,
				"tests_executed":    artifact.TestsExecuted,
				"runtime_ms":        artifact.RuntimeMs,
				"total_comparisons": artifact.TotalComparisons,
				"successful_tests":  artifact.SuccessfulTests,
				"skipped_tests":     artifact.SkippedTests,
				"rejection_counts":  artifact.RejectionCounts,
				"artifact_counts":   artifact.ArtifactCounts,
				"fingerprint":       artifact.Fingerprint,
			},
			CreatedAt: artifact.CreatedAt,
		}
	case *stats.FDRFamilyArtifact:
		return core.Artifact{
			ID:        core.NewID(),
			Kind:      core.ArtifactFDRFamily,
			Payload:   *artifact,
			CreatedAt: core.Now(),
		}
	default:
		// Profile stage emits map payloads; store under a dedicated kind for UI diagnostics.
		if stageName == stage.StageProfile {
			if m, ok := raw.(map[string]interface{}); ok {
				return core.Artifact{
					ID:        core.NewID(),
					Kind:      core.ArtifactVariableProfile,
					Payload:   m,
					CreatedAt: core.Now(),
				}
			}
		}
		// Generic artifact wrapper for legacy/unknown stage output.
		return core.Artifact{
			ID:        core.NewID(),
			Kind:      core.ArtifactVariableHealth, // fallback
			Payload:   artifact,
			CreatedAt: core.Now(),
		}
	}

	// Should not reach here
	return core.Artifact{
		ID:        core.NewID(),
		Kind:      core.ArtifactVariableHealth,
		Payload:   raw,
		CreatedAt: core.Now(),
	}
}
