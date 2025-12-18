package ports

import (
	"context"

	"gohypo/domain/core"
)

// RigorProfile defines the level of statistical validation
type RigorProfile string

const (
	RigorBasic    RigorProfile = "basic"    // minimal validation
	RigorStandard RigorProfile = "standard" // standard validation
	RigorDecision RigorProfile = "decision" // full validation for decisions
)

// GeneratorPort creates hypothesis candidates
type GeneratorPort interface {
	GenerateHypotheses(ctx context.Context, req HypothesisRequest) (*HypothesisGeneration, error)
}

// HypothesisContext provides context for hypothesis generation
type HypothesisContext struct {
	RelationshipArts []core.Artifact `json:"relationship_arts"`
}

// HypothesisRequest specifies hypothesis generation parameters
type HypothesisRequest struct {
	Context       HypothesisContext `json:"context"`
	MaxHypotheses int               `json:"max_hypotheses"`
	RigorProfile  RigorProfile      `json:"rigor_profile"`
}

// DroppedCandidate records why a candidate was rejected (audit trail).
type DroppedCandidate struct {
	CandidateIndex int    `json:"candidate_index"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
}

// GenerationAudit is metadata about a generation call (prompt/response hashes, model, drops).
type GenerationAudit struct {
	GeneratorType string             `json:"generator_type"` // "llm" | "heuristic"
	Model         string             `json:"model,omitempty"`
	Temperature   float64            `json:"temperature,omitempty"`
	MaxTokens     int                `json:"max_tokens,omitempty"`
	PromptHash    core.Hash          `json:"prompt_hash,omitempty"`
	ResponseHash  core.Hash          `json:"response_hash,omitempty"`
	Dropped       []DroppedCandidate `json:"dropped,omitempty"`
}

// HypothesisGeneration is the full output of hypothesis generation.
// Candidates are what the user can review; Audit is what the system persists for replay/debugging.
type HypothesisGeneration struct {
	Candidates []HypothesisCandidate `json:"candidates"`
	Audit      GenerationAudit       `json:"audit"`
}

// HypothesisCandidate represents a generated hypothesis
type HypothesisCandidate struct {
	CauseKey            core.VariableKey   `json:"cause_key"`
	EffectKey           core.VariableKey   `json:"effect_key"`
	ConfounderKeys      []core.VariableKey `json:"confounder_keys"`
	MechanismCategory   string             `json:"mechanism_category"`
	Rationale           string             `json:"rationale"`
	SuggestedRigor      RigorProfile       `json:"suggested_rigor"`
	SupportingArtifacts []core.ArtifactID  `json:"supporting_artifacts,omitempty"`
	GeneratorType       string             `json:"generator_type,omitempty"` // "llm" | "heuristic"
}
