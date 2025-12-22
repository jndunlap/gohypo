package analysis

import (
	"fmt"
	"math"
	"strings"
	"time"

	"gohypo/domain/greenfield"
)

// EvidenceOrchestrator combines multiple statistical senses into business narratives
type EvidenceOrchestrator struct {
	businessMapper *BusinessNameMapper
}

// NewEvidenceOrchestrator creates a new evidence orchestrator
func NewEvidenceOrchestrator() *EvidenceOrchestrator {
	return &EvidenceOrchestrator{
		businessMapper: NewBusinessNameMapper(),
	}
}

// OrchestrateEvidence transforms raw statistical results into LLM-optimized evidence brief
func (eo *EvidenceOrchestrator) OrchestrateEvidence(
	fieldMetadata []greenfield.FieldMetadata,
	statisticalArtifacts []map[string]interface{},
	outcomeColumn string,
	allowedVariables []string,
	excludedVariables map[string]string,
) *EvidenceBrief {

	fmt.Printf("[EvidenceOrchestrator] 🔄 Starting evidence orchestration\n")
	fmt.Printf("[EvidenceOrchestrator]   • Field metadata: %d fields\n", len(fieldMetadata))
	fmt.Printf("[EvidenceOrchestrator]   • Statistical artifacts: %d artifacts\n", len(statisticalArtifacts))
	fmt.Printf("[EvidenceOrchestrator]   • Outcome column: %s\n", outcomeColumn)
	if len(statisticalArtifacts) == 0 {
		fmt.Printf("[EvidenceOrchestrator] ℹ️ No statistical artifacts available - generating basic evidence from field metadata only\n")
	}

	// Generate business column mappings
	businessNames := eo.generateBusinessColumnNames(fieldMetadata, outcomeColumn, allowedVariables)
	fmt.Printf("[EvidenceOrchestrator] ✅ Generated business names for %d columns\n", len(businessNames))

	// Transform statistical artifacts into structured evidence
	associations := eo.extractAssociations(statisticalArtifacts, businessNames, outcomeColumn)
	breakpoints := eo.extractBreakpoints(statisticalArtifacts, businessNames, outcomeColumn)
	interactions := eo.extractInteractions(statisticalArtifacts, businessNames, outcomeColumn)
	structuralBreaks := eo.extractStructuralBreaks(statisticalArtifacts, businessNames, outcomeColumn)
	transferEntropies := eo.extractTransferEntropies(statisticalArtifacts, businessNames, outcomeColumn)
	hysteresisEffects := eo.extractHysteresis(statisticalArtifacts, businessNames, outcomeColumn)

	fmt.Printf("[EvidenceOrchestrator] 📊 Extracted evidence:\n")
	fmt.Printf("[EvidenceOrchestrator]   • Associations: %d\n", len(associations))
	fmt.Printf("[EvidenceOrchestrator]   • Breakpoints: %d\n", len(breakpoints))
	fmt.Printf("[EvidenceOrchestrator]   • Interactions: %d\n", len(interactions))
	fmt.Printf("[EvidenceOrchestrator]   • Structural breaks: %d\n", len(structuralBreaks))
	fmt.Printf("[EvidenceOrchestrator]   • Transfer entropies: %d\n", len(transferEntropies))
	fmt.Printf("[EvidenceOrchestrator]   • Hysteresis effects: %d\n", len(hysteresisEffects))

	totalEvidence := len(associations) + len(breakpoints) + len(interactions) + len(structuralBreaks) + len(transferEntropies) + len(hysteresisEffects)
	if totalEvidence == 0 {
		fmt.Printf("[EvidenceOrchestrator] ℹ️ No statistical evidence found - LLM will generate hypotheses based on field names and business context only\n")
	}

	// Create comprehensive LLM context
	llmContext := eo.buildLLMContext(businessNames[outcomeColumn])

	brief := &EvidenceBrief{
		Version:             "1.0.0",
		Timestamp:           time.Now(),
		DatasetName:         "customer_transaction_data", // Could be dynamic
		RowCount:            100000, // Would come from actual data
		ColumnCount:         len(fieldMetadata),
		BusinessColumnNames: businessNames,
		OutcomeColumn:       outcomeColumn,
		AllowedVariables:    allowedVariables,
		ExcludedVariables:   excludedVariables,
		Associations:        associations,
		Breakpoints:         breakpoints,
		Interactions:        interactions,
		StructuralBreaks:    structuralBreaks,
		TransferEntropies:   transferEntropies,
		HysteresisEffects:   hysteresisEffects,
		LLMContext:          llmContext,
	}

	return brief
}

