package ports

import (
	"context"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"time"
)

// GreenfieldResearchPort - The "LLM Architect" interface
type GreenfieldResearchPort interface {
	GenerateResearchDirectives(ctx context.Context, req GreenfieldResearchRequest) (*GreenfieldResearchResponse, error)
}

// GreenfieldResearchRequest - The metadata handoff
type GreenfieldResearchRequest struct {
	RunID                core.RunID                 `json:"run_id"`
	SnapshotID           core.SnapshotID            `json:"snapshot_id"`
	FieldMetadata        []greenfield.FieldMetadata `json:"field_metadata"`
	StatisticalArtifacts []map[string]interface{}   `json:"statistical_artifacts,omitempty"` // Full statistical artifacts for context
	DiscoveryBriefs      interface{}                `json:"discovery_briefs,omitempty"`      // Discovery briefs for grounding
	Directives           int                        `json:"directives"`
}

// GreenfieldResearchResponse - The engineering blueprint
type GreenfieldResearchResponse struct {
	Directives         []greenfield.ResearchDirective      `json:"directives"`
	EngineeringBacklog []greenfield.EngineeringBacklogItem `json:"engineering_backlog"`
	RawLLMResponse     interface{}                         `json:"-"` // Raw LLM response (models.GreenfieldResearchOutput) for internal use
	RenderedPrompt     string                              `json:"-"` // Rendered prompt with industry context injection for debugging
	Audit              GreenfieldAudit                     `json:"audit"`
}

// GreenfieldAudit - Generation tracking
type GreenfieldAudit struct {
	GeneratorType  string  `json:"generator_type"`
	Model          string  `json:"model,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	ProcessingTime string  `json:"processing_time"`
}

// GeneratorPort - Interface for hypothesis generators
type GeneratorPort interface {
	GenerateHypotheses(ctx context.Context, req HypothesisRequest) (*HypothesisGeneration, error)
}

// HypothesisRequest - Request for hypothesis generation
type HypothesisRequest struct {
	Context       HypothesisContext `json:"context"`
	MaxHypotheses int               `json:"max_hypotheses"`
	RigorProfile  RigorProfile      `json:"rigor_profile"`
}

// HypothesisContext - Context information for hypothesis generation
type HypothesisContext struct {
	RelationshipArts []core.Artifact `json:"relationship_artifacts"`
}

// HypothesisGeneration - Response from hypothesis generation
type HypothesisGeneration struct {
	Candidates      []HypothesisCandidate `json:"candidates"`
	Dropped         []DroppedCandidate    `json:"dropped,omitempty"`
	DiscoveryBriefs interface{}           `json:"discovery_briefs,omitempty"`
	Audit           GenerationAudit       `json:"audit"`
}

// HypothesisCandidate - A generated hypothesis candidate
type HypothesisCandidate struct {
	ID                  string             `json:"id"`
	CauseKey            core.VariableKey   `json:"cause_key"`
	EffectKey           core.VariableKey   `json:"effect_key"`
	ConfounderKeys      []core.VariableKey `json:"confounder_keys,omitempty"`
	MechanismCategory   string             `json:"mechanism_category"`
	Rationale           string             `json:"rationale"`
	BusinessStory       string             `json:"business_story,omitempty"`
	ScientificStory     string             `json:"scientific_story,omitempty"`
	Confidence          float64            `json:"confidence"`
	SuggestedRigor      RigorProfile       `json:"suggested_rigor"`
	SupportingArtifacts []core.ArtifactID  `json:"supporting_artifacts,omitempty"`
	GeneratorType       string             `json:"generator_type,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
}

// GenerationAudit - Audit information for generation
type GenerationAudit struct {
	GeneratorType  string             `json:"generator_type"`
	Model          string             `json:"model,omitempty"`
	Temperature    float64            `json:"temperature,omitempty"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	PromptHash     core.Hash          `json:"prompt_hash,omitempty"`
	ResponseHash   core.Hash          `json:"response_hash,omitempty"`
	Dropped        []DroppedCandidate `json:"dropped,omitempty"`
	ProcessingTime time.Duration      `json:"processing_time"`
}

// DroppedCandidate - A candidate that was dropped during generation
type DroppedCandidate struct {
	CandidateIndex int    `json:"candidate_index"`
	Reason         string `json:"reason"`
	Message        string `json:"message"`
	CauseKey       string `json:"cause_key,omitempty"`
	EffectKey      string `json:"effect_key,omitempty"`
}

// RigorProfile - Validation rigor levels
type RigorProfile string

const (
	RigorBasic    RigorProfile = "basic"    // minimal validation
	RigorStandard RigorProfile = "standard" // standard validation
	RigorDecision RigorProfile = "decision" // full validation for decisions
)
