package research

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"gohypo/domain/core"
	"gohypo/ports"
)

// WorkspaceAssembler assembles workspace state for iterative generation
type WorkspaceAssembler struct {
	ledgerPort    ports.LedgerPort
	datasetRepo   ports.DatasetRepository
	workspaceRepo ports.WorkspaceRepository
	forensicScout interface{} // AI service for context
}

// NewWorkspaceAssembler creates the assembler service
func NewWorkspaceAssembler(
	ledgerPort ports.LedgerPort,
	datasetRepo ports.DatasetRepository,
	workspaceRepo ports.WorkspaceRepository,
	forensicScout interface{},
) *WorkspaceAssembler {
	return &WorkspaceAssembler{
		ledgerPort:    ledgerPort,
		datasetRepo:   datasetRepo,
		workspaceRepo: workspaceRepo,
		forensicScout: forensicScout,
	}
}

// AssembleFullContext builds complete workspace context for hypothesis generation
func (wa *WorkspaceAssembler) AssembleFullContext(ctx context.Context, workspaceID string) (*ports.WorkspaceContext, error) {
	// Gather all components concurrently
	validatedChan := make(chan []ports.ValidatedHypothesisSummary, 1)
	rejectedChan := make(chan []ports.RejectedHypothesisSummary, 1)
	datasetChan := make(chan []ports.DatasetSummary, 1)
	forensicChan := make(chan ports.ForensicSummary, 1)

	// Launch concurrent gathering
	go func() {
		vh, _ := wa.gatherValidatedHypotheses(ctx, workspaceID)
		validatedChan <- vh
	}()
	go func() {
		rh, _ := wa.gatherRejectedHypotheses(ctx, workspaceID)
		rejectedChan <- rh
	}()
	go func() {
		ds, _ := wa.summarizeDatasets(ctx, workspaceID)
		datasetChan <- ds
	}()
	go func() {
		fs, _ := wa.extractForensicContext(ctx, workspaceID)
		forensicChan <- fs
	}()

	// Collect results
	validated := <-validatedChan
	rejected := <-rejectedChan
	datasets := <-datasetChan
	forensic := <-forensicChan

	trajectory := wa.analyzeResearchTrajectory(validated, rejected)
	temporal := wa.assessTemporalCoverage(datasets)

	return &ports.WorkspaceContext{
		ValidatedHypotheses: validated,
		RejectedHypotheses:  rejected,
		DatasetSummaries:    datasets,
		ForensicContext:     forensic,
		ResearchTrajectory:  trajectory,
		TemporalCoverage:    temporal,
	}, nil
}

// gatherValidatedHypotheses extracts successfully validated hypotheses
func (wa *WorkspaceAssembler) gatherValidatedHypotheses(ctx context.Context, workspaceID string) ([]ports.ValidatedHypothesisSummary, error) {
	kind := core.ArtifactHypothesis
	artifacts, err := wa.ledgerPort.ListArtifacts(ctx, ports.ArtifactFilters{
		Kind:  &kind,
		Limit: 100, // Most recent 100
	})
	if err != nil {
		return nil, err
	}

	var validated []ports.ValidatedHypothesisSummary
	for _, artifact := range artifacts {
		if vh := wa.extractValidatedHypothesis(artifact); vh != nil {
			validated = append(validated, *vh)
		}
	}

	// Sort by validation strength (E-value)
	sort.Slice(validated, func(i, j int) bool {
		return validated[i].EValue > validated[j].EValue
	})

	return validated, nil
}

