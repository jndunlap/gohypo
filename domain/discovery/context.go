package discovery

import (
	"fmt"
	"math"
	"strings"

	"gohypo/domain/core"
	"gohypo/domain/stats"
)

// ============================================================================
// LLM CONTEXT INJECTION METHODS
// ============================================================================

// GenerateLLMContext creates rich context summaries for LLM consumption
func (db *DiscoveryBrief) GenerateLLMContext() {
	db.LLMContext = LLMContext{
		BehavioralInsights: []string{},
		HypothesisSeeds:    []HypothesisSeed{},
		PromptFragments:    []PromptFragment{},
		UncertaintyFactors: []string{},
	}

	// Generate human-readable summaries
	db.generateExecutiveSummary()
	db.generateStatisticalSummary()

	// Generate behavioral insights
	db.generateBehavioralInsights()

	// Generate hypothesis seeds
	db.generateHypothesisSeeds()

	// Generate prompt fragments
	db.generatePromptFragments()

	// Identify uncertainty factors
	db.identifyUncertaintyFactors()

	// Calculate evidence strength
	db.calculateEvidenceStrength()
}

// generateExecutiveSummary creates a concise overview for decision makers
func (db *DiscoveryBrief) generateExecutiveSummary() {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("Discovery Brief for %s: ", db.VariableKey))

	// Primary statistical signal
	primarySense := db.getPrimarySense()
	if primarySense != "" {
		summary.WriteString(fmt.Sprintf("Strongest signal from %s ", primarySense))
	}

	// Confidence and risk
	confidence := "low"
	if db.ConfidenceScore > 0.7 {
		confidence = "high"
	} else if db.ConfidenceScore > 0.4 {
		confidence = "medium"
	}

	risk := "low"
	switch db.RiskAssessment {
	case RiskMedium:
		risk = "medium"
	case RiskHigh:
		risk = "high"
	}

	summary.WriteString(fmt.Sprintf("(confidence: %s, risk: %s). ", confidence, risk))

	// Key behavioral insights
	if db.SilenceAcceleration.Detected {
		summary.WriteString("Recent relationship breakdown detected. ")
	}
	if db.BlastRadius.RadiusScore > 0.5 {
		summary.WriteString("High systemic impact potential. ")
	}
	if db.TwinSegments.Detected {
		summary.WriteString("Similar behavioral patterns identified. ")
	}

	db.LLMContext.ExecutiveSummary = summary.String()
}

// generateStatisticalSummary creates detailed statistical overview
func (db *DiscoveryBrief) generateStatisticalSummary() {
	var summary strings.Builder

	summary.WriteString("Statistical Profile:\n")

	// Mutual Information
	if db.MutualInformation.SampleSize > 0 {
		summary.WriteString(fmt.Sprintf("- Mutual Information: %.3f (normalized: %.3f)",
			db.MutualInformation.MIValue, db.MutualInformation.NormalizedMI))
		if db.MutualInformation.PValue < 0.05 {
			summary.WriteString(" ✓")
		}
		summary.WriteString("\n")
	}

	// Welch's t-Test
	if db.WelchsTTest.DegreesFreedom > 0 {
		summary.WriteString(fmt.Sprintf("- Group Differences: t=%.3f, effect=%.3f",
			db.WelchsTTest.TStatistic, db.WelchsTTest.EffectSize))
		if db.WelchsTTest.PValue < 0.05 {
			summary.WriteString(" ✓")
		}
		summary.WriteString("\n")
	}

	// Chi-Square
	if db.ChiSquare.DegreesFreedom > 0 {
		summary.WriteString(fmt.Sprintf("- Categorical Patterns: χ²=%.3f, Cramér's V=%.3f",
			db.ChiSquare.ChiSquareStatistic, db.ChiSquare.CramersV))
		if db.ChiSquare.PValue < 0.05 {
			summary.WriteString(" ✓")
		}
		summary.WriteString("\n")
	}

	// Spearman
	if db.Spearman.SampleSize > 0 {
		summary.WriteString(fmt.Sprintf("- Rank Correlation: ρ=%.3f",
			db.Spearman.Correlation))
		if db.Spearman.PValue < 0.05 {
			summary.WriteString(" ✓")
		}
		summary.WriteString("\n")
	}

	// Cross-correlation
	if len(db.CrossCorrelation.CrossCorrelations) > 0 {
		summary.WriteString(fmt.Sprintf("- Temporal Dependencies: max r=%.3f at lag=%d",
			db.CrossCorrelation.MaxCorrelation, db.CrossCorrelation.OptimalLag))
		if db.CrossCorrelation.PValue < 0.05 {
			summary.WriteString(" ✓")
		}
		summary.WriteString("\n")
	}

	db.LLMContext.StatisticalSummary = summary.String()
}

