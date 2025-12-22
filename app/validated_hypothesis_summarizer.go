package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gohypo/models"
	"gohypo/ports"
)

// ValidatedHypothesisSummarizer generates summaries of validated hypotheses for feedback into research generation
type ValidatedHypothesisSummarizer struct {
	hypothesisRepo ports.HypothesisRepository
}

// ValidatedHypothesisSummary represents a structured summary of both validated and invalidated hypotheses
type ValidatedHypothesisSummary struct {
	// Overall statistics
	TotalValidatedHypotheses   int     `json:"total_validated_hypotheses"`
	TotalInvalidatedHypotheses int     `json:"total_invalidated_hypotheses"`
	AverageConfidence          float64 `json:"average_confidence"`
	AverageNormalizedEValue    float64 `json:"average_normalized_e_value"`

	// Variable relationship patterns (validated)
	TopCauseEffectPairs []CauseEffectPair `json:"top_cause_effect_pairs"`
	CommonEffectKeys    []VariableFrequency `json:"common_effect_keys"`
	CommonCauseKeys     []VariableFrequency `json:"common_cause_keys"`

	// Failed relationship patterns (invalidated)
	FailedCauseEffectPairs []CauseEffectPair     `json:"failed_cause_effect_pairs"`
	CommonFailureReasons   []FailureReason       `json:"common_failure_reasons"`

	// Referee success patterns
	RefereeSuccessRates []RefereePerformance `json:"referee_success_rates"`
	RefereeCombinations []RefereeCombination `json:"referee_combinations"`

	// Risk and feasibility patterns
	RiskLevelDistribution map[string]int `json:"risk_level_distribution"`
	FeasibilityScoreRanges []ScoreRange  `json:"feasibility_score_ranges"`

	// Confidence thresholds
	ConfidenceThresholds []ConfidenceThreshold `json:"confidence_thresholds"`

	// Recent validated hypotheses (last 30 days)
	RecentValidatedHypotheses []RecentHypothesis `json:"recent_validated_hypotheses"`

	// Recent invalidated hypotheses (learning from failures)
	RecentInvalidatedHypotheses []RecentHypothesis `json:"recent_invalidated_hypotheses"`

	// Summary generation timestamp
	GeneratedAt time.Time `json:"generated_at"`
}

// CauseEffectPair represents a validated cause-effect relationship
type CauseEffectPair struct {
	CauseKey       string  `json:"cause_key"`
	EffectKey      string  `json:"effect_key"`
	Frequency      int     `json:"frequency"`
	AverageConfidence float64 `json:"average_confidence"`
	BusinessExamples []string `json:"business_examples"` // Sample business hypotheses
}

// VariableFrequency tracks how often a variable appears in validated hypotheses
type VariableFrequency struct {
	Variable  string  `json:"variable"`
	Frequency int     `json:"frequency"`
	AverageConfidence float64 `json:"average_confidence"`
}

// RefereePerformance tracks success rates of individual referees
type RefereePerformance struct {
	RefereeName     string  `json:"referee_name"`
	RefereeCategory string  `json:"referee_category"`
	SuccessRate     float64 `json:"success_rate"`
	TotalTests      int     `json:"total_tests"`
	AveragePValue   float64 `json:"average_p_value"`
}

// RefereeCombination tracks successful referee combinations
type RefereeCombination struct {
	Referees       []string `json:"referees"`
	Frequency      int      `json:"frequency"`
	SuccessRate    float64  `json:"success_rate"`
	AverageConfidence float64 `json:"average_confidence"`
}

// ScoreRange represents a range of feasibility scores
type ScoreRange struct {
	MinScore  float64 `json:"min_score"`
	MaxScore  float64 `json:"max_score"`
	Frequency int     `json:"frequency"`
	AverageConfidence float64 `json:"average_confidence"`
}

// ConfidenceThreshold represents confidence level thresholds
type ConfidenceThreshold struct {
	Threshold       float64 `json:"threshold"`
	SuccessRate     float64 `json:"success_rate"`
	SampleSize      int     `json:"sample_size"`
}

