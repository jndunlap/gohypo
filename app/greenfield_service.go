package app

import (
	"context"
	"fmt"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"gohypo/ports"
)

type GreenfieldService struct {
	greenfieldPort ports.GreenfieldResearchPort
	ledgerPort     ports.LedgerPort
}

func NewGreenfieldService(greenfieldPort ports.GreenfieldResearchPort, ledgerPort ports.LedgerPort) *GreenfieldService {
	return &GreenfieldService{
		greenfieldPort: greenfieldPort,
		ledgerPort:     ledgerPort,
	}
}

// GetGreenfieldPort returns the greenfield research port (for worker access)
func (s *GreenfieldService) GetGreenfieldPort() ports.GreenfieldResearchPort {
	return s.greenfieldPort
}

// ExecuteGreenfieldFlow - The complete "Discovery Pre-Flight" protocol
func (s *GreenfieldService) ExecuteGreenfieldFlow(
	ctx context.Context,
	runID core.RunID,
	snapshotID core.SnapshotID,
	fieldMetadata []greenfield.FieldMetadata,
) (*GreenfieldFlowResult, error) {

	// 1. Call LLM for research directives (Discovery Pre-Flight)
	req := ports.GreenfieldResearchRequest{
		RunID:         runID,
		SnapshotID:    snapshotID,
		FieldMetadata: fieldMetadata,
		MaxDirectives: 3, // As specified in your prompt
	}

	response, err := s.greenfieldPort.GenerateResearchDirectives(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate research directives: %w", err)
	}

	// 2. Store research directives as artifacts
	directiveArtifacts := s.convertDirectivesToArtifacts(response.Directives, runID)
	for _, artifact := range directiveArtifacts {
		if err := s.ledgerPort.StoreArtifact(ctx, string(runID), artifact); err != nil {
			return nil, fmt.Errorf("failed to store directive artifact: %w", err)
		}
	}

	// 3. Store engineering backlog as artifacts (the "Missing Instruments")
	backlogArtifacts := s.convertBacklogToArtifacts(response.EngineeringBacklog, runID)
	for _, artifact := range backlogArtifacts {
		if err := s.ledgerPort.StoreArtifact(ctx, string(runID), artifact); err != nil {
			return nil, fmt.Errorf("failed to store backlog artifact: %w", err)
		}
	}

	return &GreenfieldFlowResult{
		DirectivesCreated:    len(directiveArtifacts),
		BacklogItemsCreated:  len(backlogArtifacts),
		CapabilitiesRequired: len(response.EngineeringBacklog),
	}, nil
}

func (s *GreenfieldService) convertDirectivesToArtifacts(directives []greenfield.ResearchDirective, runID core.RunID) []core.Artifact {
	artifacts := make([]core.Artifact, len(directives))

	for i, directive := range directives {
		artifacts[i] = core.Artifact{
			ID:   core.ID(directive.ID),
			Kind: core.ArtifactResearchDirective,
			Payload: map[string]interface{}{
				"run_id":              runID,
				"id":                  directive.ID,
				"claim":               directive.Claim,
				"logic_type":          directive.LogicType,
				"validation_strategy": directive.ValidationStrategy,
				"referee_gates":       directive.RefereeGates,
				"created_at":          directive.CreatedAt,
			},
			CreatedAt: directive.CreatedAt,
		}
	}

	return artifacts
}

func (s *GreenfieldService) convertBacklogToArtifacts(backlog []greenfield.EngineeringBacklogItem, runID core.RunID) []core.Artifact {
	artifacts := make([]core.Artifact, len(backlog))

	for i, item := range backlog {
		artifacts[i] = core.Artifact{
			ID:   item.ID,
			Kind: core.ArtifactEngineeringBacklog,
			Payload: map[string]interface{}{
				"run_id":           runID,
				"directive_id":     item.DirectiveID,
				"capability_type":  item.CapabilityType,
				"capability_name":  item.CapabilityName,
				"priority":         item.Priority,
				"status":           item.Status,
				"description":      item.Description,
				"estimated_effort": item.EstimatedEffort,
				"created_at":       item.CreatedAt,
			},
			CreatedAt: item.CreatedAt,
		}
	}

	return artifacts
}

// GreenfieldFlowResult - Summary of the pre-flight execution
type GreenfieldFlowResult struct {
	DirectivesCreated    int `json:"directives_created"`
	BacklogItemsCreated  int `json:"backlog_items_created"`
	CapabilitiesRequired int `json:"capabilities_required"`
}