// generateBehavioralInsights extracts human-interpretable patterns
func (db *DiscoveryBrief) generateBehavioralInsights() {
	var insights []string

	// Silence acceleration insights
	if db.SilenceAcceleration.Detected {
		insights = append(insights,
			fmt.Sprintf("Relationship breakdown: %.1f%% acceleration rate over %d periods",
				db.SilenceAcceleration.AccelerationRate*100, db.SilenceAcceleration.SilencePeriod))
	}

	// Blast radius insights
	if db.BlastRadius.RadiusScore > 0.3 {
		impactLevel := "moderate"
		if db.BlastRadius.RadiusScore > 0.7 {
			impactLevel = "high"
		}
		insights = append(insights,
			fmt.Sprintf("%s systemic impact: affects %d variables with centrality score %.2f",
				impactLevel, len(db.BlastRadius.AffectedVariables), db.BlastRadius.CentralityScore))

		if db.BlastRadius.DominoEffect {
			insights = append(insights, "Cascading effects detected - changes may propagate through the system")
		}
	}

	// Twin segments insights
	if db.TwinSegments.Detected {
		insights = append(insights,
			fmt.Sprintf("Behavioral redundancy: %d similar segment pairs with %.1f%% average similarity",
				len(db.TwinSegments.SegmentPairs), db.TwinSegments.SimilarityScore*100))

		if db.TwinSegments.ConfoundingRisk > 0.5 {
			insights = append(insights, "High confounding risk - similar segments may mask true relationships")
		}
	}

	// Cross-sense insights
	insights = append(insights, db.generateCrossSenseInsights()...)

	db.LLMContext.BehavioralInsights = insights
}

// generateCrossSenseInsights finds patterns across multiple statistical senses
func (db *DiscoveryBrief) generateCrossSenseInsights() []string {
	var insights []string

	// Check for consistency across senses
	senseCount := 0
	significantCount := 0

	if db.MutualInformation.PValue < 0.05 {
		senseCount++
		significantCount++
	}
	if db.WelchsTTest.PValue < 0.05 {
		senseCount++
		significantCount++
	}
	if db.ChiSquare.PValue < 0.05 {
		senseCount++
		significantCount++
	}
	if db.Spearman.PValue < 0.05 {
		senseCount++
		significantCount++
	}
	if db.CrossCorrelation.PValue < 0.05 {
		senseCount++
		significantCount++
	}

	if senseCount >= 3 {
		consistency := float64(significantCount) / float64(senseCount)
		if consistency > 0.8 {
			insights = append(insights, "High cross-sense consistency - multiple statistical methods agree")
		} else if consistency < 0.4 {
			insights = append(insights, "Low cross-sense consistency - results vary by statistical method")
		}
	}

	// Check for non-linear vs linear patterns
	if db.MutualInformation.NormalizedMI > 0.3 && math.Abs(db.Spearman.Correlation) < 0.3 {
		insights = append(insights, "Non-linear relationships detected - linear methods may miss important patterns")
	}

	// Check for temporal patterns
	if math.Abs(db.CrossCorrelation.MaxCorrelation) > 0.5 {
		direction := "simultaneous"
		if db.CrossCorrelation.OptimalLag > 0 {
			direction = "lagged"
		}
		insights = append(insights, fmt.Sprintf("Strong temporal patterns: %s relationships with %d period lag",
			direction, db.CrossCorrelation.OptimalLag))
	}

	return insights
}