// extractValidatedHypothesis parses hypothesis artifact into structured form
func (wa *WorkspaceAssembler) extractValidatedHypothesis(artifact core.Artifact) *ports.ValidatedHypothesisSummary {
	payload, ok := artifact.Payload.(map[string]interface{})
	if !ok {
		return nil
	}

	// Check if this hypothesis passed validation (has high E-value)
	eValue, _ := payload["combined_e_value"].(float64)
	if eValue < 5.0 { // Threshold for "validated"
		return nil
	}

	return &ports.ValidatedHypothesisSummary{
		ID:          string(artifact.ID),
		CauseKey:    wa.extractString(payload, "cause_key"),
		EffectKey:   wa.extractString(payload, "effect_key"),
		EValue:      eValue,
		Confidence:  wa.extractFloat64(payload, "confidence"),
		Rationale:   wa.extractString(payload, "rationale"),
		TestCount:   int(wa.extractFloat64(payload, "test_count")),
		ValidatedAt: artifact.CreatedAt.Time().Format(time.RFC3339),
	}
}

// gatherRejectedHypotheses extracts failed hypotheses for learning
func (wa *WorkspaceAssembler) gatherRejectedHypotheses(ctx context.Context, workspaceID string) ([]ports.RejectedHypothesisSummary, error) {
	// Look for artifacts with validation_audit kind or failed hypothesis artifacts
	artifacts, err := wa.ledgerPort.ListArtifacts(ctx, ports.ArtifactFilters{
		Limit: 50, // Most recent failures
	})
	if err != nil {
		return nil, err
	}

	var rejected []ports.RejectedHypothesisSummary
	for _, artifact := range artifacts {
		if wa.isRejectionArtifact(artifact) {
			if rh := wa.extractRejectedHypothesis(artifact); rh != nil {
				rejected = append(rejected, *rh)
			}
		}
	}

	// Sort by recency (most recent failures first)
	sort.Slice(rejected, func(i, j int) bool {
		return rejected[i].RejectedAt > rejected[j].RejectedAt
	})

	return rejected, nil
}

// isRejectionArtifact checks if artifact represents a rejection
func (wa *WorkspaceAssembler) isRejectionArtifact(artifact core.Artifact) bool {
	// Check for variable health artifacts (audit artifacts)
	if artifact.Kind == core.ArtifactVariableHealth {
		return true
	}

	// Check for hypothesis artifacts with low E-values
	if artifact.Kind == core.ArtifactHypothesis {
		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if eValue, exists := payload["combined_e_value"].(float64); exists && eValue < 2.0 {
				return true
			}
		}
	}

	return false
}

// extractRejectedHypothesis parses rejection artifact
func (wa *WorkspaceAssembler) extractRejectedHypothesis(artifact core.Artifact) *ports.RejectedHypothesisSummary {
	payload, ok := artifact.Payload.(map[string]interface{})
	if !ok {
		return nil
	}

	return &ports.RejectedHypothesisSummary{
		ID:            string(artifact.ID),
		CauseKey:      wa.extractString(payload, "cause_key"),
		EffectKey:     wa.extractString(payload, "effect_key"),
		FailureReason: wa.extractString(payload, "failure_reason"),
		WeakestEValue: wa.extractFloat64(payload, "weakest_e_value"),
		RejectedAt:    artifact.CreatedAt.Time().Format(time.RFC3339),
		CommonFailure: wa.categorizeFailure(payload),
	}
}

// summarizeDatasets provides overview of datasets in workspace
func (wa *WorkspaceAssembler) summarizeDatasets(ctx context.Context, workspaceID string) ([]ports.DatasetSummary, error) {
	// This would query the dataset repository for workspace datasets
	// For now, return mock data
	return []ports.DatasetSummary{
		{
			Name:          "Primary Dataset",
			RecordCount:   10000,
			FieldCount:    25,
			TemporalRange: "2020-01-01 to 2024-01-01",
			KeyVariables:  []string{"outcome", "treatment", "time"},
		},
	}, nil
}

// extractForensicContext captures industry/domain context
func (wa *WorkspaceAssembler) extractForensicContext(ctx context.Context, workspaceID string) (ports.ForensicSummary, error) {
	// This would use the forensic scout service
	// For now, return mock data
	return ports.ForensicSummary{
		Domain:      "Sports Analytics",
		DatasetName: "Match Results Database",
		KeyInsights: []string{"Temporal patterns in performance", "Home advantage effects"},
		RiskFactors: []string{"Selection bias in match scheduling", "Confounding by team quality"},
	}, nil
}