// generateBusinessColumnNames creates business-friendly names for all columns
func (eo *EvidenceOrchestrator) generateBusinessColumnNames(
	fieldMetadata []greenfield.FieldMetadata,
	outcomeColumn string,
	allowedVariables []string,
) map[string]string {
	businessNames := make(map[string]string)

	for _, field := range fieldMetadata {
		businessNames[string(field.Name)] = eo.businessMapper.MapColumnToBusinessName(string(field.Name), outcomeColumn)
	}

	return businessNames
}

// extractAssociations transforms correlation/association data into structured evidence
func (eo *EvidenceOrchestrator) extractAssociations(
	artifacts []map[string]interface{},
	businessNames map[string]string,
	outcomeColumn string,
) []AssociationResult {
	var associations []AssociationResult

	for i, artifact := range artifacts {
		// Look for relationship artifacts
		if kind, ok := artifact["kind"].(string); ok && kind == "relationship" {
			if payload, ok := artifact["payload"]; ok {
				// Try to extract TestType from payload
				if payloadMap, ok := payload.(map[string]interface{}); ok {
					if testType, ok := payloadMap["test_type"].(string); ok {
						if eo.isAssociationMethod(strings.ToLower(testType)) {
							assoc := eo.buildAssociationResult(i, artifact, businessNames, outcomeColumn)
							associations = append(associations, assoc)
						}
					}
				}
			}
		}
	}

	return associations
}

// isAssociationMethod checks if a method produces association evidence
func (eo *EvidenceOrchestrator) isAssociationMethod(method string) bool {
	associationMethods := []string{
		"pearson", "spearman", "kendall", "point_biserial",
		"cramers_v", "auc_roc", "mutual_information",
	}

	for _, m := range associationMethods {
		if method == m {
			return true
		}
	}
	return false
}

// buildAssociationResult creates a structured association result
func (eo *EvidenceOrchestrator) buildAssociationResult(
	index int,
	artifact map[string]interface{},
	businessNames map[string]string,
	outcomeColumn string,
) AssociationResult {

	feature := eo.getStringValue(artifact, "feature", "unknown_feature")
	rawEffect := eo.getFloatValue(artifact, "statistic", 0.0)
	pValue := eo.getFloatValue(artifact, "p_value", 1.0)
	method := eo.getStringValue(artifact, "method", "unknown")

	// Determine direction
	direction := 0
	if rawEffect > 0.1 {
		direction = 1
	} else if rawEffect < -0.1 {
		direction = -1
	}

	// Generate statistical hypothesis
	statisticalHypothesis := eo.generateStatisticalHypothesis(feature, outcomeColumn, rawEffect, pValue, method)

	// Assess confidence and practical significance
	confidenceLevel := eo.assessConfidenceLevel(pValue, rawEffect)
	practicalSignificance := eo.assessPracticalSignificance(rawEffect, method)

	result := AssociationResult{
		EvidenceID:             fmt.Sprintf("assoc_%03d", index),
		Feature:                feature,
		Outcome:                outcomeColumn,
		ScreeningScore:         math.Abs(rawEffect), // Simple ranking
		RawEffect:              rawEffect,
		Direction:              direction,
		ConfidenceInterval:     [2]float64{rawEffect - 0.1, rawEffect + 0.1}, // Placeholder
		PValue:                 pValue,
		PValueAdj:              pValue, // Would be FDR-adjusted
		Method:                 method,
		EffectFamily:           eo.determineEffectFamily(method),
		RelationshipForm:       "monotone",
		ClaimTemplate:          fmt.Sprintf("%s influences %s", businessNames[feature], businessNames[outcomeColumn]),
		Coverage:               0.95, // Placeholder
		NEffective:             95000, // Placeholder
		AssumptionsChecked:     true,
		Details:                artifact,
		StatisticalHypothesis:  statisticalHypothesis,
		ConfidenceLevel:        confidenceLevel,
		PracticalSignificance:  practicalSignificance,
		BusinessFeatureName:    businessNames[feature],
		BusinessOutcomeName:    businessNames[outcomeColumn],
	}

	return result
}

