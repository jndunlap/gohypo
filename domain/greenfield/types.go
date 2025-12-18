package greenfield

import (
	"gohypo/domain/core"
)

// ResearchDirectiveID represents a unique identifier for a research directive
type ResearchDirectiveID core.ID

func (id ResearchDirectiveID) String() string { return core.ID(id).String() }

// ResearchDirective - The "Engineering Blueprint" from the LLM
type ResearchDirective struct {
	ID                 ResearchDirectiveID `json:"id"`
	Claim              string              `json:"claim"`
	LogicType          string              `json:"logic_type"`
	ValidationStrategy ValidationStrategy  `json:"validation_strategy"`
	RefereeGates       RefereeGates        `json:"referee_gates"`
	CreatedAt          core.Timestamp      `json:"created_at"`
}

// ValidationStrategy - The "Statistical Instruments" required
type ValidationStrategy struct {
	Detector string `json:"detector"`
	Scanner  string `json:"scanner"`
	Proxy    string `json:"proxy"`
}

// RefereeGates - The "Pass/Fail Thresholds"
type RefereeGates struct {
	PValueThreshold float64 `json:"p_value_threshold"`
	StabilityScore  float64 `json:"stability_score"`
	PermutationRuns int     `json:"permutation_runs"`
}

// EngineeringBacklogItem - The "Missing Instruments" that need to be built
type EngineeringBacklogItem struct {
	ID              core.ID             `json:"id"`
	DirectiveID     ResearchDirectiveID `json:"directive_id"`
	CapabilityType  string              `json:"capability_type"` // "detector", "scanner", "proxy"
	CapabilityName  string              `json:"capability_name"`
	Priority        int                 `json:"priority"`
	Status          BacklogStatus       `json:"status"`
	Description     string              `json:"description"`
	EstimatedEffort string              `json:"estimated_effort"`
	CreatedAt       core.Timestamp      `json:"created_at"`
}

// BacklogStatus - Implementation progress
type BacklogStatus string

const (
	BacklogStatusPending    BacklogStatus = "pending"
	BacklogStatusInProgress BacklogStatus = "in_progress"
	BacklogStatusCompleted  BacklogStatus = "completed"
	BacklogStatusBlocked    BacklogStatus = "blocked"
)

// FieldMetadata - The "Research Brief" sent to the LLM
type FieldMetadata struct {
	Name          string   `json:"name"`
	SemanticType  string   `json:"semantic_type"`
	DataType      string   `json:"data_type"`
	Description   string   `json:"description,omitempty"`
	ExampleValues []string `json:"example_values,omitempty"`
}
