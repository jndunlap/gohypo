package ports

import (
	"context"

	"gohypo/domain/stats"
)

// EValueCalibrator handles E-value operations and conversions
type EValueCalibrator interface {
	// ConvertPValueToEValue converts p-values to E-values with calibration
	ConvertPValueToEValue(pValue float64, testType string, isTwoTailed bool) stats.EValue

	// CombineEvidence combines multiple E-values with correlation handling
	CombineEvidence(eValues []stats.EValue, testCount int) stats.EvidenceCombination

	// GetDynamicThreshold returns appropriate threshold based on test count and confidence
	GetDynamicThreshold(testCount int, confidence float64) float64

	// CheckEarlyStopEligibility determines if validation can stop early
	CheckEarlyStopEligibility(normalizedE float64, currentTestCount int) bool
}

// TestSelector handles dynamic test selection
type TestSelector interface {
	// SelectTests chooses optimal validation tests for a hypothesis
	SelectTests(profile stats.HypothesisProfile) ([]stats.SelectedTest, stats.SelectionRationale)

	// GetTestCountRange returns appropriate test count bounds for risk level
	GetTestCountRange(riskLevel stats.HypothesisRiskLevel) (minTests, maxTests int)
}

// WorkspaceContextAssembler assembles workspace state for iterative generation
type WorkspaceContextAssembler interface {
	// AssembleFullContext builds complete workspace context
	AssembleFullContext(ctx context.Context, workspaceID string) (*WorkspaceContext, error)

	// AssembleHypothesisPrompt builds prompt from workspace context
	AssembleHypothesisPrompt(ctx context.Context, workspaceID string, directive ResearchDirective) (string, error)
}

// WorkspaceContext represents the complete state of research workspace
type WorkspaceContext struct {
	ValidatedHypotheses []ValidatedHypothesisSummary
	RejectedHypotheses  []RejectedHypothesisSummary
	DatasetSummaries    []DatasetSummary
	ForensicContext     ForensicSummary
	ResearchTrajectory  ResearchTrajectory
	TemporalCoverage    TemporalCoverage
}

// Supporting types for workspace context
type ValidatedHypothesisSummary struct {
	ID          string
	CauseKey    string
	EffectKey   string
	EValue      float64
	Confidence  float64
	Rationale   string
	TestCount   int
	ValidatedAt string
}

type RejectedHypothesisSummary struct {
	ID            string
	CauseKey      string
	EffectKey     string
	FailureReason string
	WeakestEValue float64
	RejectedAt    string
	CommonFailure string
}

type DatasetSummary struct {
	Name          string
	RecordCount   int
	FieldCount    int
	TemporalRange string
	KeyVariables  []string
}

type ForensicSummary struct {
	Domain      string
	DatasetName string
	KeyInsights []string
	RiskFactors []string
}

type ResearchTrajectory struct {
	TotalHypotheses int
	ValidationRate  float64
	CommonThemes    []string
	EvolvingFocus   []string
}

type TemporalCoverage struct {
	DateRange   string
	DataDensity string
	MissingGaps []string
}

// ResearchDirective represents the current research goal
type ResearchDirective struct {
	Description string
	Priority    string
	FocusArea   string
	AvoidAreas  []string
}
