package research

import (
	"fmt"

	"gohypo/domain/core"
	"gohypo/internal/analysis"
	refereePkg "gohypo/internal/referee"
	"gohypo/models"
)

// ValidationRequest represents a hypothesis validation request with Q-value continuity
type ValidationRequest struct {
	// Hypothesis identification
	SessionID    string
	HypothesisID string

	// Hypothesis content
	Directive models.ResearchDirectiveResponse

	// Q-Value Continuity: Pre-computed statistical evidence from discovery phase
	DiscoveryEvidences []refereePkg.DiscoveryEvidence

	// Data partitioning for validation
	ValidationMatrixBundle MatrixBundle // Held-out data for validation
	DiscoveryMatrixBundle  MatrixBundle // Original discovery data (for reference only)

	// E-Value calibration
	EValueCalibrator *analysis.EValueCalibrator

	// Workspace context for guardrails
	WorkspaceID         string
	HypothesesGenerated int     // For dynamic thresholding
	GlobalAlphaSpent    float64 // Remaining statistical significance budget
}

// DiscoveryEvidence uses the referee package definition for compatibility

// MatrixBundle represents partitioned dataset for validation
type MatrixBundle struct {
	Matrix          interface{} // The actual matrix data
	EntityIDs       []core.ID
	VariableKeys    []core.VariableKey
	IsValidationSet bool // true = held-out data, false = discovery data
}

// NewValidationRequest creates a validation request with Q-value continuity
func NewValidationRequest(
	sessionID string,
	directive models.ResearchDirectiveResponse,
	discoveryEvidences []refereePkg.DiscoveryEvidence,
	validationBundle MatrixBundle,
	discoveryBundle MatrixBundle,
	calibrator *analysis.EValueCalibrator,
) *ValidationRequest {
	return &ValidationRequest{
		SessionID:              sessionID,
		HypothesisID:           directive.ID,
		Directive:              directive,
		DiscoveryEvidences:     discoveryEvidences,
		ValidationMatrixBundle: validationBundle,
		DiscoveryMatrixBundle:  discoveryBundle,
		EValueCalibrator:       calibrator,
	}
}

// ValidateQValueContinuity ensures Q-values are properly integrated
func (vr *ValidationRequest) ValidateQValueContinuity() error {
	if len(vr.DiscoveryEvidences) == 0 {
		return NewValidationError("Q_VALUE_CONTINUITY_BREACH",
			"No discovery evidence provided - cannot enforce Q-value continuity")
	}

	// Ensure we have evidence for the hypothesis variables
	hasCauseEvidence := false
	hasEffectEvidence := false

	for _, evidence := range vr.DiscoveryEvidences {
		if evidence.CauseKey == vr.Directive.CauseKey {
			hasCauseEvidence = true
		}
		if evidence.EffectKey == vr.Directive.EffectKey {
			hasEffectEvidence = true
		}
	}

	if !hasCauseEvidence || !hasEffectEvidence {
		return NewValidationError("MISSING_DISCOVERY_EVIDENCE",
			"Discovery evidence missing for hypothesis variables - Q-value continuity cannot be enforced")
	}

	return nil
}

// GetRelevantDiscoveryEvidence returns FDR-corrected evidence for this hypothesis
func (vr *ValidationRequest) GetRelevantDiscoveryEvidence() []refereePkg.DiscoveryEvidence {
	var relevant []refereePkg.DiscoveryEvidence

	for _, evidence := range vr.DiscoveryEvidences {
		if evidence.CauseKey == vr.Directive.CauseKey &&
			evidence.EffectKey == vr.Directive.EffectKey {
			relevant = append(relevant, evidence)
		}
	}

	return relevant
}

// ShouldRejectEarly determines if hypothesis should be rejected before full validation
func (vr *ValidationRequest) ShouldRejectEarly() (bool, string) {
	relevantEvidence := vr.GetRelevantDiscoveryEvidence()

	if len(relevantEvidence) == 0 {
		return false, ""
	}

	// If any Q-value indicates likely noise, reject early
	for _, evidence := range relevantEvidence {
		if evidence.QValue > 0.10 { // q > 0.10 indicates likely noise
			return true, fmt.Sprintf("Discovery Q-value (%.4f) indicates likely noise - rejecting before validation",
				evidence.QValue)
		}
	}

	return false, ""
}

// CalculateDynamicThreshold computes E-value threshold based on workspace state
func (vr *ValidationRequest) CalculateDynamicThreshold() float64 {
	baseThreshold := 8.0 // Conservative default

	// Scale based on hypotheses generated (multiple testing penalty)
	hypothesisPenalty := 1.0 + float64(vr.HypothesesGenerated)/20.0
	baseThreshold *= hypothesisPenalty

	// Scale based on remaining alpha budget
	if vr.GlobalAlphaSpent > 0.8 {
		baseThreshold *= 1.5 // More conservative when alpha budget is low
	}

	return baseThreshold
}

// ValidationError represents validation-specific errors
type ValidationError struct {
	Code    string
	Message string
}

func (ve ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s", ve.Code, ve.Message)
}

func NewValidationError(code, message string) ValidationError {
	return ValidationError{Code: code, Message: message}
}