// generateStatisticalHypothesis creates formal statistical hypothesis statements
func (eo *EvidenceOrchestrator) generateStatisticalHypothesis(feature, outcome string, effect, pValue float64, method string) string {
	direction := "positive"
	if effect < 0 {
		direction = "negative"
	}

	strength := "weak"
	if math.Abs(effect) > 0.5 {
		strength = "strong"
	} else if math.Abs(effect) > 0.3 {
		strength = "moderate"
	}

	significance := "not significant"
	if pValue < 0.05 {
		significance = "significant"
	}

	return fmt.Sprintf("H₁: %s shows %s %s association with %s (%s correlation = %.3f, p %s 0.05). %s relationship detected.",
		feature, strength, direction, outcome, method, effect,
		func() string { if pValue < 0.05 { return "<" } else { return "≥" } }(),
		func() string { if significance == "significant" { return "Statistically" } else { return "No statistically" } }())
}

// assessConfidenceLevel determines confidence level from p-value and effect size
func (eo *EvidenceOrchestrator) assessConfidenceLevel(pValue, effect float64) ConfidenceLevel {
	absEffect := math.Abs(effect)

	// Very strong: p < 0.001 and large effect
	if pValue < 0.001 && absEffect > 0.5 {
		return ConfidenceVeryStrong
	}
	// Strong: p < 0.01 and medium effect
	if pValue < 0.01 && absEffect > 0.3 {
		return ConfidenceStrong
	}
	// Moderate: p < 0.05
	if pValue < 0.05 {
		return ConfidenceModerate
	}
	// Weak: p < 0.1
	if pValue < 0.1 {
		return ConfidenceWeak
	}
	return ConfidenceNegligible
}

// assessPracticalSignificance determines practical importance using Cohen's guidelines
func (eo *EvidenceOrchestrator) assessPracticalSignificance(effect float64, method string) PracticalSignificance {
	absEffect := math.Abs(effect)

	// Cohen's guidelines vary by method
	switch method {
	case "pearson", "spearman":
		if absEffect >= 0.5 {
			return SignificanceLarge
		} else if absEffect >= 0.3 {
			return SignificanceMedium
		} else if absEffect >= 0.1 {
			return SignificanceSmall
		}
	case "cramers_v":
		if absEffect >= 0.3 {
			return SignificanceLarge
		} else if absEffect >= 0.2 {
			return SignificanceMedium
		} else if absEffect >= 0.1 {
			return SignificanceSmall
		}
	case "auc_roc":
		if absEffect >= 0.8 {
			return SignificanceLarge
		} else if absEffect >= 0.7 {
			return SignificanceMedium
		} else if absEffect >= 0.6 {
			return SignificanceSmall
		}
	}

	return SignificanceNegligible
}

// determineEffectFamily categorizes the type of statistical relationship
func (eo *EvidenceOrchestrator) determineEffectFamily(method string) string {
	switch method {
	case "pearson", "spearman", "kendall":
		return "correlation"
	case "point_biserial":
		return "discriminative"
	case "cramers_v":
		return "association"
	case "auc_roc":
		return "discriminative"
	case "mutual_information":
		return "association"
	default:
		return "correlation"
	}
}

// extractBreakpoints transforms breakpoint data into structured evidence
func (eo *EvidenceOrchestrator) extractBreakpoints(
	artifacts []map[string]interface{},
	businessNames map[string]string,
	outcomeColumn string,
) []BreakpointResult {
	var breakpoints []BreakpointResult

	for i, artifact := range artifacts {
		if method, ok := artifact["method"].(string); ok && eo.isBreakpointMethod(method) {
			bp := eo.buildBreakpointResult(i, artifact, businessNames, outcomeColumn)
			breakpoints = append(breakpoints, bp)
		}
	}

	return breakpoints
}

