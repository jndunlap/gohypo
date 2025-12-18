package stage

import (
	"crypto/sha256"
	"encoding/json"
	"sort"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

// StageName represents a named stage in the pipeline
type StageName string

// StageKind categorizes stages by function
type StageKind string

const (
	StageKindStats   StageKind = "stats"   // statistical computation
	StageKindBattery StageKind = "battery" // validation tests
	StageKindAudit   StageKind = "audit"   // auditing/logging
)

// RigorProfile defines the level of statistical validation
type RigorProfile string

const (
	RigorBasic    RigorProfile = "basic"    // minimal validation
	RigorStandard RigorProfile = "standard" // standard validation
	RigorDecision RigorProfile = "decision" // full validation for decisions
)

// Predefined stage names
const (
	// Stats stages
	StageProfile     StageName = "profile"
	StagePairwise    StageName = "pairwise"
	StagePermutation StageName = "permutation"
	StageStability   StageName = "stability"
	StageRank        StageName = "rank"
	StageSweep       StageName = "sweep"

	// Battery stages
	StageAdmissibility    StageName = "admissibility"
	StageBaseline         StageName = "baseline"
	StagePhantomGate      StageName = "phantom_gate"
	StageConfounderStress StageName = "confounder_stress"
	StageDecision         StageName = "decision"

	// Audit stages
	StageAudit StageName = "audit"
)

// StageSpec defines a single stage in the pipeline
type StageSpec struct {
	Name   StageName              `json:"name"`
	Kind   StageKind              `json:"kind"`
	Config map[string]interface{} `json:"config"`
}

// StageResult represents the output of a stage execution
// CONTRACT: Every stage consumes (MatrixBundle, StageSpec) and produces (Artifacts, StageExecutionAudit)
type StageResult struct {
	StageName StageName           `json:"stage_name"`
	Success   bool                `json:"success"`
	Metrics   StageMetrics        `json:"metrics"`
	Artifacts []core.Artifact     `json:"artifacts,omitempty"` // typed artifacts produced
	Audit     StageExecutionAudit `json:"audit"`               // execution audit trail
	Error     string              `json:"error,omitempty"`
	Duration  int64               `json:"duration_ms"` // milliseconds
}

// StageExecutionAudit captures the execution context and results of a stage
type StageExecutionAudit struct {
	StageName        StageName       `json:"stage_name"`
	RunID            core.RunID      `json:"run_id"`
	SnapshotID       core.SnapshotID `json:"snapshot_id"`
	Seed             int64           `json:"seed"`
	ArtifactsWritten int             `json:"artifacts_written"`
	SkipsByReason    map[string]int  `json:"skips_by_reason"` // e.g., {"low_n": 5, "insufficient_variance": 2}
	Warnings         []string        `json:"warnings,omitempty"`
	ExecutedAt       core.Timestamp  `json:"executed_at"`
}

// StageMetrics contains canonical metrics for stage results
type StageMetrics struct {
	// Common metrics
	ProcessedCount int   `json:"processed_count"`
	SuccessCount   int   `json:"success_count"`
	FailureCount   int   `json:"failure_count"`
	DurationMs     int64 `json:"duration_ms"`

	// Statistical metrics (for stats stages)
	EffectSize     *float64 `json:"effect_size,omitempty"`
	PValue         *float64 `json:"p_value,omitempty"`
	StabilityScore *float64 `json:"stability_score,omitempty"`

	// Validation metrics (for battery stages)
	Passed     *bool    `json:"passed,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`

	// Custom metrics (stage-specific)
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// StagePlan represents an ordered list of stages with configuration
type StagePlan struct {
	Stages []StageSpec `json:"stages"`
}

// NewStagePlan creates a new stage plan
func NewStagePlan(stages []StageSpec) *StagePlan {
	return &StagePlan{Stages: stages}
}

// Hash computes a deterministic hash of the stage plan
func (p *StagePlan) Hash() core.StageListHash {
	// Sort stages by name for deterministic hashing
	sortedStages := make([]StageSpec, len(p.Stages))
	copy(sortedStages, p.Stages)
	sort.Slice(sortedStages, func(i, j int) bool {
		return sortedStages[i].Name < sortedStages[j].Name
	})

	data, _ := json.Marshal(sortedStages)
	sum := sha256.Sum256(data)
	return core.NewStageListHash(sum[:])
}

// Validate checks if the stage plan is valid
func (p *StagePlan) Validate() error {
	if len(p.Stages) == 0 {
		return core.NewValidationError("stage_plan", "must contain at least one stage")
	}

	seenNames := make(map[StageName]bool)
	for _, stage := range p.Stages {
		if stage.Name == "" {
			return core.NewValidationError("stage", "name cannot be empty")
		}
		if seenNames[stage.Name] {
			return core.NewValidationError("stage", "duplicate stage name: "+string(stage.Name))
		}
		seenNames[stage.Name] = true
	}

	return nil
}

// GetStagesByKind returns all stages of a specific kind
func (p *StagePlan) GetStagesByKind(kind StageKind) []StageSpec {
	var result []StageSpec
	for _, stage := range p.Stages {
		if stage.Kind == kind {
			result = append(result, stage)
		}
	}
	return result
}

// PipelineResult contains the results of executing a stage plan
type PipelineResult struct {
	Plan    *StagePlan      `json:"plan"`
	Results []StageResult   `json:"results"`
	Overall PipelineSummary `json:"overall"`
}

// PipelineSummary provides high-level pipeline statistics
type PipelineSummary struct {
	TotalStages    int   `json:"total_stages"`
	Successful     int   `json:"successful"`
	Failed         int   `json:"failed"`
	TotalDuration  int64 `json:"total_duration_ms"`
	ArtifactsCount int   `json:"artifacts_count"`
}

// NewPipelineResult creates a new pipeline result
func NewPipelineResult(plan *StagePlan) *PipelineResult {
	return &PipelineResult{
		Plan:    plan,
		Results: make([]StageResult, 0),
		Overall: PipelineSummary{},
	}
}

// AddResult adds a stage result and updates summary
func (r *PipelineResult) AddResult(result StageResult) {
	r.Results = append(r.Results, result)
	r.Overall.TotalStages++

	if result.Success {
		r.Overall.Successful++
	} else {
		r.Overall.Failed++
	}

	r.Overall.TotalDuration += result.Duration
	r.Overall.ArtifactsCount += len(result.Artifacts)
}

// Success returns true if all stages succeeded
func (r *PipelineResult) Success() bool {
	return r.Overall.Failed == 0
}

// PipelineRequest specifies a pipeline execution
type PipelineRequest struct {
	RunID       string                `json:"run_id"`
	InputBundle *dataset.MatrixBundle `json:"input_bundle"`
	Stages      []StageSpec           `json:"stages"`
	Seed        int64                 `json:"seed"`
}
