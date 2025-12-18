package ports

import (
	"context"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/hypothesis"
	"gohypo/domain/run"
	"gohypo/domain/stage"
)

// ReaderPort provides read-only access to system data for UI/API
// This ensures UI cannot write to ledger or modify system state
type ReaderPort interface {
	// Artifact queries (read-only)
	ListArtifacts(ctx context.Context, filters ArtifactFilters) ([]core.Artifact, error)
	GetArtifact(ctx context.Context, artifactID core.ArtifactID) (*core.Artifact, error)
	GetArtifactsByRun(ctx context.Context, runID core.RunID) ([]core.Artifact, error)
	GetArtifactsByKind(ctx context.Context, kind core.ArtifactKind, limit int) ([]core.Artifact, error)

	// Hypothesis queries (read-only)
	ListHypotheses(ctx context.Context, filters HypothesisFilters) ([]HypothesisSummary, error)
	GetHypothesis(ctx context.Context, hypothesisID core.HypothesisID) (*HypothesisDetail, error)

	// Run queries (read-only)
	ListRuns(ctx context.Context, filters RunFilters) ([]RunSummary, error)
	GetRun(ctx context.Context, runID core.RunID) (*RunDetail, error)

	// Contract queries (read-only)
	ListContracts(ctx context.Context) ([]ContractSummary, error)
	GetContract(ctx context.Context, varKey core.VariableKey) (*ContractDetail, error)

	// Variable health queries (read-only)
	GetVariableHealth(ctx context.Context, varKey core.VariableKey) (*VariableHealth, error)
}

// ArtifactFilters moved to ledger.go as part of port splitting

// HypothesisFilters for querying hypotheses
type HypothesisFilters struct {
	RunID     *core.RunID
	Status    *HypothesisStatus
	CauseKey  *core.VariableKey
	EffectKey *core.VariableKey
	Limit     int
	Offset    int
}

// RunFilters for querying runs
type RunFilters struct {
	Status *RunStatus
	Limit  int
	Offset int
}

// Summary and detail types for read-only responses
type HypothesisSummary struct {
	ID        core.HypothesisID
	CauseKey  core.VariableKey
	EffectKey core.VariableKey
	Status    HypothesisStatus
	CreatedAt core.Timestamp
}

type HypothesisDetail struct {
	ID                core.HypothesisID
	CauseKey          core.VariableKey
	EffectKey         core.VariableKey
	MechanismCategory string
	Rationale         string
	Status            HypothesisStatus
	ValidationResult  *ValidationSummary
	CreatedAt         core.Timestamp
}

type ValidationSummary struct {
	Verdict          hypothesis.ValidationVerdict
	RejectionReasons []string
	TriageResults    TriageSummary
	DecisionResults  *DecisionSummary
}

type TriageSummary struct {
	BaselineSignal   bool
	BeatsPhantom     bool
	ConfounderStress bool
}

type DecisionSummary struct {
	ConditionalIndependence bool
	NestedModelComparison   float64
	StabilityScore          float64
}

type RunSummary struct {
	ID        core.RunID
	Status    RunStatus
	StartedAt core.Timestamp
	Duration  int64 // milliseconds
}

type RunDetail struct {
	ID          core.RunID
	Fingerprint run.RunFingerprint
	Status      RunStatus
	StartedAt   core.Timestamp
	CompletedAt *core.Timestamp
	Duration    int64
	Stages      []StageResult
}

type ContractSummary struct {
	VarKey          core.VariableKey
	AsOfMode        dataset.AsOfMode
	StatisticalType dataset.StatisticalType
}

type ContractDetail struct {
	VarKey           core.VariableKey
	AsOfMode         dataset.AsOfMode
	StatisticalType  dataset.StatisticalType
	WindowDays       *int
	ScalarGuarantee  bool
	ImputationPolicy string
}

type VariableHealth struct {
	VarKey          core.VariableKey
	ResolutionCount int
	FailureCount    int
	LastResolved    *core.Timestamp
	HealthScore     float64 // 0-1, higher is better
}

// Enums for read models
type HypothesisStatus string

const (
	HypothesisPending   HypothesisStatus = "pending"
	HypothesisValidated HypothesisStatus = "validated"
	HypothesisRejected  HypothesisStatus = "rejected"
)

type RunStatus string

const (
	RunPending   RunStatus = "pending"
	RunRunning   RunStatus = "running"
	RunCompleted RunStatus = "completed"
	RunFailed    RunStatus = "failed"
)

type StageResult struct {
	Name     stage.StageName
	Success  bool
	Duration int64
	Error    string
}