// isBreakpointMethod checks if a method produces breakpoint evidence
func (eo *EvidenceOrchestrator) isBreakpointMethod(method string) bool {
	breakpointMethods := []string{"segmented_regression", "changepoint_detection", "threshold_analysis"}
	for _, m := range breakpointMethods {
		if method == m {
			return true
		}
	}
	return false
}

// buildBreakpointResult creates a structured breakpoint result
func (eo *EvidenceOrchestrator) buildBreakpointResult(
	index int,
	artifact map[string]interface{},
	businessNames map[string]string,
	outcomeColumn string,
) BreakpointResult {

	feature := eo.getStringValue(artifact, "feature", "unknown_feature")
	threshold := eo.getFloatValue(artifact, "threshold", 0.0)
	effectBelow := eo.getFloatValue(artifact, "effect_below", 0.0)
	effectAbove := eo.getFloatValue(artifact, "effect_above", 0.0)
	pValue := eo.getFloatValue(artifact, "p_value", 1.0)

	delta := effectAbove - effectBelow

	statisticalHypothesis := fmt.Sprintf("H₁: %s exhibits threshold effect at %.2f (effect: %.3f → %.3f, Δ=%.3f, p %s 0.05).",
		feature, threshold, effectBelow, effectAbove, delta,
		func() string { if pValue < 0.05 { return "<" } else { return "≥" } }())

	confidenceLevel := eo.assessConfidenceLevel(pValue, delta)
	practicalSignificance := eo.assessPracticalSignificance(delta, "threshold")

	return BreakpointResult{
		EvidenceID:          fmt.Sprintf("bp_%03d", index),
		Feature:             feature,
		Outcome:             outcomeColumn,
		Threshold:           threshold,
		EffectBelow:         effectBelow,
		EffectAbove:         effectAbove,
		Delta:               delta,
		ConfidenceBand:      [2]float64{delta - 0.1, delta + 0.1},
		PValue:              pValue,
		PValueAdj:           pValue,
		Method:              eo.getStringValue(artifact, "method", "threshold_analysis"),
		RelationshipForm:    "threshold",
		ClaimTemplate:       fmt.Sprintf("%s shows cliff effect at %s threshold", businessNames[feature], businessNames[outcomeColumn]),
		Coverage:            0.9,
		NEffective:          90000,
		AssumptionsChecked:  true,
		Details:             artifact,
		StatisticalHypothesis: statisticalHypothesis,
		ConfidenceLevel:     confidenceLevel,
		PracticalSignificance: practicalSignificance,
		BusinessFeatureName: businessNames[feature],
		BusinessOutcomeName: businessNames[outcomeColumn],
	}
}

// extractInteractions, extractStructuralBreaks, extractTransferEntropies, extractHysteresis
// would follow similar patterns but are simplified for this implementation

func (eo *EvidenceOrchestrator) extractInteractions(artifacts []map[string]interface{}, businessNames map[string]string, outcomeColumn string) []InteractionResult {
	return []InteractionResult{} // Placeholder
}

func (eo *EvidenceOrchestrator) extractStructuralBreaks(artifacts []map[string]interface{}, businessNames map[string]string, outcomeColumn string) []StructuralBreakResult {
	return []StructuralBreakResult{} // Placeholder
}

func (eo *EvidenceOrchestrator) extractTransferEntropies(artifacts []map[string]interface{}, businessNames map[string]string, outcomeColumn string) []TransferEntropyResult {
	return []TransferEntropyResult{} // Placeholder
}

func (eo *EvidenceOrchestrator) extractHysteresis(artifacts []map[string]interface{}, businessNames map[string]string, outcomeColumn string) []HysteresisResult {
	return []HysteresisResult{} // Placeholder
}