// FailureReason represents common reasons why hypotheses fail validation
type FailureReason struct {
	Reason     string  `json:"reason"`
	Frequency  int     `json:"frequency"`
	Percentage float64 `json:"percentage"`
}

// RecentHypothesis represents a recent hypothesis (validated or invalidated)
type RecentHypothesis struct {
	BusinessHypothesis string    `json:"business_hypothesis"`
	CauseKey           string    `json:"cause_key"`
	EffectKey          string    `json:"effect_key"`
	Confidence         float64   `json:"confidence"`
	Referees           []string  `json:"referees"`
	ValidatedAt        time.Time `json:"validated_at"`
	FailureReason      string    `json:"failure_reason,omitempty"` // Only for invalidated hypotheses
}

// NewValidatedHypothesisSummarizer creates a new summarizer instance
func NewValidatedHypothesisSummarizer(hypothesisRepo ports.HypothesisRepository) *ValidatedHypothesisSummarizer {
	return &ValidatedHypothesisSummarizer{
		hypothesisRepo: hypothesisRepo,
	}
}

// GenerateSummary creates a comprehensive summary of both validated and invalidated hypotheses for a user
func (s *ValidatedHypothesisSummarizer) GenerateSummary(ctx context.Context, userID uuid.UUID, limit int) (*ValidatedHypothesisSummary, error) {
	if limit <= 0 {
		limit = 1000 // Default to last 1000 hypotheses (validated + invalidated)
	}

	// Retrieve validated hypotheses
	validatedHypotheses, err := s.hypothesisRepo.ListByValidationState(ctx, userID, true, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve validated hypotheses: %w", err)
	}

	// Retrieve invalidated hypotheses
	invalidatedHypotheses, err := s.hypothesisRepo.ListByValidationState(ctx, userID, false, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve invalidated hypotheses: %w", err)
	}

	summary := &ValidatedHypothesisSummary{
		TotalValidatedHypotheses:   len(validatedHypotheses),
		TotalInvalidatedHypotheses: len(invalidatedHypotheses),
		GeneratedAt:                time.Now(),
	}

	// Calculate basic statistics for validated hypotheses
	if len(validatedHypotheses) > 0 {
		s.calculateBasicStats(summary, validatedHypotheses)
	}

	// Analyze validated variable relationships
	if len(validatedHypotheses) > 0 {
		s.analyzeVariableRelationships(summary, validatedHypotheses)
	}

	// Analyze invalidated hypotheses patterns
	if len(invalidatedHypotheses) > 0 {
		s.analyzeFailedRelationships(summary, invalidatedHypotheses)
		s.analyzeFailureReasons(summary, invalidatedHypotheses)
	}

	// Analyze referee performance (across both validated and invalidated)
	allHypotheses := append(validatedHypotheses, invalidatedHypotheses...)
	if len(allHypotheses) > 0 {
		s.analyzeRefereePerformance(summary, allHypotheses)

		// Analyze risk and feasibility patterns (across all hypotheses)
		s.analyzeRiskFeasibilityPatterns(summary, allHypotheses)

		// Analyze confidence thresholds (across all hypotheses)
		s.analyzeConfidenceThresholds(summary, allHypotheses)
	}

	// Extract recent validated hypotheses
	if len(validatedHypotheses) > 0 {
		s.extractRecentHypotheses(summary, validatedHypotheses)
	}

	// Extract recent invalidated hypotheses
	if len(invalidatedHypotheses) > 0 {
		s.extractRecentInvalidatedHypotheses(summary, invalidatedHypotheses)
	}

	return summary, nil
}

// calculateBasicStats computes basic statistical measures
func (s *ValidatedHypothesisSummarizer) calculateBasicStats(summary *ValidatedHypothesisSummary, hypotheses []*models.HypothesisResult) {
	var totalConfidence, totalNormalizedEValue float64

	for _, h := range hypotheses {
		totalConfidence += h.Confidence
		totalNormalizedEValue += h.NormalizedEValue
	}

	if len(hypotheses) > 0 {
		summary.AverageConfidence = totalConfidence / float64(len(hypotheses))
		summary.AverageNormalizedEValue = totalNormalizedEValue / float64(len(hypotheses))
	}
}

