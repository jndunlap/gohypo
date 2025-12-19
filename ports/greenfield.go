package ports

import (
	"context"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
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
	MaxDirectives        int                        `json:"max_directives"`
	// Optional thinking callback for real-time progress updates
	OnThinking func(string) `json:"-"` // Not serialized
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
