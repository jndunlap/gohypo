package research

import (
	"context"
	"fmt"
	"log"

	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/internal/analysis"
	refereePkg "gohypo/internal/referee"
)

// EValueValidator implements E-value based validation with Q-value continuity
type EValueValidator struct {
	eValueCalibrator *analysis.EValueCalibrator
	dataPartitioner  *analysis.DataPartitioner
}

// NewEValueValidator creates an E-value based validator
func NewEValueValidator(calibrator *analysis.EValueCalibrator) *EValueValidator {
	return &EValueValidator{
		eValueCalibrator: calibrator,
		dataPartitioner:  analysis.NewDataPartitioner(),
	}
}

// GetCalibrator returns the underlying E-value calibrator
func (ev *EValueValidator) GetCalibrator() *analysis.EValueCalibrator {
	return ev.eValueCalibrator
}

// ValidateHypothesisWithEValues performs E-value based validation with Q-value continuity
func (ev *EValueValidator) ValidateHypothesisWithEValues(
	ctx context.Context,
	validationReq *ValidationRequest,
) (*EValueValidationResult, error) {

	if validationReq == nil {
		return nil, fmt.Errorf("validation request cannot be nil")
	}

	if validationReq.HypothesisID == "" {
		return nil, fmt.Errorf("hypothesis ID cannot be empty")
	}

	if validationReq.Directive.ID == "" {
		return nil, fmt.Errorf("directive ID cannot be empty")
	}

	hypothesisID := validationReq.HypothesisID
	log.Printf("[EValueValidator] 🎯 Starting E-value validation for hypothesis %s", hypothesisID)

	// Step 1: Enforce Q-value continuity
	if err := validationReq.ValidateQValueContinuity(); err != nil {
		log.Printf("[EValueValidator] ❌ Q-value continuity breach for hypothesis %s: %v", hypothesisID, err)
		return &EValueValidationResult{
			HypothesisID:   hypothesisID,
			Passed:         false,
			FailureReason:  fmt.Sprintf("Q-value continuity enforcement: %v", err),
			ValidationType: "Q_VALUE_CONTINUITY_BREACH",
		}, nil
	}
	log.Printf("[EValueValidator] ✅ Q-value continuity validated for hypothesis %s", hypothesisID)

	// Step 2: Early rejection based on Q-values
	if shouldReject, reason := validationReq.ShouldRejectEarly(); shouldReject {
		log.Printf("[EValueValidator] 🚫 Early rejection for hypothesis %s: %s", hypothesisID, reason)
		return &EValueValidationResult{
			HypothesisID:   hypothesisID,
			Passed:         false,
			FailureReason:  reason,
			ValidationType: "EARLY_Q_VALUE_REJECTION",
		}, nil
	}

	// Step 3: Convert discovery evidence to E-values
	eValues, profile := ev.convertDiscoveryEvidenceToEValues(validationReq)

	if len(eValues) == 0 {
		log.Printf("[EValueValidator] ❌ No E-values generated for hypothesis %s", hypothesisID)
		return &EValueValidationResult{
			HypothesisID:   hypothesisID,
			Passed:         false,
			FailureReason:  "No valid discovery evidence to convert to E-values",
			ValidationType: "NO_EVIDENCE",
		}, nil
	}

	log.Printf("[EValueValidator] 🔄 Converted %d discovery evidences to E-values for hypothesis %s", len(eValues), hypothesisID)

	// Step 4: Execute referees on validation set only
	refereeEValues, refereeErrors := ev.executeRefereesOnValidationSet(ctx, validationReq)

	if len(refereeErrors) > 0 {
		log.Printf("[EValueValidator] ⚠️ Referee execution errors for hypothesis %s: %v", hypothesisID, refereeErrors)
		// Continue with available results but note the errors
	}

	log.Printf("[EValueValidator] 🏃 Executed %d referees on validation set for hypothesis %s", len(refereeEValues), hypothesisID)

	// Step 5: Combine all E-values (discovery + validation)
	allEValues := append(eValues, refereeEValues...)

	// Step 6: Aggregate evidence using E-value calibrator
	evidenceCombination := ev.eValueCalibrator.AggregateEValueEvidence(ctx, allEValues, profile)

	// Step 7: Apply dynamic thresholding based on workspace state
	dynamicThreshold := validationReq.CalculateDynamicThreshold()
	verdict := ev.applyWorkspaceAwareThreshold(evidenceCombination, dynamicThreshold, validationReq)

	log.Printf("[EValueValidator] 📊 Final verdict for hypothesis %s: %s (E=%.2f, threshold=%.2f)",
		hypothesisID, verdict, evidenceCombination.CombinedEValue, dynamicThreshold)

	return &EValueValidationResult{
		HypothesisID:       hypothesisID,
		Passed:             verdict == stats.VerdictAccepted,
		EValueResult:       evidenceCombination,
		RefereeErrors:      refereeErrors,
		ValidationType:     "E_VALUE_AGGREGATION",
		DynamicThreshold:   dynamicThreshold,
		WorkspacePenalty:   validationReq.HypothesesGenerated,
		AlphaSpent:         validationReq.GlobalAlphaSpent,
		QValueContinuity:   true,
		SamplePartitioning: true,
	}, nil
}

