package discovery

import (
	"fmt"
	"gohypo/domain/core"
	"gohypo/domain/stats"
)

// ConvertSenseResultsToDiscoveryBrief transforms sense results into DiscoveryBrief structure
// This bridges the gap between statistical senses and LLM-ready context
func ConvertSenseResultsToDiscoveryBrief(
	snapshotID core.SnapshotID,
	runID core.RunID,
	variableKey core.VariableKey,
	senseResults []stats.SenseResult,
) *DiscoveryBrief {
	brief := NewDiscoveryBrief(snapshotID, runID, variableKey)

	// Map sense results to their respective fields
	for _, sense := range senseResults {
		switch sense.SenseName {
		case "mutual_information":
			brief.MutualInformation = extractMutualInformation(sense)
		case "welch_ttest":
			brief.WelchsTTest = extractWelchsTTest(sense)
		case "chi_square":
			brief.ChiSquare = extractChiSquare(sense)
		case "spearman":
			brief.Spearman = extractSpearman(sense)
		case "cross_correlation":
			brief.CrossCorrelation = extractCrossCorrelation(sense)
		}
	}

	// Calculate confidence and risk
	brief.CalculateConfidence()
	brief.RiskAssessment = brief.AssessRisk()

	// Generate warning flags
	brief.WarningFlags = generateWarningFlags(senseResults)

	// Generate LLM context
	brief.LLMContext = generateLLMContext(brief)

	return brief
}

// extractMutualInformation converts sense result to MutualInformationSense
func extractMutualInformation(sense stats.SenseResult) MutualInformationSense {
	mi := MutualInformationSense{
		MIValue:      sense.EffectSize,
		NormalizedMI: sense.EffectSize, // Already normalized in most cases
		PValue:       sense.PValue,
	}

	// Extract metadata if available
	if meta := sense.Metadata; meta != nil {
		if sampleSize, ok := meta["sample_size"].(int); ok {
			mi.SampleSize = sampleSize
		}
		if entropyX, ok := meta["entropy_x"].(float64); ok {
			mi.EntropyX = entropyX
		}
		if entropyY, ok := meta["entropy_y"].(float64); ok {
			mi.EntropyY = entropyY
		}
	}

	return mi
}

// extractWelchsTTest converts sense result to WelchsTTestSense
func extractWelchsTTest(sense stats.SenseResult) WelchsTTestSense {
	ttest := WelchsTTestSense{
		TStatistic: 0, // Will be extracted from metadata
		PValue:     sense.PValue,
		EffectSize: sense.EffectSize, // Cohen's d
	}

	if meta := sense.Metadata; meta != nil {
		if tstat, ok := meta["t_statistic"].(float64); ok {
			ttest.TStatistic = tstat
		}
		if group1Size, ok := meta["group1_size"].(int); ok {
			ttest.Group1Size = group1Size
		}
		if group2Size, ok := meta["group2_size"].(int); ok {
			ttest.Group2Size = group2Size
		}
		if group1Mean, ok := meta["group1_mean"].(float64); ok {
			ttest.Group1Mean = group1Mean
		}
		if group2Mean, ok := meta["group2_mean"].(float64); ok {
			ttest.Group2Mean = group2Mean
		}
	}

	// Calculate sample size from group sizes
	ttest.SampleSize = ttest.Group1Size + ttest.Group2Size

	return ttest
}

// extractChiSquare converts sense result to ChiSquareSense
func extractChiSquare(sense stats.SenseResult) ChiSquareSense {
	chiSquare := ChiSquareSense{
		ChiSquareStatistic:  0, // Will be extracted from metadata
		PValue:              sense.PValue,
		CramersV:            sense.EffectSize, // Cramer's V as effect size
		ExpectedFrequencies: make(map[string]float64),
		ObservedFrequencies: make(map[string]int),
		Residuals:           make(map[string]float64),
	}

	if meta := sense.Metadata; meta != nil {
		if chiStat, ok := meta["chi_square_stat"].(float64); ok {
			chiSquare.ChiSquareStatistic = chiStat
		}
		if df, ok := meta["degrees_freedom"].(int); ok {
			chiSquare.DegreesFreedom = df
		}
	}

	return chiSquare
}

