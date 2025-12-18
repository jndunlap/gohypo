package discovery

import (
	"fmt"
	"strings"

	"gohypo/domain/core"
	"gohypo/ports"
)

// ============================================================================
// LLM PROMPT GENERATION
// ============================================================================

// GenerateHypothesisPrompt creates a comprehensive LLM prompt for hypothesis generation
func (db *DiscoveryBrief) GenerateHypothesisPrompt(
	req ports.HypothesisRequest,
	variableRegistry map[core.VariableKey]string, // Variable key -> description
) string {

	var prompt strings.Builder

	// Header with context
	prompt.WriteString(fmt.Sprintf(`# Hypothesis Generation for %s

## Discovery Overview
%s

## Statistical Evidence
%s

## Behavioral Insights
`, db.VariableKey, db.LLMContext.ExecutiveSummary, db.LLMContext.StatisticalSummary))

	// Add behavioral insights
	if len(db.LLMContext.BehavioralInsights) > 0 {
		for _, insight := range db.LLMContext.BehavioralInsights {
			prompt.WriteString(fmt.Sprintf("- %s\n", insight))
		}
	} else {
		prompt.WriteString("- No significant behavioral patterns detected\n")
	}

	// Add uncertainty factors if present
	if len(db.LLMContext.UncertaintyFactors) > 0 {
		prompt.WriteString("\n## Important Considerations\n")
		for _, factor := range db.LLMContext.UncertaintyFactors {
			prompt.WriteString(fmt.Sprintf("- %s\n", factor))
		}
	}

	// Variable descriptions
	prompt.WriteString("\n## Variable Context\n")
	prompt.WriteString("Available variables for hypothesis construction:\n")
	for varKey, description := range variableRegistry {
		prompt.WriteString(fmt.Sprintf("- %s: %s\n", varKey, description))
	}

	// Hypothesis seeds
	if len(db.LLMContext.HypothesisSeeds) > 0 {
		prompt.WriteString("\n## Hypothesis Seeds\n")
		prompt.WriteString("Consider these patterns as starting points:\n")
		for _, seed := range db.LLMContext.HypothesisSeeds {
			if seed.Priority > 0.6 { // Only include high-priority seeds
				prompt.WriteString(fmt.Sprintf("- %s (confidence: %.1f)\n",
					seed.Description, seed.Confidence))
			}
		}
	}

	// Generation instructions
	prompt.WriteString(fmt.Sprintf(`
## Generation Requirements

Generate up to %d hypotheses that explain the patterns observed in %s. Each hypothesis must:

### Structure Requirements
- **cause_key**: Primary variable driving the effect (must exist in variable list)
- **effect_key**: Variable being affected (must exist in variable list)
- **mechanism_category**: One of:
  - direct_causal: Direct cause-effect relationship
  - effect_modification: Variable changes how other relationships work
  - confounding_path: Hidden common causes create spurious relationships
  - proxy_relationship: Variables related through third factors
  - measurement_bias: Data collection issues create artificial patterns
- **confounder_keys**: Variables to control for (array, can be empty)
- **rationale**: 2-3 sentence explanation with specific evidence references
- **suggested_rigor**: Validation approach (%s)
- **supporting_evidence**: Array of evidence types that support this hypothesis

### Quality Criteria
- **Evidence-Based**: Reference specific statistical patterns from above
- **Parsimonious**: Prefer simple explanations over complex ones
- **Testable**: Must be falsifiable with available data
- **Domain-Aware**: Consider behavioral insights and uncertainty factors

### Rigor Guidance
- **basic**: Simple correlation checks
- **standard**: Control for confounding, check subgroups
- **decision**: Full causal inference methods, sensitivity analyses

Output only valid JSON array of hypothesis objects.`, req.MaxHypotheses, db.VariableKey, req.RigorProfile))

	return prompt.String()
}