// analyzeVariableRelationships identifies patterns in cause-effect relationships
func (s *ValidatedHypothesisSummarizer) analyzeVariableRelationships(summary *ValidatedHypothesisSummary, hypotheses []*models.HypothesisResult) {
	// Maps to track frequencies and confidence scores
	causeEffectPairs := make(map[string]*CauseEffectPair)
	effectKeyFreq := make(map[string]*VariableFrequency)
	causeKeyFreq := make(map[string]*VariableFrequency)

	for _, h := range hypotheses {
		// Extract cause and effect keys from execution metadata or hypothesis text
		causeKey, effectKey := s.extractCauseEffectKeys(h)

		if causeKey != "" && effectKey != "" {
			pairKey := causeKey + "|" + effectKey

			if pair, exists := causeEffectPairs[pairKey]; exists {
				pair.Frequency++
				pair.AverageConfidence = (pair.AverageConfidence*float64(pair.Frequency-1) + h.Confidence) / float64(pair.Frequency)
				if len(pair.BusinessExamples) < 3 { // Keep up to 3 examples
					pair.BusinessExamples = append(pair.BusinessExamples, h.BusinessHypothesis)
				}
			} else {
				causeEffectPairs[pairKey] = &CauseEffectPair{
					CauseKey:           causeKey,
					EffectKey:          effectKey,
					Frequency:          1,
					AverageConfidence:  h.Confidence,
					BusinessExamples:   []string{h.BusinessHypothesis},
				}
			}
		}

		// Track individual variable frequencies
		if effectKey != "" {
			if freq, exists := effectKeyFreq[effectKey]; exists {
				freq.Frequency++
				freq.AverageConfidence = (freq.AverageConfidence*float64(freq.Frequency-1) + h.Confidence) / float64(freq.Frequency)
			} else {
				effectKeyFreq[effectKey] = &VariableFrequency{
					Variable:          effectKey,
					Frequency:         1,
					AverageConfidence: h.Confidence,
				}
			}
		}

		if causeKey != "" {
			if freq, exists := causeKeyFreq[causeKey]; exists {
				freq.Frequency++
				freq.AverageConfidence = (freq.AverageConfidence*float64(freq.Frequency-1) + h.Confidence) / float64(freq.Frequency)
			} else {
				causeKeyFreq[causeKey] = &VariableFrequency{
					Variable:          causeKey,
					Frequency:         1,
					AverageConfidence: h.Confidence,
				}
			}
		}
	}

	// Convert maps to sorted slices
	summary.TopCauseEffectPairs = s.mapToSortedCauseEffectPairs(causeEffectPairs)
	summary.CommonEffectKeys = s.mapToSortedVariableFrequencies(effectKeyFreq)
	summary.CommonCauseKeys = s.mapToSortedVariableFrequencies(causeKeyFreq)
}

// extractCauseEffectKeys extracts cause and effect variable names from hypothesis data
func (s *ValidatedHypothesisSummarizer) extractCauseEffectKeys(h *models.HypothesisResult) (causeKey, effectKey string) {
	// Try to extract from execution metadata first
	if h.ExecutionMetadata != nil {
		if cause, ok := h.ExecutionMetadata["cause_key"].(string); ok {
			causeKey = cause
		}
		if effect, ok := h.ExecutionMetadata["effect_key"].(string); ok {
			effectKey = effect
		}
	}

	// If not found in metadata, try to extract from science hypothesis text
	if causeKey == "" || effectKey == "" {
		scienceHyp := strings.ToLower(h.ScienceHypothesis)

		// Look for common patterns in science hypotheses
		patterns := []string{
			"predicts", "influences", "affects", "determines", "correlates with",
			"leads to", "causes", "results in", "associated with",
		}

		for _, pattern := range patterns {
			if idx := strings.Index(scienceHyp, pattern); idx != -1 {
				// Extract variable names around the pattern
				before := scienceHyp[:idx]
				after := scienceHyp[idx+len(pattern):]

				// Simple heuristic: look for variable-like patterns (uppercase letters, underscores)
				causeKey = s.extractVariableName(before)
				effectKey = s.extractVariableName(after)
				break
			}
		}
	}

	return causeKey, effectKey
}