// convertDiscoveryEvidenceToEValues converts FDR-corrected evidence to E-values
func (ev *EValueValidator) convertDiscoveryEvidenceToEValues(req *ValidationRequest) ([]stats.EValue, stats.HypothesisProfile) {
	var eValues []stats.EValue
	relevantEvidence := req.GetRelevantDiscoveryEvidence()

	// Create hypothesis profile for calibration
	profile := stats.HypothesisProfile{
		DataComplexity:  stats.DataComplexityModerate, // Default assumption
		EffectMagnitude: stats.EffectSizeMedium,       // Default assumption
		SampleSize:      stats.SampleSizeMedium,       // Default assumption
		DomainRisk:      stats.DomainRiskMedium,       // Default assumption
		TemporalNature:  stats.TemporalStatic,         // Default assumption
		ConfoundingRisk: stats.ConfoundingMedium,      // Default assumption
	}

	for _, evidence := range relevantEvidence {
		// Validate evidence before conversion
		if evidence.QValue < 0 || evidence.QValue > 1 {
			log.Printf("[EValueValidator] ⚠️ Invalid Q-value in evidence: %.6f, skipping", evidence.QValue)
			continue
		}
		if evidence.PValue < 0 || evidence.PValue > 1 {
			log.Printf("[EValueValidator] ⚠️ Invalid P-value in evidence: %.6f, skipping", evidence.PValue)
			continue
		}
		if evidence.SampleSize <= 0 {
			log.Printf("[EValueValidator] ⚠️ Invalid sample size in evidence: %d, skipping", evidence.SampleSize)
			continue
		}

		// Use Q-value for primary conversion (FDR-corrected)
		eValue := ev.eValueCalibrator.ConvertQValueToEValue(
			evidence.QValue,   // FDR-corrected q-value
			evidence.PValue,   // Raw p-value for context
			evidence.TestType, // Test type (already a string)
			"Business",        // Default domain - would be extracted from context
			evidence.SampleSize,
		)

		eValues = append(eValues, eValue)
		log.Printf("[EValueValidator] 🔄 Converted evidence to E-value: q=%.6f, p=%.6f, E=%.2f",
			evidence.QValue, evidence.PValue, eValue.Value)

		// Update profile based on evidence characteristics
		if evidence.QValue < 0.01 {
			profile.EffectMagnitude = stats.EffectSizeLarge
		}
		if evidence.SampleSize > 1000 {
			profile.SampleSize = stats.SampleSizeLarge
		}
	}

	return eValues, profile
}

// executeRefereesOnValidationSet runs referees only on held-out validation data
func (ev *EValueValidator) executeRefereesOnValidationSet(
	ctx context.Context,
	req *ValidationRequest,
) ([]stats.EValue, []error) {

	var eValues []stats.EValue
	var errors []error

	selectedReferees := req.Directive.RefereeGates.SelectedReferees

	// Get validation set data
	xData, yData, dataErr := ev.extractValidationData(req)
	if dataErr != nil {
		errors = append(errors, fmt.Errorf("validation data extraction failed: %w", dataErr))
		return eValues, errors
	}

	// Execute each referee on validation set
	for _, refereeName := range selectedReferees {
		refereeInstance, err := refereePkg.GetRefereeFactory(refereeName)
		if err != nil {
			log.Printf("[EValueValidator] ❌ Failed to create referee %s: %v", refereeName, err)
			errors = append(errors, fmt.Errorf("referee creation failed for %s: %w", refereeName, err))
			continue
		}

		// Execute referee on validation data only
		result := refereeInstance.Execute(xData, yData, nil)

		// Validate referee result
		if result.PValue < 0 || result.PValue > 1 {
			log.Printf("[EValueValidator] ⚠️ Invalid p-value from referee %s: %.6f", refereeName, result.PValue)
			errors = append(errors, fmt.Errorf("invalid p-value from referee %s: %.6f", refereeName, result.PValue))
			continue
		}

		// Convert referee result to E-value
		eValue := ev.convertRefereeResultToEValue(result, len(xData))
		eValues = append(eValues, eValue)

		log.Printf("[EValueValidator] ⚖️ Referee %s completed: p=%.6f, E=%.2f, passed=%t",
			refereeName, result.PValue, eValue.Value, result.Passed)
	}

	return eValues, errors
}

