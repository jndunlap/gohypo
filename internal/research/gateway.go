package research

import (
	"context"
	"fmt"
	"log"
	"math"
	"strings"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
)

// SuccessGateway implements the "Success-Only" persistence strategy
// Only hypotheses that survive the Tri-Gate Gauntlet reach the database
type SuccessGateway struct {
	hypothesisRepo ports.HypothesisRepository
	userRepo       ports.UserRepository
	sessionRepo    ports.SessionRepository
}

// NewSuccessGateway creates a new success-only gateway
func NewSuccessGateway(hypothesisRepo ports.HypothesisRepository, userRepo ports.UserRepository, sessionRepo ports.SessionRepository) *SuccessGateway {
	return &SuccessGateway{
		hypothesisRepo: hypothesisRepo,
		userRepo:       userRepo,
		sessionRepo:    sessionRepo,
	}
}

// PersistUniversalLaw implements the "Success-Only" persistence strategy
// This is the critical Layer 3 gateway that ensures only economically viable,
// statistically validated hypotheses reach permanent storage
func (sg *SuccessGateway) PersistUniversalLaw(ctx context.Context, hypothesis *models.HypothesisResult) error {
	// ========================================================================
	// HARD GATE 1: PhD Statistical Standard (p ≤ 0.001)
	// ========================================================================
	// Only hypotheses that achieve "PhD-level" statistical significance pass
	if hypothesis.TriGateResult.Confidence < 0.999 {
		log.Printf("STRATEGIC DISCARD: Hypothesis %s failed Tri-Gate confidence (%.3f < 0.999)",
			hypothesis.ID, hypothesis.TriGateResult.Confidence)
		log.Printf("   📋 Rationale: %s", hypothesis.TriGateResult.Rationale)
		log.Printf("   🧪 Referee Results:")
		for i, result := range hypothesis.RefereeResults {
			status := "❌ FAILED"
			if result.Passed {
				status = "✅ PASSED"
			}
			log.Printf("      %d. %s %s (p=%.4f)", i+1, result.GateName, status, result.PValue)
			if !result.Passed && result.FailureReason != "" {
				log.Printf("         💥 Reason: %s", result.FailureReason)
			}
		}
		log.Printf("   📊 Final Confidence: %.1f%%", hypothesis.TriGateResult.Confidence*100)
		return nil // Silent discard - no database write for failures
	}

	// ========================================================================
	// HARD GATE 2: Economic Viability Check
	// ========================================================================
	// Calculate opportunity cost - the economic value of acting on this insight
	opportunityCost := sg.calculateOpportunityCost(hypothesis)

	// Minimum viable economic threshold ($1,000)
	minValueThreshold := 1000.0
	if opportunityCost < minValueThreshold {
		log.Printf("STRATEGIC DISCARD: Hypothesis %s below economic threshold ($%.2f < $%.2f)",
			hypothesis.ID, opportunityCost, minValueThreshold)
		log.Printf("   💰 Opportunity Cost Breakdown:")
		log.Printf("      • Confidence Contribution: $%.2f (%.3f × $1000)", hypothesis.TriGateResult.Confidence*1000, hypothesis.TriGateResult.Confidence)
		log.Printf("      • Complexity Multiplier: %.2fx (based on hypothesis length: %d chars)", math.Min(float64(len(hypothesis.ScienceHypothesis))/500.0, 3.0), len(hypothesis.ScienceHypothesis))
		log.Printf("      • Domain Bonus: Applied (causal/predictive/segmentation keywords)")
		log.Printf("      • Final Value: $%.2f", opportunityCost)
		log.Printf("   📊 Minimum Threshold: $%.2f", minValueThreshold)
		return nil // Economically unviable - discard
	}

	// ========================================================================
	// LAYER 3: Persist the Universal Law
	// ========================================================================
	// Only the most rigorous, economically valuable hypotheses reach this point

	// Get default user
	user, err := sg.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get user for persistence: %w", err)
	}

	// Parse session ID
	sessionUUID, err := uuid.Parse(hypothesis.SessionID)
	if err != nil {
		log.Printf("⚠️ Invalid session ID in hypothesis %s: %v", hypothesis.ID, err)
		// Continue with nil UUID - session relationship is optional
		sessionUUID = uuid.Nil
	}

	// Create the validated law record
	validatedLaw := &models.HypothesisResult{
		ID:                  hypothesis.ID,
		SessionID:           hypothesis.SessionID, // Keep as string for compatibility
		BusinessHypothesis:  hypothesis.BusinessHypothesis,
		ScienceHypothesis:   hypothesis.ScienceHypothesis,
		RefereeResults:      hypothesis.RefereeResults,
		TriGateResult:       hypothesis.TriGateResult,
		Passed:              true, // Only successful hypotheses reach here
		ValidationTimestamp: hypothesis.ValidationTimestamp,
		StandardsVersion:    hypothesis.StandardsVersion,
		ExecutionMetadata: map[string]interface{}{
			"opportunity_cost":   opportunityCost,
			"leverage_score":     sg.calculateLeverageScore(hypothesis),
			"intervention_price": sg.calculateInterventionPrice(hypothesis),
			"confidence_score":   hypothesis.TriGateResult.Confidence,
			"persistence_tier":   "universal_law", // Highest tier
			"gateway_version":    "success_only_v1",
		},
	}

	// Persist to database
	if err := sg.hypothesisRepo.SaveHypothesis(ctx, user.ID, sessionUUID, validatedLaw); err != nil {
		return fmt.Errorf("failed to persist universal law %s: %w", hypothesis.ID, err)
	}

	// Log the successful persistence
	leverageScore := sg.calculateLeverageScore(hypothesis)
	log.Printf("🎯 UNIVERSAL LAW PERSISTED: %s", hypothesis.ID)
	log.Printf("   📊 Confidence: %.3f%%", hypothesis.TriGateResult.Confidence*100)
	log.Printf("   💰 Opportunity Cost: $%.2f", opportunityCost)
	log.Printf("   📈 Leverage Score: %.2f", leverageScore)
	log.Printf("   🏛️ Status: Validated Universal Law (Tier 3)")

	return nil
}