// generateHypothesisSeeds creates creative starting points for LLM hypothesis generation
func (db *DiscoveryBrief) generateHypothesisSeeds() {
	var seeds []HypothesisSeed

	// Causal hypothesis seeds
	if db.CrossCorrelation.OptimalLag != 0 {
		seeds = append(seeds, HypothesisSeed{
			Category: "causal",
			Description: fmt.Sprintf("Temporal precedence: %s may cause changes in related variables with %d period lag",
				db.VariableKey, db.CrossCorrelation.OptimalLag),
			Priority:   0.9,
			Confidence: db.CrossCorrelation.MaxCorrelation,
		})
	}

	// Effect modification seeds
	if db.BlastRadius.RadiusScore > 0.5 {
		seeds = append(seeds, HypothesisSeed{
			Category: "effect_modification",
			Description: fmt.Sprintf("Systemic influence: %s changes may modify relationships throughout the system",
				db.VariableKey),
			Priority:   0.8,
			Confidence: db.BlastRadius.RadiusScore,
		})
	}

	// Confounding seeds
	if db.TwinSegments.ConfoundingRisk > 0.6 {
		seeds = append(seeds, HypothesisSeed{
			Category:    "confounding",
			Description: fmt.Sprintf("Hidden confounding: similar behavioral patterns may indicate unmeasured common causes"),
			Priority:    0.7,
			Confidence:  db.TwinSegments.ConfoundingRisk,
		})
	}

	// Non-linear relationship seeds
	if db.MutualInformation.NormalizedMI > db.Spearman.Correlation {
		seeds = append(seeds, HypothesisSeed{
			Category: "causal",
			Description: fmt.Sprintf("Non-linear causation: %s may have threshold or interaction effects not captured by linear models",
				db.VariableKey),
			Priority:   0.6,
			Confidence: db.MutualInformation.NormalizedMI,
		})
	}

	// Group difference seeds
	if db.WelchsTTest.EffectSize > 0.5 {
		seeds = append(seeds, HypothesisSeed{
			Category: "effect_modification",
			Description: fmt.Sprintf("Group heterogeneity: %s behavior differs substantially between segments, suggesting moderating factors",
				db.VariableKey),
			Priority:   0.7,
			Confidence: min(db.WelchsTTest.EffectSize/2.0, 1.0),
		})
	}

	db.LLMContext.HypothesisSeeds = seeds
}

// generatePromptFragments creates reusable prompt components
func (db *DiscoveryBrief) generatePromptFragments() {
	var fragments []PromptFragment

	// Statistical context fragment
	statFragment := PromptFragment{
		Type:     "statistical",
		Priority: 10,
		Content:  db.LLMContext.StatisticalSummary,
	}
	fragments = append(fragments, statFragment)

	// Behavioral context fragment
	if len(db.LLMContext.BehavioralInsights) > 0 {
		behavioralContent := "Key Behavioral Patterns:\n" + strings.Join(db.LLMContext.BehavioralInsights, "\n")
		behavioralFragment := PromptFragment{
			Type:     "behavioral",
			Priority: 9,
			Content:  behavioralContent,
		}
		fragments = append(fragments, behavioralFragment)
	}

	// Uncertainty fragment
	if len(db.WarningFlags) > 0 {
		warnings := make([]string, len(db.WarningFlags))
		for i, flag := range db.WarningFlags {
			warnings[i] = string(flag)
		}
		uncertaintyContent := fmt.Sprintf("Important Caveats: %s", strings.Join(warnings, ", "))
		uncertaintyFragment := PromptFragment{
			Type:     "statistical",
			Priority: 8,
			Content:  uncertaintyContent,
		}
		fragments = append(fragments, uncertaintyFragment)
	}

	// Hypothesis guidance fragment
	if len(db.LLMContext.HypothesisSeeds) > 0 {
		guidanceContent := "Hypothesis Generation Guidance:\n"
		for _, seed := range db.LLMContext.HypothesisSeeds {
			if seed.Priority > 0.7 {
				guidanceContent += fmt.Sprintf("- %s\n", seed.Description)
			}
		}
		guidanceFragment := PromptFragment{
			Type:        "domain",
			Priority:    7,
			Content:     guidanceContent,
			Conditional: "when generating hypotheses",
		}
		fragments = append(fragments, guidanceFragment)
	}

	db.LLMContext.PromptFragments = fragments
}