// GenerateResearchDirectivePrompt creates prompts for research directive generation
func (db *DiscoveryBrief) GenerateResearchDirectivePrompt() string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`# Research Directive Generation for %s

## Statistical Discovery Summary
%s

## Evidence Strength Assessment
- Overall confidence: %.2f (%s risk)
- Evidence consistency: %.2f
- Method robustness: %.2f

## Key Patterns Identified
`,
		db.VariableKey,
		db.LLMContext.ExecutiveSummary,
		db.ConfidenceScore,
		db.RiskAssessment,
		db.LLMContext.EvidenceStrength.ConsistencyScore,
		db.LLMContext.EvidenceStrength.RobustnessScore))

	// Add sense-specific evidence
	senseEvidence := []string{}
	if db.MutualInformation.SampleSize > 0 {
		senseEvidence = append(senseEvidence,
			fmt.Sprintf("Mutual Information: %.3f (p=%.3f)", db.MutualInformation.NormalizedMI, db.MutualInformation.PValue))
	}
	if db.Spearman.SampleSize > 0 {
		senseEvidence = append(senseEvidence,
			fmt.Sprintf("Spearman correlation: %.3f (p=%.3f)", db.Spearman.Correlation, db.Spearman.PValue))
	}
	if db.WelchsTTest.DegreesFreedom > 0 {
		senseEvidence = append(senseEvidence,
			fmt.Sprintf("Group differences: effect=%.3f (p=%.3f)", db.WelchsTTest.EffectSize, db.WelchsTTest.PValue))
	}
	if len(db.CrossCorrelation.CrossCorrelations) > 0 {
		senseEvidence = append(senseEvidence,
			fmt.Sprintf("Temporal patterns: max r=%.3f at lag %d", db.CrossCorrelation.MaxCorrelation, db.CrossCorrelation.OptimalLag))
	}

	if len(senseEvidence) > 0 {
		prompt.WriteString(strings.Join(senseEvidence, "\n- "))
		prompt.WriteString("\n")
	}

	// Add behavioral narratives
	if db.SilenceAcceleration.Detected {
		prompt.WriteString(fmt.Sprintf("\n## Behavioral Signals\n- Relationship breakdown detected (acceleration: %.2f)\n",
			db.SilenceAcceleration.AccelerationRate))
	}
	if db.BlastRadius.RadiusScore > 0.5 {
		prompt.WriteString(fmt.Sprintf("- High systemic impact potential (%d affected variables)\n",
			len(db.BlastRadius.AffectedVariables)))
	}
	if db.TwinSegments.Detected {
		prompt.WriteString(fmt.Sprintf("- Redundancy detected (%d similar segment pairs)\n",
			len(db.TwinSegments.SegmentPairs)))
	}

	// Generation requirements
	prompt.WriteString(`
## Directive Generation Requirements

Generate research directives that would validate or falsify the patterns observed. Each directive must include:

### Required Structure
- **id**: Unique identifier for this directive
- **claim**: Clear, testable hypothesis statement
- **logic_type**: One of "causal", "associational", "predictive", "descriptive"
- **validation_strategy**: 
  - detector: Primary statistical method to test the claim
  - scanner: Method to check for alternative explanations
  - proxy: Backup validation approach if primary methods fail

### Validation Strategy Guidelines
**Detector Options:**
- anova_detector: For group differences with 3+ categories
- correlation_detector: For linear relationships
- regression_detector: For controlling confounding
- ttest_detector: For binary group comparisons
- chisquare_detector: For categorical associations
- mutual_info_detector: For non-linear relationships
- timeseries_detector: For temporal patterns

**Scanner Options:**
- power_scanner: Check statistical power and sample size adequacy
- robustness_scanner: Test sensitivity to assumptions
- falsification_scanner: Look for contradictory evidence
- subgroup_scanner: Check consistency across subgroups
- sensitivity_scanner: Test parameter variations

**Proxy Options:**
- permutation_proxy: Non-parametric significance testing
- bootstrap_proxy: Resampling-based validation
- simulation_proxy: Model-based validation
- external_proxy: Use external data sources

### Quality Requirements
- **Actionable**: Must specify concrete statistical tests
- **Prioritized**: Include confidence thresholds and sample size requirements
- **Robust**: Consider multiple validation approaches
- **Evidence-Based**: Reference specific statistical patterns

Output only valid JSON array of directive objects with complete validation strategies.`)

	return prompt.String()
}

// GeneratePromptFragments returns prioritized prompt components for injection
func (db *DiscoveryBrief) GeneratePromptFragments() []PromptFragment {
	// Sort fragments by priority (highest first)
	fragments := make([]PromptFragment, len(db.LLMContext.PromptFragments))
	copy(fragments, db.LLMContext.PromptFragments)

	// Simple sort by priority
	for i := 0; i < len(fragments)-1; i++ {
		for j := i + 1; j < len(fragments); j++ {
			if fragments[i].Priority < fragments[j].Priority {
				fragments[i], fragments[j] = fragments[j], fragments[i]
			}
		}
	}

	return fragments
}

// GetHypothesisSeeds returns high-confidence hypothesis starting points
func (db *DiscoveryBrief) GetHypothesisSeeds() []HypothesisSeed {
	var highConfidence []HypothesisSeed

	for _, seed := range db.LLMContext.HypothesisSeeds {
		if seed.Confidence > 0.6 && seed.Priority > 0.6 {
			highConfidence = append(highConfidence, seed)
		}
	}

	return highConfidence
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// CreateVariableRegistry builds a description map from available variables
// This would typically be populated from the variable registry service
func CreateVariableRegistry(variableKeys []core.VariableKey) map[core.VariableKey]string {
	registry := make(map[core.VariableKey]string)

	for _, key := range variableKeys {
		// Default descriptions - in practice, these would come from metadata
		switch {
		case strings.Contains(string(key), "age"):
			registry[key] = "Age of individual"
		case strings.Contains(string(key), "income"):
			registry[key] = "Income amount"
		case strings.Contains(string(key), "score"):
			registry[key] = "Performance or quality score"
		case strings.Contains(string(key), "count"):
			registry[key] = "Frequency or count measure"
		case strings.Contains(string(key), "rate"):
			registry[key] = "Rate or percentage measure"
		case strings.Contains(string(key), "category"):
			registry[key] = "Categorical grouping variable"
		case strings.Contains(string(key), "status"):
			registry[key] = "Status or state indicator"
		default:
			registry[key] = "Variable: " + string(key)
		}
	}

	return registry
}

// ValidateHypothesisPrompt checks if a generated prompt meets basic requirements
func ValidateHypothesisPrompt(prompt string) []string {
	var issues []string

	requiredSections := []string{
		"Statistical Evidence",
		"Behavioral Insights",
		"Generation Requirements",
		"cause_key",
		"mechanism_category",
	}

	for _, section := range requiredSections {
		if !strings.Contains(prompt, section) {
			issues = append(issues, fmt.Sprintf("Missing required section: %s", section))
		}
	}

	if !strings.Contains(prompt, "Output only valid JSON") {
		issues = append(issues, "Missing JSON output requirement")
	}

	return issues
}