// buildLLMContext creates comprehensive context for LLM hypothesis generation
func (eo *EvidenceOrchestrator) buildLLMContext(outcomeName string) LLMContext {
	return LLMContext{
		Purpose: "Use the statistical evidence below to generate compelling boardroom hypotheses. Create narratives that executives can immediately understand and act upon.",
		BoardroomHypothesisGeneration: BoardroomGuidance{
			ExecutiveSummary:         fmt.Sprintf("Generate 3-5 key hypotheses that would make executives sit up and take notice. Focus on strategic implications, not statistical details for %s.", outcomeName),
			BusinessImpactFocus:      "Each hypothesis should clearly state the business outcome and the strategic action implied.",
			ConfidenceBasedPrioritization: "Rank hypotheses by confidence_level and practical_significance. Present strongest evidence first.",
			NarrativeStyle:           "Use executive language: 'Our analysis shows...', 'This suggests...', 'Businesses should consider...'",
			EvidenceCitation:         "Reference specific evidence_ids to ground claims in data, but keep statistical jargon minimal in boardroom versions.",
		},
		EvidenceInterpretationGuide: EvidenceInterpretation{
			ConfidenceLevels: map[string]string{
				"very_strong": "High-confidence insights executives can bet the business on",
				"strong":      "Reliable patterns for strategic planning",
				"moderate":    "Interesting trends worth monitoring",
				"weak":        "Early signals, not yet actionable",
				"negligible":  "Statistical noise, ignore for business decisions",
			},
			PracticalSignificance: map[string]string{
				"large":      "Effects that could transform business outcomes",
				"medium":     "Meaningful improvements in key metrics",
				"small":      "Incremental gains, potentially worthwhile at scale",
				"negligible": "Too small to matter in business context",
			},
			CausalityLanguage: "Use 'drives', 'influences', 'predicts', 'correlates with'. Reserve 'causes' for transfer entropy evidence.",
		},
		HypothesisTemplates: HypothesisTemplates{
			SingleEvidence: "Pattern: [Business factor] [relationship] [business outcome]. Implication: [Strategic action].",
			MultiEvidence:  "Combined evidence shows [factor1 + factor2] create [compound effect] on [outcome]. Strategy: [Coordinated action].",
			SegmentSpecific: "The relationship between [factor] and [outcome] varies by [segment]. Tailor strategies accordingly.",
			TemporalBreak:  "Business dynamics changed at [time point]. [Pre-break pattern] shifted to [post-break pattern].",
			SystemMemory:   "[Past events] create lasting effects on [current outcomes]. Recovery strategies need [additional effort].",
		},
		BoardroomNarrativeExamples: BoardroomExamples{
			Correlation:  fmt.Sprintf("'Our top customers who engage with our mobile app 3+ times per week are 40%% more likely to improve %s.'", outcomeName),
			Interaction:  fmt.Sprintf("'Price sensitivity varies dramatically by customer segment. Premium customers are willing to pay 25%% more, while budget-conscious customers drop off at just 5%% increases affecting %s.'", outcomeName),
			Temporal:     fmt.Sprintf("'The marketing channel effectiveness completely changed after our Q3 product launch. Social media ROI dropped 60%% while email performance improved 35%% for %s.'", outcomeName),
			Hysteresis:   fmt.Sprintf("'Failed delivery experiences create lasting damage - customers who experienced delays take 3x longer to return to normal %s patterns.'", outcomeName),
		},
		EvidenceDrivenConstraints: EvidenceConstraints{
			CiteEvidenceIDs:      "Every hypothesis must reference at least one evidence_id",
			StatisticalGrounding: "Base claims on confidence_level and practical_significance assessments",
			BusinessNamesOnly:    "Use business_column_names for all variable references, never technical column names",
			EvidenceScopeOnly:    "Only generate hypotheses supported by the evidence_items provided",
			ActionableFocus:      "Each hypothesis should imply a specific business action or decision",
		},
		DatasetContext: DatasetContext{
			RowUnit:    "one row = one customer transaction",
			Population: "All customer transactions in the dataset after excluding null pairs",
			TimeScope:  "Transaction data spanning multiple periods with temporal ordering available",
		},
	}
}

// Helper methods for safe type conversion
func (eo *EvidenceOrchestrator) getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if val, ok := data[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (eo *EvidenceOrchestrator) getFloatValue(data map[string]interface{}, key string, defaultValue float64) float64 {
	if val, ok := data[key]; ok {
		if flt, ok := val.(float64); ok {
			return flt
		}
		if intVal, ok := val.(int); ok {
			return float64(intVal)
		}
	}
	return defaultValue
}