// extractVariableName attempts to extract a variable name from text
func (s *ValidatedHypothesisSummarizer) extractVariableName(text string) string {
	// Look for patterns that match variable names (e.g., "HTP", "FTR", "HTGD")
	words := strings.Fields(text)
	for _, word := range words {
		// Remove punctuation
		word = strings.Trim(word, ".,!?\"'()")

		// Check if it looks like a variable name (uppercase, may contain underscores/numbers)
		if len(word) >= 2 && len(word) <= 20 {
			hasUpper := false
			hasSpecial := false
			for _, r := range word {
				if r >= 'A' && r <= 'Z' {
					hasUpper = true
				}
				if r == '_' || (r >= '0' && r <= '9') {
					hasSpecial = true
				}
			}

			if hasUpper && (hasSpecial || len(word) <= 6) {
				return word
			}
		}
	}
	return ""
}

// analyzeRefereePerformance analyzes which referees perform well
func (s *ValidatedHypothesisSummarizer) analyzeRefereePerformance(summary *ValidatedHypothesisSummary, hypotheses []*models.HypothesisResult) {
	refereeStats := make(map[string]*RefereePerformance)
	refereeCombinations := make(map[string]*RefereeCombination)

	for _, h := range hypotheses {
		refereeNames := make([]string, len(h.RefereeResults))
		for i, result := range h.RefereeResults {
			refereeNames[i] = result.GateName

			if stat, exists := refereeStats[result.GateName]; exists {
				stat.TotalTests++
				if result.Passed {
					stat.SuccessRate = (stat.SuccessRate*float64(stat.TotalTests-1) + 1) / float64(stat.TotalTests)
				} else {
					stat.SuccessRate = (stat.SuccessRate * float64(stat.TotalTests-1)) / float64(stat.TotalTests)
				}
				if result.PValue >= 0 {
					stat.AveragePValue = (stat.AveragePValue*float64(stat.TotalTests-1) + result.PValue) / float64(stat.TotalTests)
				}
			} else {
				success := 0.0
				if result.Passed {
					success = 1.0
				}
				refereeStats[result.GateName] = &RefereePerformance{
					RefereeName:     result.GateName,
					RefereeCategory: s.getRefereeCategory(result.GateName),
					SuccessRate:     success,
					TotalTests:      1,
					AveragePValue:   result.PValue,
				}
			}
		}

		// Track referee combinations
		sort.Strings(refereeNames)
		comboKey := strings.Join(refereeNames, "+")

		if combo, exists := refereeCombinations[comboKey]; exists {
			combo.Frequency++
			combo.SuccessRate = (combo.SuccessRate*float64(combo.Frequency-1) + 1) / float64(combo.Frequency) // All in this list passed
			combo.AverageConfidence = (combo.AverageConfidence*float64(combo.Frequency-1) + h.Confidence) / float64(combo.Frequency)
		} else {
			refereeCombinations[comboKey] = &RefereeCombination{
				Referees:           refereeNames,
				Frequency:          1,
				SuccessRate:        1.0,
				AverageConfidence:  h.Confidence,
			}
		}
	}

	// Convert to sorted slices
	summary.RefereeSuccessRates = s.mapToSortedRefereePerformance(refereeStats)
	summary.RefereeCombinations = s.mapToSortedRefereeCombinations(refereeCombinations)
}

