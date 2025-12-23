package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"gohypo/internal/analysis"
	"gohypo/models"
	"gohypo/ports"
	"strings"
)

type GreenfieldAdapter struct {
	StructuredClient *ai.StructuredClient[models.GreenfieldResearchOutput]
	LogicalAuditor   *LogicalAuditorAdapter
	Scout            *ai.ForensicScout
}

func NewGreenfieldAdapter(config *models.AIConfig) *GreenfieldAdapter {
	// Create a reasonable token limit for hypothesis generation
	// gpt-5.2 has 8192 token context limit, so limit completion to ~5000 tokens
	reasonableConfig := *config // copy config
	if reasonableConfig.MaxTokens > 5000 {
		reasonableConfig.MaxTokens = 5000 // Reasonable limit for hypothesis generation
	}

	return &GreenfieldAdapter{
		StructuredClient: ai.NewStructuredClientLegacy[models.GreenfieldResearchOutput](&reasonableConfig, config.PromptsDir),
		LogicalAuditor:   NewLogicalAuditorAdapter(config),
		Scout:            ai.NewForensicScout(config),
	}
}

// GetScout returns the Forensic Scout instance (for direct access)
func (ga *GreenfieldAdapter) GetScout() *ai.ForensicScout {
	return ga.Scout
}

func (ga *GreenfieldAdapter) GenerateResearchDirectives(ctx context.Context, req ports.GreenfieldResearchRequest) (*ports.GreenfieldResearchResponse, error) {
	// Extract field names for scout analysis
	fieldNames := make([]string, len(req.FieldMetadata))
	for i, field := range req.FieldMetadata {
		fieldNames[i] = field.Name
	}

	scoutResponse, err := ga.Scout.AnalyzeFields(ctx, fieldNames)
	if err != nil {
		scoutResponse = nil
	}

	// Build evidence orchestration
	orchestrator := analysis.NewEvidenceOrchestrator()
	outcomeCol := ga.determineOutcomeColumn(req.FieldMetadata)

	evidenceBrief := orchestrator.OrchestrateEvidence(
		req.FieldMetadata,
		req.StatisticalArtifacts,
		outcomeCol,
		[]string{},
		map[string]string{},
	)

	dynamicPrompt := ga.buildDynamicResearchPrompt(evidenceBrief, req.FieldMetadata)

	systemMessage := "You are a statistical research assistant. For dynamic e-value validation, you must select at least 1 referee from the approved list based on the hypothesis requirements. Output valid JSON only."

	llmResponse, err := ga.StructuredClient.GetJsonResponseWithContext(ctx, "openai", dynamicPrompt, systemMessage)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Use industry context from scout result if available
	if llmResponse.IndustryContext == "" && scoutResponse != nil {
		llmResponse.IndustryContext = fmt.Sprintf("%s dataset: %s", scoutResponse.Domain, scoutResponse.DatasetName)
	}

	// Enhance directives with referee selections from logical auditor
	enhancedDirectives, err := ga.enhanceDirectivesWithReferees(ctx, llmResponse.ResearchDirectives, req.FieldMetadata, req.StatisticalArtifacts)
	if err != nil {
		enhancedDirectives = llmResponse.ResearchDirectives
	}

	// Convert LLM response to domain objects
	directives := ga.convertToDomainDirectives(enhancedDirectives)

	// Generate engineering backlog from the directives
	engineeringBacklog := ga.generateEngineeringBacklog(directives)

	return &ports.GreenfieldResearchResponse{
		Directives:         directives,
		EngineeringBacklog: engineeringBacklog,
		RawLLMResponse:     llmResponse,   // Preserve raw LLM response for worker
		RenderedPrompt:     dynamicPrompt, // Preserve dynamic prompt for debugging
		Audit: ports.GreenfieldAudit{
			GeneratorType: "llm",
			Model:         "gpt-5.2", // TODO: Get from LLMClient
			Temperature:   0.1,       // TODO: Get from LLMClient
		},
	}, nil
}

func (ga *GreenfieldAdapter) convertToDomainDirectives(llmDirectives []models.ResearchDirectiveResponse) []greenfield.ResearchDirective {
	directives := make([]greenfield.ResearchDirective, len(llmDirectives))

	for i, llmDir := range llmDirectives {
		directives[i] = greenfield.ResearchDirective{
			ID:        greenfield.ResearchDirectiveID(core.NewID()),
			Claim:     llmDir.Claim,
			CauseKey:  core.VariableKey(llmDir.CauseKey),
			EffectKey: core.VariableKey(llmDir.EffectKey),
			LogicType: llmDir.LogicType,
			ValidationStrategy: greenfield.ValidationStrategy{
				Detector: llmDir.ValidationStrategy.Detector,
				Scanner:  llmDir.ValidationStrategy.Scanner,
				Proxy:    llmDir.ValidationStrategy.Proxy,
			},
			RefereeGates: greenfield.RefereeGates{
				PValueThreshold: llmDir.RefereeGates.PValueThreshold,
				StabilityScore:  llmDir.RefereeGates.StabilityScore,
				PermutationRuns: llmDir.RefereeGates.PermutationRuns,
			},
			ExplanationMarkdown: llmDir.ExplanationMarkdown,
			CreatedAt:           core.Now(),
		}
	}

	return directives
}

