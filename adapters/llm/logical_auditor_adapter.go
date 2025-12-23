package llm

import (
	"context"
	"fmt"
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

// buildAuditorPrompt constructs the prompt for auditor analysis
func (laa *LogicalAuditorAdapter) buildAuditorPrompt(req ports.LogicalAuditorRequest) (string, error) {
	templateData := map[string]string{
		"business_hypothesis":       req.BusinessHypothesis,
		"science_hypothesis":        req.ScienceHypothesis,
		"null_case":                 req.NullCase,
		"cause_key":                 req.CauseKey,
		"effect_key":                req.EffectKey,
		"statistical_relationships": req.StatisticalEvidence,
		"variable_context":          req.VariableContext,
		"sample_size":               fmt.Sprintf("%d", req.SampleSize),
		"sparsity_ratio":            fmt.Sprintf("%.3f", req.SparsityRatio),
		"cardinality_cause":         fmt.Sprintf("%d", req.CardinalityCause),
		"cardinality_effect":        fmt.Sprintf("%d", req.CardinalityEffect),
		"skewness_cause":            fmt.Sprintf("%.3f", req.SkewnessCause),
		"skewness_effect":           fmt.Sprintf("%.3f", req.SkewnessEffect),
		"temporal_coverage":         fmt.Sprintf("%.3f", req.TemporalCoverage),
		"confounding_signals":       req.ConfoundingSignals,
		"rigor_level":               req.RigorLevel,
		"computational_budget":      req.ComputationalBudget,
	}

	prompt, err := laa.StructuredClient.PromptManager.RenderPrompt("auditor", templateData)
	if err != nil {
		return "", fmt.Errorf("failed to render auditor prompt template: %w", err)
	}

	return prompt, nil
}