// getRefereeCategory determines the category of a referee
func (s *ValidatedHypothesisSummarizer) getRefereeCategory(refereeName string) string {
	categories := map[string]string{
		// SHREDDER
		"Permutation_Shredder": "SHREDDER",

		// DIRECTIONAL
		"Transfer_Entropy":        "DIRECTIONAL",
		"Convergent_Cross_Mapping": "DIRECTIONAL",

		// INVARIANCE
		"Chow_Stability_Test": "INVARIANCE",

		// ANTI_CONFOUNDER
		"Conditional_MI": "ANTI_CONFOUNDER",

		// MECHANISM
		"Isotonic_Mechanism_Check": "MECHANISM",

		// SENSITIVITY
		"LOO_Cross_Validation": "SENSITIVITY",

		// TOPOLOGICAL
		"Persistent_Homology": "TOPOLOGICAL",

		// THERMODYNAMIC
		"Algorithmic_Complexity": "THERMODYNAMIC",

		// COUNTERFACTUAL
		"Synthetic_Intervention": "COUNTERFACTUAL",

		// SPECTRAL
		"Wavelet_Coherence": "SPECTRAL",
	}

	if category, exists := categories[refereeName]; exists {
		return category
	}
	return "UNKNOWN"
}

// analyzeRiskFeasibilityPatterns analyzes patterns in risk levels and feasibility scores
func (s *ValidatedHypothesisSummarizer) analyzeRiskFeasibilityPatterns(summary *ValidatedHypothesisSummary, hypotheses []*models.HypothesisResult) {
	riskDistribution := make(map[string]int)
	feasibilityRanges := make(map[string]*ScoreRange)

	for _, h := range hypotheses {
		// Count risk levels
		riskDistribution[h.RiskLevel]++

		// Group feasibility scores into ranges
		rangeKey := s.getFeasibilityRange(h.FeasibilityScore)
		if scoreRange, exists := feasibilityRanges[rangeKey]; exists {
			scoreRange.Frequency++
			scoreRange.AverageConfidence = (scoreRange.AverageConfidence*float64(scoreRange.Frequency-1) + h.Confidence) / float64(scoreRange.Frequency)
		} else {
			minScore, maxScore := s.parseFeasibilityRange(rangeKey)
			feasibilityRanges[rangeKey] = &ScoreRange{
				MinScore:          minScore,
				MaxScore:          maxScore,
				Frequency:         1,
				AverageConfidence: h.Confidence,
			}
		}
	}

	summary.RiskLevelDistribution = riskDistribution
	summary.FeasibilityScoreRanges = s.mapToSortedScoreRanges(feasibilityRanges)
}

// getFeasibilityRange groups feasibility scores into ranges
func (s *ValidatedHypothesisSummarizer) getFeasibilityRange(score float64) string {
	if score < 0.33 {
		return "0.0-0.33"
	} else if score < 0.66 {
		return "0.33-0.66"
	} else {
		return "0.66-1.0"
	}
}

// parseFeasibilityRange parses range string back to min/max
func (s *ValidatedHypothesisSummarizer) parseFeasibilityRange(rangeStr string) (float64, float64) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) == 2 {
		// Simple parsing - in production you'd want more robust parsing
		switch rangeStr {
		case "0.0-0.33":
			return 0.0, 0.33
		case "0.33-0.66":
			return 0.33, 0.66
		case "0.66-1.0":
			return 0.66, 1.0
		}
	}
	return 0.0, 1.0
}

// analyzeConfidenceThresholds analyzes success rates at different confidence levels
func (s *ValidatedHypothesisSummarizer) analyzeConfidenceThresholds(summary *ValidatedHypothesisSummary, hypotheses []*models.HypothesisResult) {
	thresholds := []float64{0.5, 0.6, 0.7, 0.8, 0.9, 0.95, 0.99}

	for _, threshold := range thresholds {
		aboveThreshold := 0
		total := 0

		for _, h := range hypotheses {
			if h.Confidence >= threshold {
				aboveThreshold++
			}
			total++
		}

		if total > 0 {
			successRate := float64(aboveThreshold) / float64(total)
			summary.ConfidenceThresholds = append(summary.ConfidenceThresholds, ConfidenceThreshold{
				Threshold:   threshold,
				SuccessRate: successRate,
				SampleSize:  total,
			})
		}
	}
}