// identifyUncertaintyFactors lists factors that reduce confidence
func (db *DiscoveryBrief) identifyUncertaintyFactors() {
	var factors []string

	// Sample size issues
	if db.MutualInformation.SampleSize < 30 || db.Spearman.SampleSize < 30 {
		factors = append(factors, "Small sample size limits statistical power")
	}

	// Missing data
	// Note: This would need to be populated from actual data quality metrics

	// Warning flags
	for _, flag := range db.WarningFlags {
		switch flag {
		case WarningLowSampleSize:
			factors = append(factors, "Insufficient sample size for reliable estimates")
		case WarningHighMissingData:
			factors = append(factors, "High missing data rates may bias results")
		case WarningOutlierSensitivity:
			factors = append(factors, "Results may be sensitive to outlier values")
		case WarningNonlinearOnly:
			factors = append(factors, "Relationships appear non-linear, linear methods inadequate")
		case WarningTemporalInstability:
			factors = append(factors, "Relationships show temporal instability")
		case WarningConfoundingSuspected:
			factors = append(factors, "Potential confounding variables not accounted for")
		}
	}

	// Cross-sense inconsistency
	senseScores := []float64{
		db.calculateMIScore(),
		db.calculateTTestScore(),
		db.calculateChiSquareScore(),
		db.calculateSpearmanScore(),
		db.calculateCrossCorrScore(),
	}

	validScores := 0
	totalScore := 0.0
	for _, score := range senseScores {
		if score >= 0 {
			validScores++
			totalScore += score
		}
	}

	if validScores >= 2 {
		avgScore := totalScore / float64(validScores)
		if avgScore < 0.5 {
			factors = append(factors, "Multiple statistical senses show weak or inconsistent signals")
		}
	}

	db.LLMContext.UncertaintyFactors = factors
}

// calculateEvidenceStrength computes comprehensive evidence metrics
func (db *DiscoveryBrief) calculateEvidenceStrength() {
	senseScores := map[string]float64{
		"mutual_information": db.calculateMIScore(),
		"welchs_t_test":      db.calculateTTestScore(),
		"chi_square":         db.calculateChiSquareScore(),
		"spearman":           db.calculateSpearmanScore(),
		"cross_correlation":  db.calculateCrossCorrScore(),
	}

	// Calculate consistency (agreement between senses)
	validScores := []float64{}
	for _, score := range senseScores {
		if score >= 0 {
			validScores = append(validScores, score)
		}
	}

	consistencyScore := 0.0
	if len(validScores) > 1 {
		meanScore := mean(validScores)
		variance := 0.0
		for _, score := range validScores {
			variance += math.Pow(score-meanScore, 2)
		}
		variance /= float64(len(validScores))
		consistencyScore = 1.0 - math.Sqrt(variance) // Lower variance = higher consistency
	}

	// Calculate robustness (resistance to assumptions)
	robustnessScore := 0.0
	robustnessCount := 0

	if db.Spearman.PValue < 0.05 { // Robust to outliers
		robustnessScore += 1.0
		robustnessCount++
	}
	if db.MutualInformation.PValue < 0.05 { // Captures non-linear
		robustnessScore += 1.0
		robustnessCount++
	}
	if len(db.CrossCorrelation.CrossCorrelations) > 0 { // Temporal awareness
		robustnessScore += 1.0
		robustnessCount++
	}

	if robustnessCount > 0 {
		robustnessScore /= float64(robustnessCount)
	}

	// Overall score
	overallScore := db.ConfidenceScore*0.4 + consistencyScore*0.3 + robustnessScore*0.3

	db.LLMContext.EvidenceStrength = EvidenceStrength{
		OverallScore:     overallScore,
		SenseScores:      senseScores,
		ConsistencyScore: consistencyScore,
		RobustnessScore:  robustnessScore,
	}
}

