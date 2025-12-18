package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"gohypo/models"
	"gohypo/ports"
)

type GreenfieldAdapter struct {
	StructuredClient *ai.StructuredClient[models.GreenfieldResearchOutput]
	Scout            *ai.ForensicScout
}

func NewGreenfieldAdapter(config *models.AIConfig) *GreenfieldAdapter {
	return &GreenfieldAdapter{
		StructuredClient: ai.NewStructuredClient[models.GreenfieldResearchOutput](config, config.PromptsDir),
		Scout:            ai.NewForensicScout(config),
	}
}

// GetScout returns the Forensic Scout instance (for direct access)
func (ga *GreenfieldAdapter) GetScout() *ai.ForensicScout {
	return ga.Scout
}

func (ga *GreenfieldAdapter) GenerateResearchDirectives(ctx context.Context, req ports.GreenfieldResearchRequest) (*ports.GreenfieldResearchResponse, error) {
	// Step 1: Run Forensic Scout to get industry context
	industryContext, err := ga.Scout.ExtractIndustryContext(ctx)
	if err != nil {
		// Log error but don't fail - continue without industry context
		fmt.Printf("[GreenfieldAdapter] Warning: Scout failed, continuing without industry context: %v\n", err)
		industryContext = ""
	}

	// Build industry context injection text
	industryContextInjection := ""
	if industryContext != "" {
		industryContextInjection = fmt.Sprintf("INDUSTRY CONTEXT (Forensic Scout Analysis):\n%s\n\n", industryContext)
	} else {
		industryContextInjection = "INDUSTRY CONTEXT: Not available (scout analysis skipped or failed).\n\n"
	}

	// Build comprehensive context JSON including field metadata, stats artifacts, and discovery briefs
	contextData := map[string]interface{}{
		"field_metadata":        req.FieldMetadata,
		"statistical_artifacts": req.StatisticalArtifacts,
		"discovery_briefs":      req.DiscoveryBriefs,
		"total_fields":          len(req.FieldMetadata),
		"total_stats_artifacts": len(req.StatisticalArtifacts),
	}

	fieldJSON, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal field metadata: %w", err)
	}

	// Prepare prompt replacements
	replacements := map[string]string{
		"FIELD_METADATA_JSON":        string(fieldJSON),
		"INDUSTRY_CONTEXT_INJECTION": industryContextInjection,
	}

	// Render and log the prompt for debugging
	renderedPrompt, err := ga.StructuredClient.PromptManager.RenderPrompt("greenfield_research", replacements)
	if err != nil {
		return nil, fmt.Errorf("failed to render prompt: %w", err)
	}
	fmt.Printf("[GreenfieldAdapter] Rendered prompt length: %d bytes\n", len(renderedPrompt))
	if len(renderedPrompt) > 500 {
		fmt.Printf("[GreenfieldAdapter] Prompt preview (first 500 chars): %s...\n", renderedPrompt[:500])
	}

	// Call LLM with external prompt
	llmResponse, err := ga.StructuredClient.GetJsonResponseFromPrompt("greenfield_research", replacements)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Use industry context from LLM response if available, otherwise use scout result
	if llmResponse.IndustryContext == "" && industryContext != "" {
		llmResponse.IndustryContext = industryContext
	}

	// Convert LLM response to domain objects
	directives := ga.convertToDomainDirectives(llmResponse.ResearchDirectives)

	// Generate engineering backlog from the directives
	engineeringBacklog := ga.generateEngineeringBacklog(directives)

	return &ports.GreenfieldResearchResponse{
		Directives:         directives,
		EngineeringBacklog: engineeringBacklog,
		RawLLMResponse:     llmResponse,    // Preserve raw LLM response for worker
		RenderedPrompt:     renderedPrompt, // Preserve rendered prompt with industry context for debugging
		Audit: ports.GreenfieldAudit{
			GeneratorType: "llm",
			Model:         ga.StructuredClient.OpenAIClient.Model,
			Temperature:   ga.StructuredClient.OpenAIClient.Temperature,
		},
	}, nil
}

func (ga *GreenfieldAdapter) convertToDomainDirectives(llmDirectives []models.ResearchDirectiveResponse) []greenfield.ResearchDirective {
	directives := make([]greenfield.ResearchDirective, len(llmDirectives))

	for i, llmDir := range llmDirectives {
		directives[i] = greenfield.ResearchDirective{
			ID:        greenfield.ResearchDirectiveID(core.NewID()),
			Claim:     llmDir.Claim,
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
			CreatedAt: core.Now(),
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