// analyzeFailedRelationships analyzes patterns in failed cause-effect relationships
func (s *ValidatedHypothesisSummarizer) analyzeFailedRelationships(summary *ValidatedHypothesisSummary, invalidatedHypotheses []*models.HypothesisResult) {
	// Maps to track failed relationship frequencies
	failedPairs := make(map[string]*CauseEffectPair)

	for _, h := range invalidatedHypotheses {
		// Extract cause and effect keys from execution metadata or hypothesis text
		causeKey, effectKey := s.extractCauseEffectKeys(h)

		if causeKey != "" && effectKey != "" {
			pairKey := causeKey + "|" + effectKey

			if pair, exists := failedPairs[pairKey]; exists {
				pair.Frequency++
				pair.AverageConfidence = (pair.AverageConfidence*float64(pair.Frequency-1) + h.Confidence) / float64(pair.Frequency)
				if len(pair.BusinessExamples) < 3 { // Keep up to 3 examples
					pair.BusinessExamples = append(pair.BusinessExamples, h.BusinessHypothesis)
				}
			} else {
				failedPairs[pairKey] = &CauseEffectPair{
					CauseKey:           causeKey,
					EffectKey:          effectKey,
					Frequency:          1,
					AverageConfidence:  h.Confidence,
					BusinessExamples:   []string{h.BusinessHypothesis},
				}
			}
		}
	}

	// Convert to sorted slice and take top failures
	summary.FailedCauseEffectPairs = s.mapToSortedCauseEffectPairs(failedPairs)
	// Limit to top 10 failed pairs to show common failure patterns
	if len(summary.FailedCauseEffectPairs) > 10 {
		summary.FailedCauseEffectPairs = summary.FailedCauseEffectPairs[:10]
	}
}

// analyzeFailureReasons analyzes common reasons why hypotheses fail
func (s *ValidatedHypothesisSummarizer) analyzeFailureReasons(summary *ValidatedHypothesisSummary, invalidatedHypotheses []*models.HypothesisResult) {
	failureReasons := make(map[string]int)

	for _, h := range invalidatedHypotheses {
		// Extract failure reason from referee results
		failureReason := s.extractFailureReason(h)
		if failureReason != "" {
			failureReasons[failureReason]++
		}
	}

	// Convert to sorted slice
	totalFailures := len(invalidatedHypotheses)
	failureReasonList := make([]FailureReason, 0, len(failureReasons))

	for reason, count := range failureReasons {
		percentage := float64(count) / float64(totalFailures) * 100
		failureReasonList = append(failureReasonList, FailureReason{
			Reason:     reason,
			Frequency:  count,
			Percentage: percentage,
		})
	}

	// Sort by frequency descending
	sort.Slice(failureReasonList, func(i, j int) bool {
		return failureReasonList[i].Frequency > failureReasonList[j].Frequency
	})

	summary.CommonFailureReasons = failureReasonList
}

// extractFailureReason extracts the primary failure reason from referee results
func (s *ValidatedHypothesisSummarizer) extractFailureReason(h *models.HypothesisResult) string {
	// Look through referee results for failure reasons
	for _, result := range h.RefereeResults {
		if !result.Passed && result.FailureReason != "" {
			return result.FailureReason
		}
	}

	// If no specific failure reason, categorize by referee type
	failedReferees := make([]string, 0)
	for _, result := range h.RefereeResults {
		if !result.Passed {
			failedReferees = append(failedReferees, result.GateName)
		}
	}

	if len(failedReferees) > 0 {
		return fmt.Sprintf("Failed %s validation", strings.Join(failedReferees, ", "))
	}

	return "Unknown validation failure"
}

// extractRecentHypotheses gets the most recent validated hypotheses
func (s *ValidatedHypothesisSummarizer) extractRecentHypotheses(summary *ValidatedHypothesisSummary, hypotheses []*models.HypothesisResult) {
	// Sort by validation timestamp (most recent first)
	sort.Slice(hypotheses, func(i, j int) bool {
		return hypotheses[i].ValidationTimestamp.After(hypotheses[j].ValidationTimestamp)
	})

	// Take the most recent 10
	recentCount := 10
	if len(hypotheses) < recentCount {
		recentCount = len(hypotheses)
	}

	summary.RecentValidatedHypotheses = make([]RecentHypothesis, recentCount)
	for i := 0; i < recentCount; i++ {
		h := hypotheses[i]
		referees := make([]string, len(h.RefereeResults))
		for j, result := range h.RefereeResults {
			referees[j] = result.GateName
		}

		causeKey, effectKey := s.extractCauseEffectKeys(h)

		summary.RecentValidatedHypotheses[i] = RecentHypothesis{
			BusinessHypothesis: h.BusinessHypothesis,
			CauseKey:           causeKey,
			EffectKey:          effectKey,
			Confidence:         h.Confidence,
			Referees:           referees,
			ValidatedAt:        h.ValidationTimestamp,
		}
	}
}