// getPrimarySense returns the name of the strongest statistical sense
func (db *DiscoveryBrief) getPrimarySense() string {
	senses := map[string]float64{
		"mutual information":   db.calculateMIScore(),
		"group differences":    db.calculateTTestScore(),
		"categorical patterns": db.calculateChiSquareScore(),
		"rank correlation":     db.calculateSpearmanScore(),
		"temporal patterns":    db.calculateCrossCorrScore(),
	}

	maxScore := -1.0
	primarySense := ""

	for sense, score := range senses {
		if score > maxScore {
			maxScore = score
			primarySense = sense
		}
	}

	return primarySense
}

// ============================================================================
// BUILDERS (Bridge: stats artifacts -> DiscoveryBrief -> LLM-ready context)
// ============================================================================

// BuildDiscoveryBriefsFromRelationships creates per-variable DiscoveryBriefs from relationship payloads and sense results.
//
// This bridges statistical analysis with LLM reasoning by populating all five senses and generating
// behavioral narratives and hypothesis seeds for rich context injection.
func BuildDiscoveryBriefsFromRelationships(
	snapshotID core.SnapshotID,
	runID core.RunID,
	relationships []stats.RelationshipPayload,
	senseResults [][]stats.SenseResult,
) []DiscoveryBrief {
	if len(relationships) == 0 {
		return nil
	}

	// If no sense results provided, create empty slice to avoid nil pointer issues
	if senseResults == nil {
		senseResults = make([][]stats.SenseResult, len(relationships))
	}

	// Convert payloads into RelationshipArtifact for narrative detectors.
	relArts := make([]stats.RelationshipArtifact, 0, len(relationships))
	varSet := make(map[core.VariableKey]struct{})
	for _, p := range relationships {
		key := stats.RelationshipKey{
			VariableX:  p.VariableX,
			VariableY:  p.VariableY,
			TestType:   p.TestType,
			TestParams: p.TestParams,
			FamilyID:   p.FamilyID,
		}
		metrics := stats.CanonicalMetrics{
			EffectSize:       p.EffectSize,
			EffectUnit:       p.EffectUnit,
			PValue:           p.PValue,
			QValue:           p.QValue,
			SampleSize:       p.SampleSize,
			TotalComparisons: p.TotalComparisons,
			FDRMethod:        p.FDRMethod,
		}

		// If invariants fail (e.g. SampleSize=0), skip the artifact for narrative computation.
		a, err := stats.NewRelationshipArtifact(key, metrics)
		if err != nil {
			continue
		}
		a.DiscoveredAt = p.DiscoveredAt
		a.Fingerprint = p.Fingerprint
		a.OverallWarnings = p.Warnings
		relArts = append(relArts, *a)

		varSet[p.VariableX] = struct{}{}
		varSet[p.VariableY] = struct{}{}
	}

	if len(relArts) == 0 {
		return nil
	}

	// Build one brief per variable, using the strongest relationship as the "anchor" for sense fields.
	briefs := make([]DiscoveryBrief, 0, len(varSet))
	for v := range varSet {
		db := NewDiscoveryBrief(snapshotID, runID, v)

		// Behavioral narrative seeds.
		db.SilenceAcceleration = DetectSilenceAcceleration(relArts, v, 5)
		db.BlastRadius = DetectBlastRadius(relArts, v, 0.10)
		db.TwinSegments = DetectTwinSegments(relArts, 0.85)

		// Populate all five statistical senses from sense results
		db.populateSensesFromResults(relArts, senseResults, relationships, v)

		// Warning flags derived from relationship warnings.
		db.WarningFlags = warningFlagsFromRelationshipWarnings(relArts, v)

		// Confidence + risk + LLM context.
		db.CalculateConfidence()
		db.RiskAssessment = db.AssessRisk()
		db.GenerateLLMContext()

		briefs = append(briefs, *db)
	}

	return briefs
}

