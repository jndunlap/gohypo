package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/domain/discovery"
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
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")
	fmt.Printf("[GreenfieldAdapter] 🚀 STARTING RESEARCH DIRECTIVE GENERATION\n")
	fmt.Printf("[GreenfieldAdapter] Session: %s\n", req.RunID)
	fmt.Printf("[GreenfieldAdapter] Input: %d fields, %d statistical artifacts\n", len(req.FieldMetadata), len(req.StatisticalArtifacts))
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")

	// Step 1: Run Forensic Scout to get industry context (Layer 2)
	fmt.Printf("[GreenfieldAdapter] 🔍 Step 1: Running Forensic Scout for industry context...\n")
	scoutResponse, err := ga.Scout.ExtractIndustryContext(ctx)
	if err != nil {
		// Log error but don't fail - continue without industry context
		fmt.Printf("[GreenfieldAdapter] ⚠️  Warning: Scout failed, continuing without industry context: %v\n", err)
		scoutResponse = nil
	} else {
		fmt.Printf("[GreenfieldAdapter] ✅ Forensic Scout completed successfully\n")
		fmt.Printf("[GreenfieldAdapter]   • Domain: %s\n", scoutResponse.Domain)
		fmt.Printf("[GreenfieldAdapter]   • Context: %s\n", scoutResponse.Context)
	}

	// Step 2: Build Layer 3 dynamic content
	fmt.Printf("[GreenfieldAdapter] 📝 Step 2: Building dynamic research prompt from greenfield_research.txt template...\n")
	// Convert DiscoveryBriefs from interface{} to []discovery.DiscoveryBrief
	var discoveryBriefs []discovery.DiscoveryBrief
	if briefs, ok := req.DiscoveryBriefs.([]discovery.DiscoveryBrief); ok {
		discoveryBriefs = briefs
	}

	dynamicPrompt := ga.buildDynamicResearchPrompt(req.FieldMetadata, req.StatisticalArtifacts, discoveryBriefs)
	fmt.Printf("[GreenfieldAdapter] ✅ Dynamic prompt built successfully (length: %d chars)\n", len(dynamicPrompt))

	fmt.Printf("[GreenfieldAdapter] 🧠 Step 3: Calling LLM with greenfield_research prompt...\n")

	// Call LLM with dynamic prompt and STRICT JSON system instructions
	systemMessage := "You are a statistical research assistant. CRITICAL: Each hypothesis MUST have EXACTLY 3 referees in the selected_referees array. Output valid JSON only."
	fmt.Printf("[GreenfieldAdapter] 📤 Sending request to LLM with greenfield_research prompt...\n")
	fmt.Printf("[GreenfieldAdapter]   • System message: %s\n", systemMessage)
	fmt.Printf("[GreenfieldAdapter]   • Prompt length: %d chars\n", len(dynamicPrompt))

	llmResponse, err := ga.StructuredClient.GetJsonResponseWithContext(ctx, "openai", dynamicPrompt, systemMessage)
	if err != nil {
		fmt.Printf("[GreenfieldAdapter] ❌ LLM CALL FAILED: %v\n", err)
		fmt.Printf("[GreenfieldAdapter] 💥 Prompt used: greenfield_research.txt (rendered)\n")
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	fmt.Printf("[GreenfieldAdapter] ✅ LLM response received successfully\n")

	// DEBUG: Log the LLM response to inspect referee selections
	fmt.Printf("[GreenfieldAdapter] ═══ LLM RESPONSE DEBUG (from greenfield_research.txt) ═══\n")
	fmt.Printf("[GreenfieldAdapter] Generated %d research directives\n", len(llmResponse.ResearchDirectives))
	for i, directive := range llmResponse.ResearchDirectives {
		fmt.Printf("[GreenfieldAdapter] Directive %d: %s\n", i+1, directive.ID)
		fmt.Printf("[GreenfieldAdapter]   • Business Hypothesis: %s\n", directive.BusinessHypothesis)
		fmt.Printf("[GreenfieldAdapter]   • Cause → Effect: %s → %s\n", directive.CauseKey, directive.EffectKey)
		fmt.Printf("[GreenfieldAdapter]   • Referees (%d): %v\n", len(directive.RefereeGates.SelectedReferees), directive.RefereeGates.SelectedReferees)
		if len(directive.RefereeGates.SelectedReferees) != 3 {
			fmt.Printf("[GreenfieldAdapter] ⚠️  WARNING: Directive %s has %d referees instead of 3!\n", directive.ID, len(directive.RefereeGates.SelectedReferees))
			fmt.Printf("[GreenfieldAdapter] 🚨 This will cause validation to FAIL!\n")
		} else {
			fmt.Printf("[GreenfieldAdapter] ✅ Referee count is correct (3)\n")
		}
	}
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")

	// Use industry context from LLM response if available, otherwise use scout result
	if llmResponse.IndustryContext == "" && scoutResponse != nil {
		// Format structured scout response for hypothesis generation
		llmResponse.IndustryContext = fmt.Sprintf("%s industry analysis: %s. Primary challenge: %s. Data characteristics: %s (%s).",
			scoutResponse.Domain, scoutResponse.Context, scoutResponse.Bottleneck, scoutResponse.Map, scoutResponse.Physics)
	}

	// Convert LLM response to domain objects
	directives := ga.convertToDomainDirectives(llmResponse.ResearchDirectives)

	// Generate engineering backlog from the directives
	fmt.Printf("[GreenfieldAdapter] 📋 Step 4: Generating engineering backlog...\n")
	engineeringBacklog := ga.generateEngineeringBacklog(directives)
	fmt.Printf("[GreenfieldAdapter] ✅ Engineering backlog created: %d items\n", len(engineeringBacklog))

	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")
	fmt.Printf("[GreenfieldAdapter] 🎉 RESEARCH DIRECTIVE GENERATION COMPLETE\n")
	fmt.Printf("[GreenfieldAdapter] Summary:\n")
	fmt.Printf("[GreenfieldAdapter]   • Prompt template: prompts/greenfield_research.txt\n")
	fmt.Printf("[GreenfieldAdapter]   • Directives generated: %d\n", len(directives))
	fmt.Printf("[GreenfieldAdapter]   • Engineering backlog items: %d\n", len(engineeringBacklog))
	fmt.Printf("[GreenfieldAdapter]   • LLM model: %s\n", ga.StructuredClient.OpenAIClient.Model)
	fmt.Printf("[GreenfieldAdapter]   • Temperature: %.2f\n", ga.StructuredClient.OpenAIClient.Temperature)
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")

	return &ports.GreenfieldResearchResponse{
		Directives:         directives,
		EngineeringBacklog: engineeringBacklog,
		RawLLMResponse:     llmResponse,   // Preserve raw LLM response for worker
		RenderedPrompt:     dynamicPrompt, // Preserve dynamic prompt for debugging
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

// buildDynamicResearchPrompt creates Layer 3 content (variable per research session)
func (ga *GreenfieldAdapter) buildDynamicResearchPrompt(metadata []greenfield.FieldMetadata, stats []map[string]interface{}, briefs []discovery.DiscoveryBrief) string {
	fmt.Printf("[GreenfieldAdapter] ═══ PROMPT BUILDING PROCESS ═══\n")
	fmt.Printf("[GreenfieldAdapter] 📋 Building context data for prompt injection...\n")

	// Build comprehensive context JSON including field metadata, stats artifacts, and discovery briefs
	contextData := map[string]interface{}{
		"field_metadata":        metadata,
		"statistical_artifacts": stats,
		"discovery_briefs":      briefs,
		"total_fields":          len(metadata),
		"total_stats_artifacts": len(stats),
	}

	fmt.Printf("[GreenfieldAdapter]   • Field metadata entries: %d\n", len(metadata))
	fmt.Printf("[GreenfieldAdapter]   • Statistical artifacts: %d\n", len(stats))
	fmt.Printf("[GreenfieldAdapter]   • Discovery briefs: %d\n", len(briefs))

	fieldJSON, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		// Fallback to simple string representation
		fmt.Printf("[GreenfieldAdapter] ❌ JSON marshaling failed: %v\n", err)
		fieldJSON = []byte(fmt.Sprintf("Error marshaling context: %v", err))
	} else {
		fmt.Printf("[GreenfieldAdapter] ✅ Context JSON marshaled successfully (%d bytes)\n", len(fieldJSON))
	}

	// Prepare template replacements
	replacements := map[string]string{
		"FIELD_METADATA_JSON":        string(fieldJSON),
		"INDUSTRY_CONTEXT_INJECTION": "Industry context will be injected by the adapter.",
	}

	fmt.Printf("[GreenfieldAdapter] 🔧 Loading template: prompts/greenfield_research.txt\n")
	fmt.Printf("[GreenfieldAdapter] 📝 Rendering prompt with dynamic replacements...\n")

	// Render the full greenfield research prompt
	prompt, err := ga.StructuredClient.PromptManager.RenderPrompt("greenfield_research", replacements)
	if err != nil {
		fmt.Printf("[GreenfieldAdapter] ❌ CRITICAL: Failed to render greenfield_research.txt template: %v\n", err)
		fmt.Printf("[GreenfieldAdapter] 🔄 Using minimal fallback prompt instead\n")
		// Fallback to minimal prompt
		return fmt.Sprintf("INPUT DATA:\n%s\n\nGenerate 3 research hypotheses as JSON.", string(fieldJSON))
	}

	fmt.Printf("[GreenfieldAdapter] ✅ Successfully rendered greenfield_research.txt prompt\n")
	fmt.Printf("[GreenfieldAdapter]   • Final prompt length: %d characters\n", len(prompt))
	fmt.Printf("[GreenfieldAdapter]   • Template replacements: %d\n", len(replacements))
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════\n")

	return prompt
}

// storeThinkingTrace saves the LLM reasoning for debugging and analysis
func (ga *GreenfieldAdapter) storeThinkingTrace(req ports.GreenfieldResearchRequest, thinkingTrace string) {
	// For now, just log the trace length - in production this could be stored in a database
	fmt.Printf("[GreenfieldAdapter] Stored thinking trace (%d chars) for session analysis\n", len(thinkingTrace))

	// TODO: Implement persistent storage for thinking traces
	// This could be useful for:
	// 1. Debugging failed hypotheses
	// 2. Understanding LLM decision-making
	// 3. Training data for future improvements
}