// extractRecentInvalidatedHypotheses gets the most recent invalidated hypotheses for learning from failures
func (s *ValidatedHypothesisSummarizer) extractRecentInvalidatedHypotheses(summary *ValidatedHypothesisSummary, invalidatedHypotheses []*models.HypothesisResult) {
	// Sort by validation timestamp (most recent first)
	sort.Slice(invalidatedHypotheses, func(i, j int) bool {
		return invalidatedHypotheses[i].ValidationTimestamp.After(invalidatedHypotheses[j].ValidationTimestamp)
	})

	// Take the most recent 5 invalidated hypotheses (less than validated since we want to focus on successes)
	recentCount := 5
	if len(invalidatedHypotheses) < recentCount {
		recentCount = len(invalidatedHypotheses)
	}

	summary.RecentInvalidatedHypotheses = make([]RecentHypothesis, recentCount)
	for i := 0; i < recentCount; i++ {
		h := invalidatedHypotheses[i]
		referees := make([]string, len(h.RefereeResults))
		for j, result := range h.RefereeResults {
			referees[j] = result.GateName
		}

		causeKey, effectKey := s.extractCauseEffectKeys(h)
		failureReason := s.extractFailureReason(h)

		summary.RecentInvalidatedHypotheses[i] = RecentHypothesis{
			BusinessHypothesis: h.BusinessHypothesis,
			CauseKey:           causeKey,
			EffectKey:          effectKey,
			Confidence:         h.Confidence,
			Referees:           referees,
			ValidatedAt:        h.ValidationTimestamp,
			FailureReason:      failureReason,
		}
	}
}

// Helper functions for sorting and converting maps to slices

func (s *ValidatedHypothesisSummarizer) mapToSortedCauseEffectPairs(pairs map[string]*CauseEffectPair) []CauseEffectPair {
	result := make([]CauseEffectPair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, *pair)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Frequency > result[j].Frequency
	})

	// Limit to top 20
	if len(result) > 20 {
		result = result[:20]
	}

	return result
}

func (s *ValidatedHypothesisSummarizer) mapToSortedVariableFrequencies(freqs map[string]*VariableFrequency) []VariableFrequency {
	result := make([]VariableFrequency, 0, len(freqs))
	for _, freq := range freqs {
		result = append(result, *freq)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Frequency > result[j].Frequency
	})

	// Limit to top 15
	if len(result) > 15 {
		result = result[:15]
	}

	return result
}

func (s *ValidatedHypothesisSummarizer) mapToSortedRefereePerformance(performances map[string]*RefereePerformance) []RefereePerformance {
	result := make([]RefereePerformance, 0, len(performances))
	for _, perf := range performances {
		result = append(result, *perf)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].SuccessRate > result[j].SuccessRate
	})

	return result
}

func (s *ValidatedHypothesisSummarizer) mapToSortedRefereeCombinations(combinations map[string]*RefereeCombination) []RefereeCombination {
	result := make([]RefereeCombination, 0, len(combinations))
	for _, combo := range combinations {
		result = append(result, *combo)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Frequency > result[j].Frequency
	})

	// Limit to top 10 combinations
	if len(result) > 10 {
		result = result[:10]
	}

	return result
}

func (s *ValidatedHypothesisSummarizer) mapToSortedScoreRanges(ranges map[string]*ScoreRange) []ScoreRange {
	result := make([]ScoreRange, 0, len(ranges))
	for _, r := range ranges {
		result = append(result, *r)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Frequency > result[j].Frequency
	})

	return result
}