func findStrongestRelationship(rels []stats.RelationshipArtifact, v core.VariableKey) *stats.RelationshipArtifact {
	var best *stats.RelationshipArtifact
	bestScore := -1.0
	for i := range rels {
		r := &rels[i]
		if r.Key.VariableX != v && r.Key.VariableY != v {
			continue
		}
		// Composite score: significance * magnitude (bounded).
		sig := 1.0 - clamp01(r.Metrics.PValue)
		mag := math.Abs(r.Metrics.EffectSize)
		if mag > 1.0 {
			mag = 1.0
		}
		score := sig*0.6 + mag*0.4
		if score > bestScore {
			bestScore = score
			best = r
		}
	}
	return best
}

func warningFlagsFromRelationshipWarnings(rels []stats.RelationshipArtifact, v core.VariableKey) []WarningFlag {
	flags := make(map[WarningFlag]struct{})
	for _, r := range rels {
		if r.Key.VariableX != v && r.Key.VariableY != v {
			continue
		}
		for _, w := range r.OverallWarnings {
			switch w {
			case stats.WarningLowN:
				flags[WarningLowSampleSize] = struct{}{}
			case stats.WarningHighMissing:
				flags[WarningHighMissingData] = struct{}{}
			case stats.WarningPerfectCorrelation:
				flags[WarningOutlierSensitivity] = struct{}{}
			}
		}
	}
	out := make([]WarningFlag, 0, len(flags))
	for f := range flags {
		out = append(out, f)
	}
	return out
}

// populateSensesFromResults populates all five statistical senses from SenseResult data
func (db *DiscoveryBrief) populateSensesFromResults(
	relArts []stats.RelationshipArtifact,
	senseResults [][]stats.SenseResult,
	relationships []stats.RelationshipPayload,
	variableKey core.VariableKey,
) {
	// Find the relationship index for this variable to get corresponding sense results
	relIndex := -1
	for i, rel := range relationships {
		if rel.VariableX == variableKey || rel.VariableY == variableKey {
			relIndex = i
			break
		}
	}

	if relIndex == -1 || relIndex >= len(senseResults) {
		// Fallback: use strongest relationship for basic Spearman sense
		best := findStrongestRelationship(relArts, variableKey)
		if best != nil {
			db.Spearman = SpearmanSense{
				Correlation: best.Metrics.EffectSize,
				PValue:      best.Metrics.PValue,
				SampleSize:  best.Metrics.SampleSize,
			}
		}
		return
	}

	// Populate each sense from the results
	results := senseResults[relIndex]
	for _, result := range results {
		switch result.SenseName {
		case "mutual_information":
			db.MutualInformation = MutualInformationSense{
				MIValue:       result.EffectSize,
				NormalizedMI:  normalizeMI(result.EffectSize),
				PValue:        result.PValue,
				SampleSize:    getSampleSizeFromMetadata(result.Metadata),
				EntropyX:      getFloatFromMetadata(result.Metadata, "entropy_x"),
				EntropyY:      getFloatFromMetadata(result.Metadata, "entropy_y"),
				ConditionalMI: getFloatFromMetadata(result.Metadata, "conditional_mi"),
			}

		case "welch_ttest":
			db.WelchsTTest = WelchsTTestSense{
				TStatistic:      getFloatFromMetadata(result.Metadata, "t_statistic"),
				DegreesFreedom:  getFloatFromMetadata(result.Metadata, "degrees_freedom"),
				PValue:          result.PValue,
				EffectSize:      result.EffectSize, // Cohen's d
				Group1Mean:      getFloatFromMetadata(result.Metadata, "group_1_mean"),
				Group1Size:      getIntFromMetadata(result.Metadata, "group_1_size"),
				Group2Mean:      getFloatFromMetadata(result.Metadata, "group_2_mean"),
				Group2Size:      getIntFromMetadata(result.Metadata, "group_2_size"),
				SampleSize:      getIntFromMetadata(result.Metadata, "sample_size"),
				Heteroscedastic: getBoolFromMetadata(result.Metadata, "heteroscedastic"),
			}

		case "chi_square":
			db.ChiSquare = ChiSquareSense{
				ChiSquareStatistic:  getFloatFromMetadata(result.Metadata, "chi_square_statistic"),
				DegreesFreedom:      getIntFromMetadata(result.Metadata, "degrees_freedom"),
				PValue:              result.PValue,
				CramersV:            result.EffectSize, // Cramer's V
				ExpectedFrequencies: getStringFloatMapFromMetadata(result.Metadata, "expected_frequencies"),
				ObservedFrequencies: getStringIntMapFromMetadata(result.Metadata, "observed_frequencies"),
				Residuals:           getStringFloatMapFromMetadata(result.Metadata, "residuals"),
			}

		case "spearman":
			db.Spearman = SpearmanSense{
				Correlation:     result.EffectSize,
				PValue:          result.PValue,
				SampleSize:      getIntFromMetadata(result.Metadata, "sample_size"),
				ConcordantPairs: getIntFromMetadata(result.Metadata, "concordant_pairs"),
				DiscordantPairs: getIntFromMetadata(result.Metadata, "discordant_pairs"),
				TiesX:           getIntFromMetadata(result.Metadata, "ties_x"),
				TiesY:           getIntFromMetadata(result.Metadata, "ties_y"),
			}

		case "cross_correlation":
			db.CrossCorrelation = CrossCorrelationSense{
				MaxCorrelation:    result.EffectSize,
				OptimalLag:        getIntFromMetadata(result.Metadata, "optimal_lag"),
				LagRange:          getIntFromMetadata(result.Metadata, "lag_range"),
				PValue:            result.PValue,
				Direction:         getStringFromMetadata(result.Metadata, "direction"),
				CrossCorrelations: getLagCorrelationsFromMetadata(result.Metadata, "cross_correlations"),
			}
		}
	}
}

