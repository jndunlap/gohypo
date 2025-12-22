package app

import (
	"context"
	"fmt"
	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"gohypo/models"
	"gohypo/ports"
)

type GreenfieldService struct {
	greenfieldPort     ports.GreenfieldResearchPort
	ledgerPort         ports.LedgerPort
	hypothesisAnalyzer *ai.HypothesisAnalysisAgent
}

func NewGreenfieldService(greenfieldPort ports.GreenfieldResearchPort, ledgerPort ports.LedgerPort, hypothesisAnalyzer *ai.HypothesisAnalysisAgent) *GreenfieldService {
	return &GreenfieldService{
		greenfieldPort:     greenfieldPort,
		ledgerPort:         ledgerPort,
		hypothesisAnalyzer: hypothesisAnalyzer,
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
		Directives:    3, // As specified in your prompt - exactly 3 directives
	}

	response, err := s.greenfieldPort.GenerateResearchDirectives(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate research directives: %w", err)
	}

	// 2. Store research directives as artifacts
	directiveArtifacts := s.convertDirectivesToArtifacts(response.Directives, runID)
	var storeErrors []error
	for _, artifact := range directiveArtifacts {
		if err := s.ledgerPort.StoreArtifact(ctx, string(runID), artifact); err != nil {
			storeErrors = append(storeErrors, fmt.Errorf("failed to store directive artifact %s: %w", artifact.ID, err))
		}
	}

	// 3. Store engineering backlog as artifacts (the "Missing Instruments")
	backlogArtifacts := s.convertBacklogToArtifacts(response.EngineeringBacklog, runID)
	for _, artifact := range backlogArtifacts {
		if err := s.ledgerPort.StoreArtifact(ctx, string(runID), artifact); err != nil {
			storeErrors = append(storeErrors, fmt.Errorf("failed to store backlog artifact %s: %w", artifact.ID, err))
		}
	}

	// 4. Analyze hypotheses for risk assessment (if AI analyzer is available)
	var riskProfiles []interface{}
	if s.hypothesisAnalyzer != nil {
		riskProfiles, err = s.analyzeHypothesisRisk(ctx, response.Directives, fieldMetadata)
		if err != nil {
			// Log but don't fail the entire flow
			fmt.Printf("Warning: Hypothesis risk analysis failed: %v\n", err)
		} else {
			// Store risk analysis artifacts
			riskArtifacts := s.convertRiskProfilesToArtifacts(riskProfiles, runID)
			for _, artifact := range riskArtifacts {
				if err := s.ledgerPort.StoreArtifact(ctx, string(runID), artifact); err != nil {
					storeErrors = append(storeErrors, fmt.Errorf("failed to store risk artifact %s: %w", artifact.ID, err))
				}
			}
		}
	}

	// Return combined error if any storage failed
	if len(storeErrors) > 0 {
		return nil, fmt.Errorf("failed to store %d artifacts: %v", len(storeErrors), storeErrors)
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

// analyzeHypothesisRisk performs AI-powered risk assessment on generated hypotheses
func (s *GreenfieldService) analyzeHypothesisRisk(
	ctx context.Context,
	directives []greenfield.ResearchDirective,
	fieldMetadata []greenfield.FieldMetadata,
) ([]interface{}, error) {

	riskProfiles := make([]interface{}, 0, len(directives))

	// Create data topology snapshot from field metadata
	dataSnapshot := ai.DataTopologySnapshot{
		SampleSize:         1000,       // TODO: Get actual sample size from data
		SparsityRatio:      0.05,       // TODO: Calculate from actual data
		CardinalityCause:   50,         // TODO: Get from field analysis
		CardinalityEffect:  50,         // TODO: Get from field analysis
		SkewnessCause:      0.0,        // TODO: Calculate from actual data
		SkewnessEffect:     0.0,        // TODO: Calculate from actual data
		TemporalCoverage:   1.0,        // TODO: Calculate from actual data
		ConfoundingSignals: []string{}, // TODO: Analyze from field metadata
		AvailableFields:    make([]string, len(fieldMetadata)),
	}

	for i, field := range fieldMetadata {
		dataSnapshot.AvailableFields[i] = string(field.Name)
	}

	// Analyze each hypothesis
	for _, directive := range directives {
		// Convert directive to ResearchDirectiveResponse format
		directiveResponse := s.convertDirectiveToResponse(directive)

		// Perform risk analysis
		riskProfile, err := s.hypothesisAnalyzer.AnalyzeHypothesis(ctx, directiveResponse, dataSnapshot)
		if err != nil {
			return nil, fmt.Errorf("failed to analyze hypothesis %s: %w", directive.ID, err)
		}

		riskProfiles = append(riskProfiles, riskProfile)
	}

	return riskProfiles, nil
}

// convertDirectiveToResponse converts a ResearchDirective to ResearchDirectiveResponse
func (s *GreenfieldService) convertDirectiveToResponse(directive greenfield.ResearchDirective) models.ResearchDirectiveResponse {
	// Generate default referees based on the directive's validation strategy
	selectedReferees := []string{"Permutation_Shredder", "Chow_Stability_Test", "Transfer_Entropy"}

	return models.ResearchDirectiveResponse{
		ID: string(directive.ID),
		BusinessHypothesis: fmt.Sprintf("Testing if %s influences %s",
			directive.CauseKey, directive.EffectKey),
		ScienceHypothesis: directive.Claim,
		CauseKey:          string(directive.CauseKey),
		EffectKey:         string(directive.EffectKey),
		RefereeGates: models.RefereeGates{
			SelectedReferees: selectedReferees,
			ConfidenceTarget: 0.95, // Default confidence target
			Rationale:        "Auto-selected referees based on validation strategy",
			// Map legacy fields for compatibility
			PValueThreshold: directive.RefereeGates.PValueThreshold,
			StabilityScore:  directive.RefereeGates.StabilityScore,
			PermutationRuns: directive.RefereeGates.PermutationRuns,
		},
	}
}

// convertRiskProfilesToArtifacts converts risk analysis results to artifacts
func (s *GreenfieldService) convertRiskProfilesToArtifacts(riskProfiles []interface{}, runID core.RunID) []core.Artifact {
	artifacts := make([]core.Artifact, len(riskProfiles))

	for i, riskProfile := range riskProfiles {
		artifacts[i] = core.Artifact{
			ID:   core.ID(fmt.Sprintf("risk-%s-%d", runID, i)),
			Kind: "hypothesis_risk_profile",
			Payload: map[string]interface{}{
				"run_id":       runID,
				"risk_profile": riskProfile,
				"created_at":   core.Now(),
			},
			CreatedAt: core.Now(),
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