func (ga *GreenfieldAdapter) generateEngineeringBacklog(directives []greenfield.ResearchDirective) []greenfield.EngineeringBacklogItem {
	var backlog []greenfield.EngineeringBacklogItem
	priority := 1

	// Track unique capabilities to avoid duplicates
	capabilitySet := make(map[string]bool)

	for _, directive := range directives {
		strategy := directive.ValidationStrategy

		// Add detector capability
		if strategy.Detector != "" && !capabilitySet[strategy.Detector] {
			capabilitySet[strategy.Detector] = true
			backlog = append(backlog, greenfield.EngineeringBacklogItem{
				ID:              core.NewID(),
				DirectiveID:     directive.ID,
				CapabilityType:  "detector",
				CapabilityName:  strategy.Detector,
				Priority:        priority,
				Status:          greenfield.BacklogStatusPending,
				Description:     ga.getCapabilityDescription("detector", strategy.Detector),
				EstimatedEffort: ga.estimateEffort(strategy.Detector),
				CreatedAt:       core.Now(),
			})
			priority++
		}

		// Add scanner capability
		if strategy.Scanner != "" && !capabilitySet[strategy.Scanner] {
			capabilitySet[strategy.Scanner] = true
			backlog = append(backlog, greenfield.EngineeringBacklogItem{
				ID:              core.NewID(),
				DirectiveID:     directive.ID,
				CapabilityType:  "scanner",
				CapabilityName:  strategy.Scanner,
				Priority:        priority,
				Status:          greenfield.BacklogStatusPending,
				Description:     ga.getCapabilityDescription("scanner", strategy.Scanner),
				EstimatedEffort: ga.estimateEffort(strategy.Scanner),
				CreatedAt:       core.Now(),
			})
			priority++
		}

		// Add proxy capability
		if strategy.Proxy != "" && !capabilitySet[strategy.Proxy] {
			capabilitySet[strategy.Proxy] = true
			backlog = append(backlog, greenfield.EngineeringBacklogItem{
				ID:              core.NewID(),
				DirectiveID:     directive.ID,
				CapabilityType:  "proxy",
				CapabilityName:  strategy.Proxy,
				Priority:        priority,
				Status:          greenfield.BacklogStatusPending,
				Description:     ga.getCapabilityDescription("proxy", strategy.Proxy),
				EstimatedEffort: ga.estimateEffort(strategy.Proxy),
				CreatedAt:       core.Now(),
			})
			priority++
		}
	}

	return backlog
}

func (ga *GreenfieldAdapter) getCapabilityDescription(capType, capName string) string {
	descriptions := map[string]map[string]string{
		"detector": {
			"mutual_information":              "Measure non-linear dependencies using information theory",
			"spearmans_rho":                   "Calculate monotonic relationships using rank correlation",
			"distance_correlation":            "Detect complex non-linear relationships",
			"maximal_information_coefficient": "Find optimal functional relationships",
		},
		"scanner": {
			"quantile_split":           "Divide data into quantiles for threshold analysis",
			"k_means":                  "Cluster data points for subgroup analysis",
			"decision_tree_split":      "Find optimal splits for interaction detection",
			"density_based_clustering": "Identify natural groupings in data",
		},
		"proxy": {
			"shap_values":                 "Explain individual predictions and feature importance",
			"random_forest_importance":    "Measure variable importance through ensemble methods",
			"partial_dependence_plots":    "Visualize marginal effects of variables",
			"feature_interaction_effects": "Quantify variable interactions",
		},
	}

	if typeMap, exists := descriptions[capType]; exists {
		if desc, exists := typeMap[capName]; exists {
			return desc
		}
	}

	return fmt.Sprintf("Implement %s capability: %s", capType, capName)
}

func (ga *GreenfieldAdapter) estimateEffort(capability string) string {
	effortMap := map[string]string{
		// Detectors - generally complex
		"mutual_information":              "large",
		"spearmans_rho":                   "small",
		"distance_correlation":            "large",
		"maximal_information_coefficient": "large",

		// Scanners - medium complexity
		"quantile_split":           "small",
		"k_means":                  "small",
		"decision_tree_split":      "medium",
		"density_based_clustering": "medium",

		// Proxies - ML heavy
		"shap_values":                 "large",
		"random_forest_importance":    "medium",
		"partial_dependence_plots":    "medium",
		"feature_interaction_effects": "large",
	}

	if effort, exists := effortMap[capability]; exists {
		return effort
	}

	return "medium" // Default
}