// analyzeResearchTrajectory provides insights into research evolution
func (wa *WorkspaceAssembler) analyzeResearchTrajectory(validated []ports.ValidatedHypothesisSummary, rejected []ports.RejectedHypothesisSummary) ports.ResearchTrajectory {
	total := len(validated) + len(rejected)
	validationRate := 0.0
	if total > 0 {
		validationRate = float64(len(validated)) / float64(total)
	}

	commonThemes := wa.extractCommonThemes(validated)
	focusShifts := wa.analyzeFocusShifts(validated, rejected)

	return ports.ResearchTrajectory{
		TotalHypotheses: total,
		ValidationRate:  validationRate,
		CommonThemes:    commonThemes,
		EvolvingFocus:   focusShifts,
	}
}

// assessTemporalCoverage shows time periods covered
func (wa *WorkspaceAssembler) assessTemporalCoverage(datasets []ports.DatasetSummary) ports.TemporalCoverage {
	if len(datasets) == 0 {
		return ports.TemporalCoverage{
			DateRange:   "No temporal data",
			DataDensity: "Unknown",
			MissingGaps: []string{},
		}
	}

	// Extract temporal information from datasets
	var allRanges []string
	var missingGaps []string

	for _, ds := range datasets {
		if ds.TemporalRange != "" {
			allRanges = append(allRanges, ds.TemporalRange)
		}
	}

	dateRange := "Multiple ranges"
	if len(allRanges) == 1 {
		dateRange = allRanges[0]
	}

	return ports.TemporalCoverage{
		DateRange:   dateRange,
		DataDensity: "Daily resolution", // Would be analyzed from actual data
		MissingGaps: missingGaps,
	}
}

// extractCommonThemes identifies patterns in successful hypotheses
func (wa *WorkspaceAssembler) extractCommonThemes(validated []ports.ValidatedHypothesisSummary) []string {
	if len(validated) < 3 {
		return []string{}
	}

	// Simple pattern extraction - could be enhanced with NLP
	themes := make(map[string]int)

	for _, vh := range validated {
		// Extract variable prefixes/suffixes as themes
		causeParts := strings.Split(vh.CauseKey, "_")
		effectParts := strings.Split(vh.EffectKey, "_")

		if len(causeParts) > 1 {
			themes[causeParts[0]]++
		}
		if len(effectParts) > 1 {
			themes[effectParts[0]]++
		}
	}

	// Return top themes
	var topThemes []string
	for theme, count := range themes {
		if count >= 2 { // Appears in at least 2 hypotheses
			topThemes = append(topThemes, theme)
		}
	}

	return topThemes
}

// analyzeFocusShifts tracks how research focus has evolved
func (wa *WorkspaceAssembler) analyzeFocusShifts(validated []ports.ValidatedHypothesisSummary, rejected []ports.RejectedHypothesisSummary) []string {
	// This would analyze temporal patterns in hypothesis generation
	// For now, return mock data
	return []string{"Shifting from individual performance to team dynamics"}
}

// categorizeFailure determines the type of failure
func (wa *WorkspaceAssembler) categorizeFailure(payload map[string]interface{}) string {
	failureReason := wa.extractString(payload, "failure_reason")
	weakestEValue := wa.extractFloat64(payload, "weakest_e_value")

	if weakestEValue < 0.1 {
		return "very_weak_evidence"
	}

	switch {
	case strings.Contains(strings.ToLower(failureReason), "spurious"):
		return "spurious_correlation"
	case strings.Contains(strings.ToLower(failureReason), "temporal"):
		return "temporal_confounding"
	case strings.Contains(strings.ToLower(failureReason), "confound"):
		return "unmeasured_confounding"
	case strings.Contains(strings.ToLower(failureReason), "power"):
		return "insufficient_power"
	default:
		return "statistical_artifact"
	}
}