// calculateOpportunityCost computes the economic value of acting on this hypothesis
// This implements the "Business-Aware Engineering" requirement
func (sg *SuccessGateway) calculateOpportunityCost(hypothesis *models.HypothesisResult) float64 {
	// Base calculation using confidence score and hypothesis complexity
	baseValue := hypothesis.TriGateResult.Confidence * 1000.0 // $0-1000 based on confidence

	// Complexity multiplier based on hypothesis length (proxy for implementation complexity)
	complexityMultiplier := math.Min(float64(len(hypothesis.ScienceHypothesis))/500.0, 3.0)
	baseValue *= complexityMultiplier

	// Domain-specific adjustments
	switch {
	case stringContains(hypothesis.ScienceHypothesis, "causal") || stringContains(hypothesis.ScienceHypothesis, "correlation"):
		baseValue *= 1.5 // Causality insights are more valuable
	case stringContains(hypothesis.ScienceHypothesis, "predict") || stringContains(hypothesis.ScienceHypothesis, "forecast"):
		baseValue *= 1.3 // Predictive insights have commercial value
	case stringContains(hypothesis.ScienceHypothesis, "segment") || stringContains(hypothesis.ScienceHypothesis, "cluster"):
		baseValue *= 1.2 // Segmentation insights enable targeting
	}

	return math.Max(baseValue, 100.0) // Minimum $100 value
}

// calculateLeverageScore computes the economic impact multiplier
func (sg *SuccessGateway) calculateLeverageScore(hypothesis *models.HypothesisResult) float64 {
	// Base score from confidence
	baseScore := hypothesis.TriGateResult.Confidence * 100

	// Bonus for strong causal relationships
	if stringContains(hypothesis.ScienceHypothesis, "strong correlation") ||
		stringContains(hypothesis.ScienceHypothesis, "significant effect") {
		baseScore *= 1.2
	}

	// Cap at reasonable maximum
	return math.Min(baseScore, 99.99)
}

// calculateInterventionPrice estimates the cost to act on this insight
func (sg *SuccessGateway) calculateInterventionPrice(hypothesis *models.HypothesisResult) float64 {
	// Base intervention cost
	baseCost := 100.0

	// Scale with hypothesis complexity
	if len(hypothesis.ScienceHypothesis) > 500 {
		baseCost *= 2.0 // Complex hypotheses require more implementation effort
	}

	// Add testing and validation costs
	baseCost += 50.0

	return baseCost
}

// stringContains is a helper function for string matching
func stringContains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// GetValidatedLaws retrieves all universal laws (successful hypotheses)
func (sg *SuccessGateway) GetValidatedLaws(ctx context.Context, limit int) ([]*models.HypothesisResult, error) {
	user, err := sg.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Get only passed hypotheses (universal laws)
	return sg.hypothesisRepo.ListByValidationState(ctx, user.ID, true, limit)
}

// GetEconomicStats returns economic impact statistics
func (sg *SuccessGateway) GetEconomicStats(ctx context.Context) (*EconomicStats, error) {
	user, err := sg.userRepo.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	stats, err := sg.hypothesisRepo.GetUserStats(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	// In a real implementation, we'd calculate these from stored economic metadata
	return &EconomicStats{
		TotalOpportunityCost: float64(stats.TotalHypotheses) * 500.0, // Rough estimate
		AverageLeverageScore: 75.0,                                   // Rough estimate
		ValidatedLawsCount:   stats.ValidatedCount,
		TotalValueCaptured:   float64(stats.ValidatedCount) * 1000.0, // Rough estimate
	}, nil
}

// EconomicStats represents economic impact metrics
type EconomicStats struct {
	TotalOpportunityCost float64 `json:"total_opportunity_cost"`
	AverageLeverageScore float64 `json:"average_leverage_score"`
	ValidatedLawsCount   int     `json:"validated_laws_count"`
	TotalValueCaptured   float64 `json:"total_value_captured"`
}