// buildDynamicResearchPrompt creates prompt content from evidence
func (ga *GreenfieldAdapter) buildDynamicResearchPrompt(evidenceBrief *analysis.EvidenceBrief, fieldMetadata []greenfield.FieldMetadata) string {

	evidenceJSON, err := json.MarshalIndent(evidenceBrief, "", "  ")
	if err != nil {
		evidenceJSON = []byte(fmt.Sprintf("Error marshaling evidence: %v", err))
	}

	fieldMetadataJSON, err := json.MarshalIndent(fieldMetadata, "", "  ")
	if err != nil {
		fieldMetadataJSON = []byte(fmt.Sprintf("Error marshaling field metadata: %v", err))
	}

	replacements := map[string]string{
		"FIELD_METADATA_JSON":          string(fieldMetadataJSON),
		"INDUSTRY_CONTEXT_INJECTION":   "Industry context will be injected by the adapter.",
		"STATISTICAL_EVIDENCE_JSON":    string(evidenceJSON),
		"VALIDATED_HYPOTHESIS_SUMMARY": "No validated hypotheses available for feedback learning.",
	}

	prompt, err := ga.StructuredClient.PromptManager.RenderPrompt("greenfield", replacements)
	if err != nil {
		return fmt.Sprintf("INPUT DATA:\n%s\n\nGenerate 3 research hypotheses as JSON.", string(evidenceJSON))
	}

	return prompt
}

// determineOutcomeColumn identifies the most likely outcome column from field metadata
func (ga *GreenfieldAdapter) determineOutcomeColumn(metadata []greenfield.FieldMetadata) string {
	// Look for common outcome column names
	outcomePatterns := []string{"totalamount", "orderstatus", "conversion", "revenue", "sales", "profit"}

	for _, field := range metadata {
		fieldName := string(field.Name)
		for _, pattern := range outcomePatterns {
			if strings.Contains(strings.ToLower(fieldName), pattern) {
				return fieldName
			}
		}
	}

	// Fallback to first numeric field
	for _, field := range metadata {
		if field.DataType == "numeric" || field.DataType == "integer" || field.DataType == "float" {
			return string(field.Name)
		}
	}

	// Ultimate fallback
	if len(metadata) > 0 {
		return string(metadata[0].Name)
	}

	return "unknown_outcome"
}

// enhanceDirectivesWithReferees calls the logical auditor for each directive to get referee selections
func (ga *GreenfieldAdapter) enhanceDirectivesWithReferees(
	ctx context.Context,
	directives []models.ResearchDirectiveResponse,
	fieldMetadata []greenfield.FieldMetadata,
	statisticalArtifacts []map[string]interface{},
) ([]models.ResearchDirectiveResponse, error) {

	enhanced := make([]models.ResearchDirectiveResponse, len(directives))

	for i, directive := range directives {
		// Convert field metadata to JSON string
		fieldMetadataJSON, err := json.Marshal(fieldMetadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal field metadata: %w", err)
		}

		// Convert statistical artifacts to JSON string
		statisticalJSON, err := json.Marshal(statisticalArtifacts)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal statistical artifacts: %w", err)
		}

		// Call logical auditor
		auditorReq := ports.LogicalAuditorRequest{
			BusinessHypothesis:  directive.BusinessHypothesis,
			ScienceHypothesis:   directive.ScienceHypothesis,
			NullCase:            directive.NullCase,
			CauseKey:            directive.CauseKey,
			EffectKey:           directive.EffectKey,
			StatisticalEvidence: string(statisticalJSON),
			VariableContext:     string(fieldMetadataJSON),
			RigorLevel:          "decision-critical", // Default to high rigor for new hypotheses
			ComputationalBudget: "medium",
		}

		auditorResponse, err := ga.LogicalAuditor.GenerateRefereeSelection(ctx, auditorReq)
		if err != nil {
			// Use default referee selection
			directive.RefereeGates = models.RefereeGates{
				SelectedReferees: []models.RefereeSelection{
					{Name: "Permutation_Shredder", Category: "VALIDATION", Priority: 1},
					{Name: "Chow_Stability_Test", Category: "VALIDATION", Priority: 2},
					{Name: "Transfer_Entropy", Category: "CAUSALITY", Priority: 3},
				},
				ConfidenceTarget: 0.95,
				Rationale:        "Default referee selection due to auditor failure",
			}
		} else {
			// Use auditor's referee selection
			directive.RefereeGates = auditorResponse.RefereeDirective
		}

		enhanced[i] = directive
	}

	return enhanced, nil
}
