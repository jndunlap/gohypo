package llm

import (
	"context"
	"gohypo/ai"
	"gohypo/models"
	"gohypo/ports"
)

// LogicalAuditorAdapter handles referee selection for hypotheses
type LogicalAuditorAdapter struct {
	StructuredClient *ai.StructuredClient[models.LogicalAuditorOutput]
}

// NewLogicalAuditorAdapter creates a new logical auditor adapter
func NewLogicalAuditorAdapter(config *models.AIConfig) *LogicalAuditorAdapter {
	return &LogicalAuditorAdapter{
		StructuredClient: ai.NewStructuredClientLegacy[models.LogicalAuditorOutput](config, config.PromptsDir),
	}
}

// GenerateRefereeSelection analyzes a hypothesis and selects appropriate statistical referees
func (laa *LogicalAuditorAdapter) GenerateRefereeSelection(ctx context.Context, req ports.LogicalAuditorRequest) (*models.LogicalAuditorOutput, error) {
	// Build the analysis prompt
	prompt, err := laa.buildAuditorPrompt(req)
	if err != nil {
		return nil, err
	}

	systemMessage := `You are the Senior Statistical Lead for GoHypo's Scientific Validation Division.
Your mandate is to issue precise technical directives that protect the platform from false discoveries while optimizing computational resources.

Select exactly 3 referees from different categories that collectively create a "statistical trap" proving the causal hypothesis.
Output valid JSON only.`

	result, err := laa.StructuredClient.GetJsonResponseWithContext(ctx, "openai", prompt, systemMessage)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// buildAuditorPrompt constructs the prompt for logical auditor analysis
func (laa *LogicalAuditorAdapter) buildAuditorPrompt(req ports.LogicalAuditorRequest) (string, error) {
	// Use the logical_auditor.txt template with dynamic content injection
	templateData := map[string]string{
		"BUSINESS_HYPOTHESIS": req.BusinessHypothesis,
		"SCIENCE_HYPOTHESIS":  req.ScienceHypothesis,
		"NULL_CASE":          req.NullCase,
		"STATISTICAL_RELATIONSHIP_JSON": req.StatisticalEvidence,
		"VARIABLE_CONTEXT_JSON":         req.VariableContext,
		"RIGOR_LEVEL":                   req.RigorLevel,
		"COMPUTATIONAL_BUDGET":         req.ComputationalBudget,
	}

	prompt, err := laa.StructuredClient.PromptManager.RenderPrompt("logical_auditor", templateData)
	if err != nil {
		return "", err
	}

	return prompt, nil
}