// AssembleHypothesisPrompt builds the complete prompt from workspace context
func (wa *WorkspaceAssembler) AssembleHypothesisPrompt(ctx context.Context, workspaceID string, directive ports.ResearchDirective) (string, error) {
	context, err := wa.AssembleFullContext(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("failed to assemble workspace context: %w", err)
	}

	var promptParts []string

	// 1. Domain Context & Forensic Intelligence
	promptParts = append(promptParts, wa.buildDomainContext(context.ForensicContext))

	// 2. Dataset Overview
	promptParts = append(promptParts, wa.buildDatasetOverview(context.DatasetSummaries, context.TemporalCoverage))

	// 3. Research Trajectory & Learning
	promptParts = append(promptParts, wa.buildResearchTrajectory(context.ResearchTrajectory))

	// 4. Validated Knowledge Base
	promptParts = append(promptParts, wa.buildValidatedKnowledge(context.ValidatedHypotheses))

	// 5. Rejection Learning (Most Important)
	promptParts = append(promptParts, wa.buildRejectionLearning(context.RejectedHypotheses))

	// 6. Evolution Instructions
	promptParts = append(promptParts, wa.buildEvolutionInstructions(directive, context))

	// 7. Counter-Hypothesis Requirement
	promptParts = append(promptParts, wa.buildCounterHypothesisRequirement(context.ValidatedHypotheses))

	// 8. Data Mart Suggestions
	promptParts = append(promptParts, wa.buildDataMartSuggestions(context))

	return strings.Join(promptParts, "\n\n---\n\n"), nil
}

// buildDomainContext formats forensic intelligence
func (wa *WorkspaceAssembler) buildDomainContext(forensic ports.ForensicSummary) string {
	var lines []string
	lines = append(lines, "DOMAIN CONTEXT & FORENSIC INTELLIGENCE:")
	lines = append(lines, fmt.Sprintf("Domain: %s", forensic.Domain))
	lines = append(lines, fmt.Sprintf("Dataset: %s", forensic.DatasetName))

	if len(forensic.KeyInsights) > 0 {
		lines = append(lines, "Key Domain Insights:")
		for _, insight := range forensic.KeyInsights {
			lines = append(lines, fmt.Sprintf("- %s", insight))
		}
	}

	if len(forensic.RiskFactors) > 0 {
		lines = append(lines, "Domain Risk Factors:")
		for _, risk := range forensic.RiskFactors {
			lines = append(lines, fmt.Sprintf("- %s", risk))
		}
	}

	return strings.Join(lines, "\n")
}

// buildDatasetOverview formats dataset information
func (wa *WorkspaceAssembler) buildDatasetOverview(datasets []ports.DatasetSummary, temporal ports.TemporalCoverage) string {
	var lines []string
	lines = append(lines, "DATASET OVERVIEW:")

	for _, ds := range datasets {
		lines = append(lines, fmt.Sprintf("- %s: %d records, %d fields", ds.Name, ds.RecordCount, ds.FieldCount))
		if ds.TemporalRange != "" {
			lines = append(lines, fmt.Sprintf("  Temporal range: %s", ds.TemporalRange))
		}
		if len(ds.KeyVariables) > 0 {
			lines = append(lines, fmt.Sprintf("  Key variables: %s", strings.Join(ds.KeyVariables, ", ")))
		}
	}

	lines = append(lines, fmt.Sprintf("Temporal coverage: %s (%s)", temporal.DateRange, temporal.DataDensity))

	return strings.Join(lines, "\n")
}

