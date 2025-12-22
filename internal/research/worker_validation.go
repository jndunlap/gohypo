package research

import (
	"context"
	"fmt"
	"log"
	"time"

	"gohypo/domain/core"
	"gohypo/internal/api"
	refereePkg "gohypo/internal/referee"
	"gohypo/models"
)

// executeEValueValidation performs e-value dynamic validation for a single hypothesis
func (rw *ResearchWorker) executeEValueValidation(ctx context.Context, sessionID string, directive models.ResearchDirectiveResponse) bool {
	return rw.executeEValueValidationWithEvidence(ctx, sessionID, directive, nil)
}

// executeEValueValidationWithEvidence performs e-value dynamic validation with optional discovery evidence
func (rw *ResearchWorker) executeEValueValidationWithEvidence(ctx context.Context, sessionID string, directive models.ResearchDirectiveResponse, discoveryEvidence []refereePkg.DiscoveryEvidence) bool {
	hypothesisID := directive.ID
	log.Printf("[ResearchWorker] Starting validation for hypothesis %s", hypothesisID)

	// Validate referee selection (allow any number including 0)
	if err := directive.RefereeGates.Validate(); err != nil {
		log.Printf("[ResearchWorker] ERROR: Invalid referee selection for hypothesis %s: %v", hypothesisID, err)
		rw.recordFailedHypothesis(ctx, sessionID, hypothesisID, fmt.Sprintf("Invalid referee selection: %v", err))
		return false
	}

	// Allow any number of referees for dynamic validation
	refereeCount := len(directive.RefereeGates.SelectedReferees)

	// If no referees selected, use simple acceptance (e-value dynamic validation allows this)
	if refereeCount == 0 {
		log.Printf("[ResearchWorker] Hypothesis %s accepted with no referees required", hypothesisID)
		return rw.acceptHypothesisWithEValue(ctx, sessionID, directive, []models.RefereeResult{}, 0)
	}

	// Load matrix data for the hypothesis variables
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	matrixBundle, err := rw.loadMatrixBundleForHypothesisWithContext(ctx, directive)
	if err != nil {
		log.Printf("[ResearchWorker] ERROR: Matrix loading failed for hypothesis %s: %v", hypothesisID, err)
		rw.recordFailedHypothesis(ctx, sessionID, hypothesisID, fmt.Sprintf("Matrix loading failed: %v", err))
		return false
	}

	// Execute referees dynamically (any number)
	refereeResults := make([]models.RefereeResult, 0, refereeCount)

	// Extract variable data once to get sample size
	xData, ok := matrixBundle.GetColumnData(core.VariableKey(directive.CauseKey))
	yData, ok2 := matrixBundle.GetColumnData(core.VariableKey(directive.EffectKey))
	sampleSize := 0
	if ok && ok2 && len(xData) > 0 {
		sampleSize = len(xData)
	}
	log.Printf("[ResearchWorker] 📏 Sample size for hypothesis %s: %d data points", hypothesisID, sampleSize)

	if !ok || !ok2 {
		log.Printf("[ResearchWorker] ERROR: Variables not found for hypothesis %s - cause: %s, effect: %s", hypothesisID, directive.CauseKey, directive.EffectKey)
		rw.recordFailedHypothesis(ctx, sessionID, hypothesisID, fmt.Sprintf("Variable data not found: cause=%s, effect=%s", directive.CauseKey, directive.EffectKey))
		return false
	}

	// Execute referees concurrently for dynamic validation
	log.Printf("[ResearchWorker] Executing %d referees for hypothesis %s", refereeCount, hypothesisID)

	type refereeJob struct {
		index    int
		name     string
		result   models.RefereeResult
		duration time.Duration
	}

	jobs := make(chan refereeJob, refereeCount)

	// Launch goroutines for each referee
	for i, refereeName := range directive.RefereeGates.SelectedReferees {
		go func(index int, name string) {
			jobStart := time.Now()
			refereeInstance, err := refereePkg.GetRefereeFactory(name)
			if err != nil {
				log.Printf("[ResearchWorker] ERROR: Cannot create referee %s for hypothesis %s: %v", name, hypothesisID, err)
				jobs <- refereeJob{
					index: index,
					name:  name,
					result: models.RefereeResult{
						GateName:      name,
						Passed:        false,
						Statistic:     0.0,
						PValue:        1.0,
						StandardUsed:  "Error during instantiation",
						FailureReason: fmt.Sprintf("Referee creation failed: %v", err),
					},
					duration: time.Since(jobStart),
				}
				return
			}

			// Execute referee - use AuditEvidence if discovery evidence is available
			var result models.RefereeResult
			if discoveryEvidence != nil && len(discoveryEvidence) > 0 {
				var relevantEvidence *refereePkg.DiscoveryEvidence
				for _, evidence := range discoveryEvidence {
					if evidence.CauseKey == directive.CauseKey && evidence.EffectKey == directive.EffectKey {
						relevantEvidence = &evidence
						break
					}
				}
				if relevantEvidence != nil {
					result = refereeInstance.AuditEvidence(*relevantEvidence, yData, nil)
				} else {
					result = refereeInstance.Execute(xData, yData, nil)
				}
			} else {
				result = refereeInstance.Execute(xData, yData, nil)
			}

			jobs <- refereeJob{
				index:    index,
				name:     name,
				result:   result,
				duration: time.Since(jobStart),
			}
		}(i, refereeName)
	}

	// Collect results and send real-time SSE updates
	refereeResults = make([]models.RefereeResult, refereeCount)
	for i := 0; i < refereeCount; i++ {
		job := <-jobs
		refereeResults[job.index] = job.result
		if !job.result.Passed {
			log.Printf("[ResearchWorker] Referee %s failed: %s", job.name, job.result.FailureReason)
		}

		// Send SSE update for each referee completion
		if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
			eventData := map[string]interface{}{
				"hypothesis_id":    hypothesisID,
				"referee_name":     job.name,
				"referee_index":    job.index,
				"passed":           job.result.Passed,
				"p_value":          job.result.PValue,
				"statistic":        job.result.Statistic,
				"standard_used":    job.result.StandardUsed,
				"duration_seconds": job.duration.Seconds(),
			}
			if !job.result.Passed {
				eventData["failure_reason"] = job.result.FailureReason
			}
			sseHub.Broadcast(api.ResearchEvent{
				SessionID:    sessionID,
				EventType:    "referee_completed",
				HypothesisID: hypothesisID,
				Progress:     50.0 + (float64(i+1)/float64(refereeCount))*40.0,
				Data:         eventData,
				Timestamp:    time.Now(),
			})
		}
	}

	// Simple e-value dynamic validation - calculate overall result
	return rw.acceptHypothesisWithEValue(ctx, sessionID, directive, refereeResults, sampleSize)
}

