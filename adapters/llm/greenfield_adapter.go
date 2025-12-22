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
	StructuredClient  *ai.StructuredClient[models.GreenfieldResearchOutput]
	LogicalAuditor    *LogicalAuditorAdapter
	Scout             *ai.ForensicScout
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
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")
	fmt.Printf("[GreenfieldAdapter] 🚀 STARTING RESEARCH DIRECTIVE GENERATION\n")
	fmt.Printf("[GreenfieldAdapter] Session: %s\n", req.RunID)
	fmt.Printf("[GreenfieldAdapter] Input: %d fields, %d statistical artifacts\n", len(req.FieldMetadata), len(req.StatisticalArtifacts))
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")

	// Step 1: Run Forensic Scout to get industry context (Layer 2)
	fmt.Printf("[GreenfieldAdapter] 🔍 Step 1: Running Forensic Scout for industry context...\n")

	// Extract field names from the request metadata for scout analysis
	fieldNames := make([]string, len(req.FieldMetadata))
	for i, field := range req.FieldMetadata {
		fieldNames[i] = field.Name
	}

	scoutResponse, err := ga.Scout.AnalyzeFields(ctx, fieldNames)
	if err != nil {
		// Log error but don't fail - continue without industry context
		fmt.Printf("[GreenfieldAdapter] ⚠️  Warning: Scout failed, continuing without industry context: %v\n", err)
		scoutResponse = nil
	} else {
		fmt.Printf("[GreenfieldAdapter] ✅ Forensic Scout completed successfully\n")
		fmt.Printf("[GreenfieldAdapter]   • Domain: %s\n", scoutResponse.Domain)
		fmt.Printf("[GreenfieldAdapter]   • Dataset: %s\n", scoutResponse.DatasetName)
	}

	// Step 2: Build Layer 3 dynamic content with evidence orchestration
	fmt.Printf("[GreenfieldAdapter] 📝 Step 2: Orchestrating evidence for LLM input...\n")

	// Create evidence orchestrator and transform raw data into LLM-friendly format
	orchestrator := analysis.NewEvidenceOrchestrator()
	outcomeCol := ga.determineOutcomeColumn(req.FieldMetadata)
	fmt.Printf("[GreenfieldAdapter] 📊 Orchestrating evidence...\n")
	fmt.Printf("[GreenfieldAdapter]   • Outcome column: %s\n", outcomeCol)
	fmt.Printf("[GreenfieldAdapter]   • Statistical artifacts: %d\n", len(req.StatisticalArtifacts))

	evidenceBrief := orchestrator.OrchestrateEvidence(
		req.FieldMetadata,
		req.StatisticalArtifacts,
		outcomeCol,
		[]string{}, // Would be populated from actual analysis
		map[string]string{}, // Would be populated from actual analysis
	)

	fmt.Printf("[GreenfieldAdapter] ✅ Evidence orchestration completed\n")
	totalItems := len(evidenceBrief.Associations) + len(evidenceBrief.Breakpoints) + len(evidenceBrief.Interactions) + len(evidenceBrief.StructuralBreaks) + len(evidenceBrief.TransferEntropies) + len(evidenceBrief.HysteresisEffects)
	fmt.Printf("[GreenfieldAdapter]   • Evidence brief items: %d total\n", totalItems)
	fmt.Printf("[GreenfieldAdapter]     - Associations: %d\n", len(evidenceBrief.Associations))
	fmt.Printf("[GreenfieldAdapter]     - Breakpoints: %d\n", len(evidenceBrief.Breakpoints))
	fmt.Printf("[GreenfieldAdapter]     - Interactions: %d\n", len(evidenceBrief.Interactions))
	fmt.Printf("[GreenfieldAdapter]     - Structural breaks: %d\n", len(evidenceBrief.StructuralBreaks))
	fmt.Printf("[GreenfieldAdapter]     - Transfer entropies: %d\n", len(evidenceBrief.TransferEntropies))
	fmt.Printf("[GreenfieldAdapter]     - Hysteresis effects: %d\n", len(evidenceBrief.HysteresisEffects))

	dynamicPrompt := ga.buildDynamicResearchPrompt(evidenceBrief, req.FieldMetadata)
	fmt.Printf("[GreenfieldAdapter] ✅ Dynamic prompt built successfully (length: %d chars)\n", len(dynamicPrompt))

	fmt.Printf("[GreenfieldAdapter] 🧠 Step 3: Calling LLM with greenfield_research prompt...\n")

	// Call LLM with dynamic prompt and STRICT JSON system instructions
	systemMessage := "You are a statistical research assistant. For dynamic e-value validation, you may select any number of referees (including 0) based on the hypothesis requirements. Output valid JSON only."
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
		llmResponse.IndustryContext = fmt.Sprintf("%s dataset: %s", scoutResponse.Domain, scoutResponse.DatasetName)
	}

	// Enhance directives with referee selections from logical auditor
	fmt.Printf("[GreenfieldAdapter] 🔍 Step 5: Calling logical auditor for referee selection...\n")
	enhancedDirectives, err := ga.enhanceDirectivesWithReferees(ctx, llmResponse.ResearchDirectives, req.FieldMetadata, req.StatisticalArtifacts)
	if err != nil {
		fmt.Printf("[GreenfieldAdapter] ⚠️  Warning: Logical auditor failed, using default referees: %v\n", err)
		enhancedDirectives = llmResponse.ResearchDirectives
	} else {
		fmt.Printf("[GreenfieldAdapter] ✅ Logical auditor enhanced %d directives with referee selections\n", len(enhancedDirectives))
	}

	// Convert LLM response to domain objects
	directives := ga.convertToDomainDirectives(enhancedDirectives)

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
	fmt.Printf("[GreenfieldAdapter]   • LLM model: %s\n", "gpt-5.2") // TODO: Get from LLMClient
	fmt.Printf("[GreenfieldAdapter]   • Temperature: %.2f\n", 0.1) // TODO: Get from LLMClient
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════════════════════════\n")

	return &ports.GreenfieldResearchResponse{
		Directives:         directives,
		EngineeringBacklog: engineeringBacklog,
		RawLLMResponse:     llmResponse,   // Preserve raw LLM response for worker
		RenderedPrompt:     dynamicPrompt, // Preserve dynamic prompt for debugging
		Audit: ports.GreenfieldAudit{
			GeneratorType: "llm",
			Model:         "gpt-5.2", // TODO: Get from LLMClient
			Temperature:   0.1,     // TODO: Get from LLMClient
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
func (ga *GreenfieldAdapter) buildDynamicResearchPrompt(evidenceBrief *analysis.EvidenceBrief, fieldMetadata []greenfield.FieldMetadata) string {
	fmt.Printf("[GreenfieldAdapter] ═══ PROMPT BUILDING PROCESS ═══\n")
	fmt.Printf("[GreenfieldAdapter] 📋 Building LLM-optimized evidence context...\n")

	// Convert EvidenceBrief to JSON for prompt injection
	fmt.Printf("[GreenfieldAdapter]   • Evidence associations: %d\n", len(evidenceBrief.Associations))
	fmt.Printf("[GreenfieldAdapter]   • Breakpoints: %d\n", len(evidenceBrief.Breakpoints))
	fmt.Printf("[GreenfieldAdapter]   • Interactions: %d\n", len(evidenceBrief.Interactions))
	fmt.Printf("[GreenfieldAdapter]   • Structural breaks: %d\n", len(evidenceBrief.StructuralBreaks))

	evidenceJSON, err := json.MarshalIndent(evidenceBrief, "", "  ")
	if err != nil {
		// Fallback to simple string representation
		fmt.Printf("[GreenfieldAdapter] ❌ Evidence JSON marshaling failed: %v\n", err)
		evidenceJSON = []byte(fmt.Sprintf("Error marshaling evidence: %v", err))
	} else {
		fmt.Printf("[GreenfieldAdapter] ✅ Evidence JSON marshaled successfully (%d bytes)\n", len(evidenceJSON))
	}

	// Convert field metadata to JSON
	fieldMetadataJSON, err := json.MarshalIndent(fieldMetadata, "", "  ")
	if err != nil {
		fmt.Printf("[GreenfieldAdapter] ❌ Field metadata JSON marshaling failed: %v\n", err)
		fieldMetadataJSON = []byte(fmt.Sprintf("Error marshaling field metadata: %v", err))
	} else {
		fmt.Printf("[GreenfieldAdapter] ✅ Field metadata JSON marshaled successfully (%d bytes)\n", len(fieldMetadataJSON))
	}

	// Prepare template replacements
	replacements := map[string]string{
		"FIELD_METADATA_JSON":           string(fieldMetadataJSON),
		"INDUSTRY_CONTEXT_INJECTION":    "Industry context will be injected by the adapter.",
		"STATISTICAL_EVIDENCE_JSON":     string(evidenceJSON),
		"VALIDATED_HYPOTHESIS_SUMMARY": "No validated hypotheses available for feedback learning.", // Placeholder
	}

	fmt.Printf("[GreenfieldAdapter] 🔧 Loading template: prompts/greenfield_research.txt\n")
	fmt.Printf("[GreenfieldAdapter] 📝 Rendering prompt with dynamic replacements...\n")

	// Render the full greenfield research prompt
	prompt, err := ga.StructuredClient.PromptManager.RenderPrompt("greenfield_research", replacements)
	if err != nil {
		fmt.Printf("[GreenfieldAdapter] ❌ CRITICAL: Failed to render greenfield_research.txt template: %v\n", err)
		fmt.Printf("[GreenfieldAdapter] 🔄 Using minimal fallback prompt instead\n")
		// Fallback to minimal prompt
		return fmt.Sprintf("INPUT DATA:\n%s\n\nGenerate 3 research hypotheses as JSON.", string(evidenceJSON))
	}

	fmt.Printf("[GreenfieldAdapter] ✅ Successfully rendered greenfield_research.txt prompt\n")
	fmt.Printf("[GreenfieldAdapter]   • Final prompt length: %d characters\n", len(prompt))
	fmt.Printf("[GreenfieldAdapter]   • Template replacements: %d\n", len(replacements))
	fmt.Printf("[GreenfieldAdapter] ═══════════════════════════════\n")

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
			NullCase:           directive.NullCase,
			CauseKey:           directive.CauseKey,
			EffectKey:          directive.EffectKey,
			StatisticalEvidence: string(statisticalJSON),
			VariableContext:    string(fieldMetadataJSON),
			RigorLevel:         "decision-critical", // Default to high rigor for new hypotheses
			ComputationalBudget: "medium",
		}

		auditorResponse, err := ga.LogicalAuditor.GenerateRefereeSelection(ctx, auditorReq)
		if err != nil {
			// Log error but continue with defaults
			fmt.Printf("[GreenfieldAdapter] ⚠️  Logical auditor failed for directive %s: %v\n", directive.ID, err)
			// Use default referee selection
			directive.RefereeGates = models.RefereeGates{
				SelectedReferees: []string{"Permutation_Shredder", "Chow_Stability_Test", "Transfer_Entropy"},
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
