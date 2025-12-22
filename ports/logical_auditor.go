package ports

import (
	"context"
	"gohypo/models"
)

// LogicalAuditorRequest represents the input for referee selection analysis
type LogicalAuditorRequest struct {
	BusinessHypothesis  string `json:"business_hypothesis"`
	ScienceHypothesis   string `json:"science_hypothesis"`
	NullCase           string `json:"null_case"`
	CauseKey           string `json:"cause_key"`
	EffectKey          string `json:"effect_key"`
	StatisticalEvidence string `json:"statistical_evidence"` // JSON string of evidence data
	VariableContext    string `json:"variable_context"`     // JSON string of field metadata
	RigorLevel         string `json:"rigor_level"`          // "exploratory", "decision-critical"
	ComputationalBudget string `json:"computational_budget"` // "low", "medium", "high"
}

// LogicalAuditorPort defines the interface for logical auditor operations
type LogicalAuditorPort interface {
	GenerateRefereeSelection(ctx context.Context, req LogicalAuditorRequest) (*models.LogicalAuditorOutput, error)
}