// acceptHypothesisWithEValue performs simple e-value dynamic validation
func (rw *ResearchWorker) acceptHypothesisWithEValue(ctx context.Context, sessionID string, directive models.ResearchDirectiveResponse, refereeResults []models.RefereeResult, sampleSize int) bool {
	id := directive.ID

	passedReferees := 0
	totalReferees := len(refereeResults)
	for _, result := range refereeResults {
		if result.Passed {
			passedReferees++
		}
	}

	overallPassed := passedReferees > 0 || totalReferees == 0

	confidence := 0.5
	if totalReferees > 0 {
		confidence = float64(passedReferees) / float64(totalReferees)
	}

	hypothesisResult := models.HypothesisResult{
		ID:                  id,
		SessionID:           sessionID,
		BusinessHypothesis:  directive.BusinessHypothesis,
		ScienceHypothesis:   directive.ScienceHypothesis,
		NullCase:            directive.NullCase,
		RefereeResults:      refereeResults,
		Passed:              overallPassed,
		ValidationTimestamp: time.Now(),
		StandardsVersion:    "1.0.0",
		ExecutionMetadata: map[string]interface{}{
			"validation_method": "e_value_dynamic",
			"passed_referees":   passedReferees,
			"total_referees":    totalReferees,
			"sample_size":       sampleSize,
		},
		PhaseEValues:     []float64{0.0, 0.0, 0.0},
		FeasibilityScore: 0.0,
		RiskLevel:        "low",
		DataTopology:     map[string]interface{}{},
		CurrentEValue:    confidence * 10.0,
		NormalizedEValue: confidence,
		Confidence:       confidence,
		Status:           "completed",
	}

	if err := rw.storage.SaveHypothesis(ctx, &hypothesisResult); err != nil {
		log.Printf("[ResearchWorker] ERROR: Failed to save hypothesis %s: %v", id, err)
		return false
	}

	log.Printf("[ResearchWorker] Hypothesis %s validation completed", id)
	return overallPassed
}

// recordFailedHypothesis creates a failed hypothesis result for error cases
func (rw *ResearchWorker) recordFailedHypothesis(ctx context.Context, sessionID, hypothesisID, failureReason string) {
	log.Printf("[ResearchWorker] Recording failed hypothesis %s: %s", hypothesisID, failureReason)

	failedResult := models.HypothesisResult{
		ID:                  hypothesisID,
		SessionID:           sessionID,
		BusinessHypothesis:  "Failed to validate - " + failureReason,
		ScienceHypothesis:   "Validation failed due to system error",
		RefereeResults:      []models.RefereeResult{}, // Empty results
		Passed:              false,
		ValidationTimestamp: time.Now(),
		StandardsVersion:    "1.0.0",
		ExecutionMetadata: map[string]interface{}{
			"session_id":      sessionID,
			"failure_reason":  failureReason,
			"validation_type": "failed",
			"error_category":  "system_error",
			"recovery_action": "marked_as_failed",
		},
		// Initialize required fields to prevent database errors
		PhaseEValues:     []float64{0.0, 0.0, 0.0}, // Initialize as array, not nil
		FeasibilityScore: 0.0,
		RiskLevel:        "low",
		DataTopology:     map[string]interface{}{},
		CurrentEValue:    0.0,
		NormalizedEValue: 0.0,
		Confidence:       0.0,
		Status:           "failed",
	}

	if err := rw.storage.SaveHypothesis(ctx, &failedResult); err != nil {
		log.Printf("[ResearchWorker] ERROR: Failed to save failed hypothesis %s: %v", hypothesisID, err)
	}

	log.Printf("[ResearchWorker] Error handling complete for hypothesis %s", hypothesisID)
}