// extractSpearman converts sense result to SpearmanSense
func extractSpearman(sense stats.SenseResult) SpearmanSense {
	spearman := SpearmanSense{
		Correlation: sense.EffectSize, // Spearman's rho
		PValue:      sense.PValue,
	}

	if meta := sense.Metadata; meta != nil {
		if sampleSize, ok := meta["sample_size"].(int); ok {
			spearman.SampleSize = sampleSize
		}
	}

	return spearman
}

// extractCrossCorrelation converts sense result to CrossCorrelationSense
func extractCrossCorrelation(sense stats.SenseResult) CrossCorrelationSense {
	crossCorr := CrossCorrelationSense{
		MaxCorrelation:    sense.EffectSize,
		PValue:            sense.PValue,
		CrossCorrelations: []LagCorrelation{},
	}

	if meta := sense.Metadata; meta != nil {
		if lag, ok := meta["best_lag"].(int); ok {
			crossCorr.OptimalLag = lag
		}
		if direction, ok := meta["direction"].(string); ok {
			crossCorr.Direction = direction
		}
		// Could extract full lag series if available in metadata
	}

	return crossCorr
}

// generateWarningFlags analyzes sense results to identify concerns
func generateWarningFlags(senseResults []stats.SenseResult) []WarningFlag {
	flags := []WarningFlag{}

	for _, sense := range senseResults {
		// Check for low sample size
		if meta := sense.Metadata; meta != nil {
			if sampleSize, ok := meta["sample_size"].(int); ok && sampleSize < 30 {
				flags = append(flags, WarningLowSampleSize)
			}
		}

		// Check for significance only in non-linear sense
		if sense.SenseName == "mutual_information" && sense.PValue < 0.05 {
			// Check if linear tests are not significant
			nonLinearOnly := true
			for _, other := range senseResults {
				if (other.SenseName == "spearman" || other.SenseName == "pearson") && other.PValue < 0.05 {
					nonLinearOnly = false
					break
				}
			}
			if nonLinearOnly {
				flags = append(flags, WarningNonlinearOnly)
			}
		}
	}

	// Remove duplicates
	flagMap := make(map[WarningFlag]bool)
	uniqueFlags := []WarningFlag{}
	for _, flag := range flags {
		if !flagMap[flag] {
			flagMap[flag] = true
			uniqueFlags = append(uniqueFlags, flag)
		}
	}

	return uniqueFlags
}

// generateLLMContext creates LLM-ready context from sense results
func generateLLMContext(brief *DiscoveryBrief) LLMContext {
	ctx := LLMContext{
		BehavioralInsights: []string{},
		HypothesisSeeds:    []HypothesisSeed{},
		PromptFragments:    []PromptFragment{},
		UncertaintyFactors: []string{},
		EvidenceStrength: EvidenceStrength{
			OverallScore:     brief.ConfidenceScore,
			SenseScores:      make(map[string]float64),
			ConsistencyScore: 0.0,
			RobustnessScore:  0.0,
		},
	}

	// Generate executive summary
	ctx.ExecutiveSummary = generateExecutiveSummary(brief)

	// Generate statistical summary
	ctx.StatisticalSummary = generateStatisticalSummary(brief)

	// Generate behavioral insights
	ctx.BehavioralInsights = generateBehavioralInsights(brief)

	// Generate hypothesis seeds
	ctx.HypothesisSeeds = generateHypothesisSeeds(brief)

	// Generate prompt fragments
	ctx.PromptFragments = generatePromptFragments(brief)

	return ctx
}

// generateExecutiveSummary creates a high-level narrative
func generateExecutiveSummary(brief *DiscoveryBrief) string {
	summary := fmt.Sprintf("Analysis of %s reveals ", brief.VariableKey)

	significantSenses := []string{}
	if brief.MutualInformation.PValue < 0.05 {
		significantSenses = append(significantSenses, "non-linear relationships")
	}
	if brief.WelchsTTest.PValue < 0.05 {
		significantSenses = append(significantSenses, "group differences")
	}
	if brief.Spearman.PValue < 0.05 {
		significantSenses = append(significantSenses, "monotonic patterns")
	}
	if brief.CrossCorrelation.PValue < 0.05 {
		significantSenses = append(significantSenses, "temporal dependencies")
	}

	if len(significantSenses) == 0 {
		summary += "no significant patterns detected across statistical senses."
	} else if len(significantSenses) == 1 {
		summary += significantSenses[0] + "."
	} else {
		summary += fmt.Sprintf("multiple patterns including %s.", joinStrings(significantSenses, ", ", " and "))
	}

	return summary
}