// buildResearchTrajectory formats research progress
func (wa *WorkspaceAssembler) buildResearchTrajectory(trajectory ports.ResearchTrajectory) string {
	var lines []string
	lines = append(lines, "RESEARCH TRAJECTORY:")
	lines = append(lines, fmt.Sprintf("Total hypotheses tested: %d", trajectory.TotalHypotheses))
	lines = append(lines, fmt.Sprintf("Validation success rate: %.1f%%", trajectory.ValidationRate*100))

	if len(trajectory.CommonThemes) > 0 {
		lines = append(lines, fmt.Sprintf("Emerging themes: %s", strings.Join(trajectory.CommonThemes, ", ")))
	}

	if len(trajectory.EvolvingFocus) > 0 {
		lines = append(lines, "Research focus evolution:")
		for _, focus := range trajectory.EvolvingFocus {
			lines = append(lines, fmt.Sprintf("- %s", focus))
		}
	}

	return strings.Join(lines, "\n")
}

// buildValidatedKnowledge formats successful hypotheses
func (wa *WorkspaceAssembler) buildValidatedKnowledge(validated []ports.ValidatedHypothesisSummary) string {
	if len(validated) == 0 {
		return "VALIDATED KNOWLEDGE BASE:\nNone yet. This is our first discovery cycle."
	}

	var lines []string
	lines = append(lines, "VALIDATED KNOWLEDGE BASE:")
	lines = append(lines, "These relationships have survived rigorous statistical validation:")

	for i, vh := range validated {
		if i >= 10 { // Limit to top 10
			break
		}

		strength := wa.categorizeEvidenceStrength(vh.EValue)
		lines = append(lines, fmt.Sprintf("✓ %s → %s (E=%.1f, %s confidence, %d tests)",
			vh.CauseKey, vh.EffectKey, vh.EValue, strength, vh.TestCount))

		if vh.Rationale != "" {
			lines = append(lines, fmt.Sprintf("  Reason: %s", vh.Rationale))
		}
	}

	return strings.Join(lines, "\n")
}

// buildRejectionLearning formats failed hypotheses for learning
func (wa *WorkspaceAssembler) buildRejectionLearning(rejected []ports.RejectedHypothesisSummary) string {
	if len(rejected) == 0 {
		return "REJECTION LEARNING:\nNo rejections yet. Avoid obvious spurious correlations."
	}

	// Group by failure type
	failureGroups := make(map[string][]ports.RejectedHypothesisSummary)
	for _, rh := range rejected {
		failureGroups[rh.CommonFailure] = append(failureGroups[rh.CommonFailure], rh)
	}

	var lines []string
	lines = append(lines, "REJECTION LEARNING:")
	lines = append(lines, "These hypothesis patterns have been statistically rejected:")

	for failureType, hypotheses := range failureGroups {
		lines = append(lines, fmt.Sprintf("\n🚫 %s (%d instances):", wa.humanizeFailureType(failureType), len(hypotheses)))

		for _, rh := range hypotheses[:3] { // Show up to 3 examples per type
			lines = append(lines, fmt.Sprintf("  ✗ %s → %s (%s)",
				rh.CauseKey, rh.EffectKey, rh.FailureReason))
		}
	}

	lines = append(lines, "\n⚠️  AVOID repeating these logical fallacies in new hypotheses!")

	return strings.Join(lines, "\n")
}

// buildEvolutionInstructions provides guidance based on current state
func (wa *WorkspaceAssembler) buildEvolutionInstructions(directive ports.ResearchDirective, context *ports.WorkspaceContext) string {
	var instructions []string

	instructions = append(instructions, "EVOLUTION INSTRUCTIONS:")

	// Check if we have validated knowledge to build upon
	if len(context.ValidatedHypotheses) > 0 {
		instructions = append(instructions, "✓ BUILD UPON VALIDATED KNOWLEDGE: Propose hypotheses that explain or extend our confirmed findings.")
		instructions = append(instructions, "✓ MECHANISM DISCOVERY: For each validated relationship, propose what underlying factor might explain it.")
	}

	// Check for rejection patterns to avoid
	if len(context.RejectedHypotheses) > 0 {
		instructions = append(instructions, "🚫 LEARN FROM REJECTIONS: Study the rejection patterns and ensure new hypotheses don't repeat these mistakes.")
	}

	// Check research trajectory
	trajectory := context.ResearchTrajectory
	if trajectory.ValidationRate < 0.3 {
		instructions = append(instructions, "🎯 FOCUS ON QUALITY: Our validation rate is low. Prioritize hypotheses with strong logical foundations.")
	} else if trajectory.ValidationRate > 0.7 {
		instructions = append(instructions, "🚀 EXPAND SCOPE: We're finding good hypotheses. Try exploring new variable combinations.")
	}

	// Add specific directive
	instructions = append(instructions, fmt.Sprintf("🎯 CURRENT DIRECTIVE: %s", directive.Description))

	return strings.Join(instructions, "\n")
}