// Helper functions for extracting metadata
func getSampleSizeFromMetadata(metadata map[string]interface{}) int {
	if sampleSize, ok := metadata["sample_size"].(float64); ok {
		return int(sampleSize)
	}
	return 0
}

func getFloatFromMetadata(metadata map[string]interface{}, key string) float64 {
	if val, ok := metadata[key].(float64); ok {
		return val
	}
	return 0.0
}

func getIntFromMetadata(metadata map[string]interface{}, key string) int {
	if val, ok := metadata[key].(float64); ok {
		return int(val)
	}
	return 0
}

func getBoolFromMetadata(metadata map[string]interface{}, key string) bool {
	if val, ok := metadata[key].(bool); ok {
		return val
	}
	return false
}

func getStringFromMetadata(metadata map[string]interface{}, key string) string {
	if val, ok := metadata[key].(string); ok {
		return val
	}
	return ""
}

func getStringFloatMapFromMetadata(metadata map[string]interface{}, key string) map[string]float64 {
	result := make(map[string]float64)
	if data, ok := metadata[key].(map[string]interface{}); ok {
		for k, v := range data {
			if val, ok := v.(float64); ok {
				result[k] = val
			}
		}
	}
	return result
}

func getStringIntMapFromMetadata(metadata map[string]interface{}, key string) map[string]int {
	result := make(map[string]int)
	if data, ok := metadata[key].(map[string]interface{}); ok {
		for k, v := range data {
			if val, ok := v.(float64); ok {
				result[k] = int(val)
			}
		}
	}
	return result
}

func getLagCorrelationsFromMetadata(metadata map[string]interface{}, key string) []LagCorrelation {
	var correlations []LagCorrelation
	if data, ok := metadata[key].([]interface{}); ok {
		for _, item := range data {
			if itemMap, ok := item.(map[string]interface{}); ok {
				corr := LagCorrelation{
					Lag:         getIntFromMetadata(itemMap, "lag"),
					Correlation: getFloatFromMetadata(itemMap, "correlation"),
					PValue:      getFloatFromMetadata(itemMap, "p_value"),
				}
				correlations = append(correlations, corr)
			}
		}
	}
	return correlations
}

// normalizeMI normalizes mutual information to 0-1 scale (rough approximation)
func normalizeMI(mi float64) float64 {
	// MI is typically bounded by min(H(X), H(Y)), but we use a simple scaling
	// In practice, MI values are usually < 1 for most datasets
	return math.Min(mi, 1.0)
}

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}