// generateStatisticalSummary creates detailed statistical narrative
func generateStatisticalSummary(brief *DiscoveryBrief) string {
	parts := []string{}

	if brief.MutualInformation.SampleSize > 0 {
		parts = append(parts, fmt.Sprintf("Mutual Information: %.3f (p=%.3f)", brief.MutualInformation.MIValue, brief.MutualInformation.PValue))
	}
	if brief.WelchsTTest.SampleSize > 0 {
		parts = append(parts, fmt.Sprintf("Welch's t-test: d=%.3f (p=%.3f)", brief.WelchsTTest.EffectSize, brief.WelchsTTest.PValue))
	}
	if brief.Spearman.SampleSize > 0 {
		parts = append(parts, fmt.Sprintf("Spearman: ρ=%.3f (p=%.3f)", brief.Spearman.Correlation, brief.Spearman.PValue))
	}

	return joinStrings(parts, "; ", "")
}

// generateBehavioralInsights creates actionable insights
func generateBehavioralInsights(brief *DiscoveryBrief) []string {
	insights := []string{}

	// Insight from mutual information
	if brief.MutualInformation.PValue < 0.05 && brief.MutualInformation.MIValue > 0.3 {
		insights = append(insights, "Strong non-linear dependencies detected - consider polynomial or interaction effects")
	}

	// Insight from group differences
	if brief.WelchsTTest.PValue < 0.05 {
		direction := "higher"
		if brief.WelchsTTest.Group1Mean < brief.WelchsTTest.Group2Mean {
			direction = "lower"
		}
		insights = append(insights, fmt.Sprintf("Group 1 shows %s mean (d=%.2f) - investigate segmentation drivers", direction, brief.WelchsTTest.EffectSize))
	}

	// Insight from temporal patterns
	if brief.CrossCorrelation.PValue < 0.05 && brief.CrossCorrelation.OptimalLag != 0 {
		insights = append(insights, fmt.Sprintf("Temporal lag of %d periods detected - consider causal ordering", brief.CrossCorrelation.OptimalLag))
	}

	return insights
}

// generateHypothesisSeeds creates starting points for hypothesis generation
func generateHypothesisSeeds(brief *DiscoveryBrief) []HypothesisSeed {
	seeds := []HypothesisSeed{}

	// Seed from non-linear relationships
	if brief.MutualInformation.PValue < 0.05 {
		seeds = append(seeds, HypothesisSeed{
			Category:    "non_linear",
			Description: "Non-linear relationship suggests threshold effects or interaction terms",
			Priority:    0.8,
			Confidence:  1.0 - brief.MutualInformation.PValue,
		})
	}

	// Seed from group differences
	if brief.WelchsTTest.PValue < 0.05 {
		seeds = append(seeds, HypothesisSeed{
			Category:    "segmentation",
			Description: "Significant group differences indicate segmentation opportunity",
			Priority:    0.7,
			Confidence:  1.0 - brief.WelchsTTest.PValue,
		})
	}

	// Seed from temporal patterns
	if brief.CrossCorrelation.PValue < 0.05 {
		seeds = append(seeds, HypothesisSeed{
			Category:    "temporal_causal",
			Description: "Lagged correlation suggests potential causal mechanism",
			Priority:    0.9,
			Confidence:  1.0 - brief.CrossCorrelation.PValue,
		})
	}

	return seeds
}

// generatePromptFragments creates prompt components for LLM
func generatePromptFragments(brief *DiscoveryBrief) []PromptFragment {
	fragments := []PromptFragment{}

	// Add statistical context fragment
	fragments = append(fragments, PromptFragment{
		Type:     "statistical",
		Content:  generateStatisticalSummary(brief),
		Priority: 10,
	})

	// Add behavioral context fragment
	insights := generateBehavioralInsights(brief)
	if len(insights) > 0 {
		fragments = append(fragments, PromptFragment{
			Type:     "behavioral",
			Content:  joinStrings(insights, ". ", "."),
			Priority: 9,
		})
	}

	return fragments
}

// joinStrings joins strings with delimiter and special last delimiter
func joinStrings(parts []string, delimiter, lastDelimiter string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 {
		return parts[0] + lastDelimiter + parts[1]
	}

	result := ""
	for i := 0; i < len(parts)-1; i++ {
		result += parts[i] + delimiter
	}
	result += lastDelimiter + parts[len(parts)-1]
	return result
}