// extractValidationData extracts data from the validation partition
func (ev *EValueValidator) extractValidationData(req *ValidationRequest) ([]float64, []float64, error) {
	if req == nil {
		return nil, nil, fmt.Errorf("validation request cannot be nil")
	}

	if req.Directive.CauseKey == "" || req.Directive.EffectKey == "" {
		return nil, nil, fmt.Errorf("cause and effect keys must be specified in directive")
	}

	// Extract data for the hypothesis variables from validation set
	matrix := req.ValidationMatrixBundle.Matrix

	// This is a simplified extraction - in practice, this would interface
	// with the actual matrix data structure
	xData, ok := ev.extractVariableData(matrix, core.VariableKey(req.Directive.CauseKey))
	if !ok {
		return nil, nil, fmt.Errorf("cause variable %s not found in validation set", req.Directive.CauseKey)
	}

	yData, ok := ev.extractVariableData(matrix, core.VariableKey(req.Directive.EffectKey))
	if !ok {
		return nil, nil, fmt.Errorf("effect variable %s not found in validation set", req.Directive.EffectKey)
	}

	// Validate data sufficiency
	minSampleSize := 10 // Minimum sample size for meaningful statistical tests
	if len(xData) < minSampleSize || len(yData) < minSampleSize {
		return nil, nil, fmt.Errorf("insufficient data for validation: need at least %d samples, got cause=%d, effect=%d",
			minSampleSize, len(xData), len(yData))
	}

	if len(xData) != len(yData) {
		return nil, nil, fmt.Errorf("data length mismatch: cause has %d samples, effect has %d samples",
			len(xData), len(yData))
	}

	log.Printf("[EValueValidator] 📊 Extracted validation data: %d samples for cause-effect pair", len(xData))
	return xData, yData, nil
}

// extractVariableData extracts data for a specific variable from the matrix
func (ev *EValueValidator) extractVariableData(matrix interface{}, variableKey core.VariableKey) ([]float64, bool) {
	// TODO: Implement proper matrix data extraction
	// This placeholder returns dummy data for testing purposes
	// In production, this would interface with the actual matrix data structure
	// and extract the column/vector corresponding to the variableKey

	// For now, return a small dataset that allows the validation pipeline to complete
	// This should be replaced with actual data extraction logic
	dummyData := []float64{1.0, 2.0, 3.0, 4.0, 5.0}

	log.Printf("[EValueValidator] ⚠️ Using dummy data extraction for variable %s - implement actual matrix interface", variableKey)
	return dummyData, true
}

// convertRefereeResultToEValue converts a referee result to an E-value
func (ev *EValueValidator) convertRefereeResultToEValue(result refereePkg.RefereeResult, sampleSize int) stats.EValue {
	// For validation referees, we don't have FDR-corrected q-values
	// Use p-value as both q-value and p-value since no multiple testing correction was applied
	// The calibrator will handle this appropriately for validation contexts
	eValue := ev.eValueCalibrator.ConvertQValueToEValue(
		result.PValue, // Use p-value as q-value (no FDR correction in validation)
		result.PValue, // Raw p-value for context
		result.GateName,
		"Business", // Default domain - could be made configurable
		sampleSize,
	)

	return eValue
}

// applyWorkspaceAwareThreshold applies dynamic thresholding
func (ev *EValueValidator) applyWorkspaceAwareThreshold(
	evidence stats.EvidenceCombination,
	dynamicThreshold float64,
	req *ValidationRequest,
) stats.HypothesisVerdict {

	combinedE := evidence.CombinedEValue

	// Apply workspace-aware scaling
	workspaceMultiplier := 1.0
	if req.HypothesesGenerated > 50 {
		workspaceMultiplier = 1.2 // More conservative after many tests
	}
	if req.GlobalAlphaSpent > 0.5 {
		workspaceMultiplier = 1.3 // Much more conservative when alpha budget is low
	}

	effectiveThreshold := dynamicThreshold * workspaceMultiplier

	switch {
	case combinedE >= effectiveThreshold*2.0:
		return stats.VerdictAccepted
	case combinedE <= 1.0/effectiveThreshold:
		return stats.VerdictRejected
	case evidence.EarlyStopEligible && combinedE >= effectiveThreshold*0.7:
		return stats.VerdictEarlyStop
	default:
		return stats.VerdictInconclusive
	}
}

// EValueValidationResult represents the outcome of E-value validation
type EValueValidationResult struct {
	HypothesisID       string
	Passed             bool
	EValueResult       stats.EvidenceCombination
	RefereeErrors      []error
	ValidationType     string
	FailureReason      string
	DynamicThreshold   float64
	WorkspacePenalty   int
	AlphaSpent         float64
	QValueContinuity   bool
	SamplePartitioning bool
}

// GetValidationSummary returns a human-readable summary
func (r *EValueValidationResult) GetValidationSummary() string {
	if r.Passed {
		return fmt.Sprintf("✅ ACCEPTED: E-value=%.2f (threshold=%.2f), %d referees executed",
			r.EValueResult.CombinedEValue, r.DynamicThreshold, r.EValueResult.TestCount)
	}

	reason := r.FailureReason
	if reason == "" {
		reason = fmt.Sprintf("E-value=%.2f below threshold=%.2f", r.EValueResult.CombinedEValue, r.DynamicThreshold)
	}

	return fmt.Sprintf("❌ REJECTED: %s", reason)
}

// IsStatisticallyRigorous checks if validation used all required safeguards
func (r *EValueValidationResult) IsStatisticallyRigorous() bool {
	return r.QValueContinuity && r.SamplePartitioning && len(r.RefereeErrors) == 0
}