// buildCounterHypothesisRequirement ensures falsification mindset
func (wa *WorkspaceAssembler) buildCounterHypothesisRequirement(validated []ports.ValidatedHypothesisSummary) string {
	if len(validated) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "COUNTER-HYPOTHESIS REQUIREMENT:")
	lines = append(lines, "For EACH validated relationship above, propose how it might actually be a false positive:")

	for i, vh := range validated[:3] { // Focus on top 3
		lines = append(lines, fmt.Sprintf("%d. For %s → %s: What confounder or alternative explanation might explain this?",
			i+1, vh.CauseKey, vh.EffectKey))
	}

	return strings.Join(lines, "\n")
}

// buildDataMartSuggestions identifies information gaps
func (wa *WorkspaceAssembler) buildDataMartSuggestions(context *ports.WorkspaceContext) string {
	var suggestions []string

	// Analyze rejection patterns for data gaps
	confoundingRejections := 0
	temporalRejections := 0

	for _, rh := range context.RejectedHypotheses {
		switch rh.CommonFailure {
		case "unmeasured_confounding":
			confoundingRejections++
		case "temporal_confounding":
			temporalRejections++
		}
	}

	if confoundingRejections > temporalRejections {
		suggestions = append(suggestions, "DATA MART SUGGESTION: Consider adding confounding variables (weather, team quality, economic factors) to control for alternative explanations.")
	} else if temporalRejections > 0 {
		suggestions = append(suggestions, "DATA MART SUGGESTION: Temporal patterns suggest adding time-series controls (seasonal effects, trend variables).")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "DATA MART STATUS: Current dataset appears sufficient for hypothesis testing.")
	}

	return strings.Join(suggestions, "\n")
}

// categorizeEvidenceStrength converts E-value to human-readable strength
func (wa *WorkspaceAssembler) categorizeEvidenceStrength(eValue float64) string {
	switch {
	case eValue >= 20:
		return "very strong"
	case eValue >= 10:
		return "strong"
	case eValue >= 5:
		return "moderate"
	case eValue >= 2:
		return "weak"
	default:
		return "very weak"
	}
}

// humanizeFailureType converts failure codes to readable explanations
func (wa *WorkspaceAssembler) humanizeFailureType(failureType string) string {
	switch failureType {
	case "spurious_correlation":
		return "Spurious Correlations (like ice cream sales vs. shark attacks)"
	case "temporal_confounding":
		return "Temporal Confounding (time-related false links)"
	case "unmeasured_confounding":
		return "Unmeasured Confounding (missing control variables)"
	case "statistical_artifact":
		return "Statistical Artifacts (p-hacking, multiple testing)"
	case "insufficient_power":
		return "Insufficient Statistical Power"
	default:
		return strings.Title(strings.ReplaceAll(failureType, "_", " "))
	}
}

// Helper methods for safe type extraction
func (wa *WorkspaceAssembler) extractString(payload map[string]interface{}, key string) string {
	if val, ok := payload[key].(string); ok {
		return val
	}
	return ""
}

func (wa *WorkspaceAssembler) extractFloat64(payload map[string]interface{}, key string) float64 {
	if val, ok := payload[key].(float64); ok {
		return val
	}
	return 0.0
}
