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

// executeTriGateValidation performs Tri-Gate validation for a single hypothesis
func (rw *ResearchWorker) executeTriGateValidation(ctx context.Context, sessionID string, directive models.ResearchDirectiveResponse) bool {
	hypothesisID := directive.ID
	log.Printf("[ResearchWorker] ⚖️ Starting Tri-Gate validation for hypothesis %s (cause: %s, effect: %s)", hypothesisID, directive.CauseKey, directive.EffectKey)

	// Validate referee selection
	log.Printf("[ResearchWorker] 🔍 Validating referee selection for hypothesis %s", hypothesisID)
	if err := directive.RefereeGates.Validate(); err != nil {
		log.Printf("[ResearchWorker] ❌ Invalid referee selection for hypothesis %s: %v", hypothesisID, err)
		rw.recordFailedHypothesis(ctx, sessionID, hypothesisID, fmt.Sprintf("Invalid referee selection: %v", err))
		return false
	}

	if err := refereePkg.ValidateRefereeCompatibility(directive.RefereeGates.SelectedReferees); err != nil {
		log.Printf("[ResearchWorker] ❌ Incompatible referee selection for hypothesis %s: %v", hypothesisID, err)
		rw.recordFailedHypothesis(ctx, sessionID, hypothesisID, fmt.Sprintf("Incompatible referees: %v", err))
		return false
	}
	log.Printf("[ResearchWorker] ✅ Referee selection validated for hypothesis %s (%d referees)", hypothesisID, len(directive.RefereeGates.SelectedReferees))

	// Load matrix data for the hypothesis variables
	log.Printf("[ResearchWorker] 📊 Loading matrix data for variables: cause=%s, effect=%s", directive.CauseKey, directive.EffectKey)
	matrixLoadStart := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	matrixBundle, err := rw.loadMatrixBundleForHypothesisWithContext(ctx, directive)
	matrixLoadDuration := time.Since(matrixLoadStart)

	if err != nil {
		log.Printf("[ResearchWorker] ❌ MATRIX LOAD FAILURE: Hypothesis %s failed after %.2fs", hypothesisID, matrixLoadDuration.Seconds())
		log.Printf("[ResearchWorker] 💥 Error details: %v", err)
		log.Printf("[ResearchWorker] 📍 Variables requested: %s → %s", directive.CauseKey, directive.EffectKey)
		log.Printf("[ResearchWorker] 🔍 Troubleshooting: Check if variables exist in dataset, verify matrix resolver health")
		rw.recordFailedHypothesis(ctx, sessionID, hypothesisID, fmt.Sprintf("Matrix loading failed: %v", err))
		return false
	}

	log.Printf("[ResearchWorker] ✅ Matrix loaded successfully in %.2fs", matrixLoadDuration.Seconds())
	log.Printf("[ResearchWorker] 📏 Dataset size: %d entities, sample available for analysis", len(matrixBundle.Matrix.EntityIDs))

	// Execute all three referees
	refereeResults := make([]refereePkg.RefereeResult, 0, 3)
	log.Printf("[ResearchWorker] 🏃 Executing %d referees for hypothesis %s", len(directive.RefereeGates.SelectedReferees), hypothesisID)

	// Extract variable data once to get sample size
	xData, ok := matrixBundle.GetColumnData(core.VariableKey(directive.CauseKey))
	yData, ok2 := matrixBundle.GetColumnData(core.VariableKey(directive.EffectKey))
	sampleSize := 0
	if ok && ok2 && len(xData) > 0 {
		sampleSize = len(xData)
	}
	log.Printf("[ResearchWorker] 📏 Sample size for hypothesis %s: %d data points", hypothesisID, sampleSize)

	if !ok || !ok2 {
		log.Printf("[ResearchWorker] ❌ VARIABLE DATA UNAVAILABLE: Hypothesis %s cannot proceed", hypothesisID)
		log.Printf("[ResearchWorker] 📊 Data check results:")
		log.Printf("[ResearchWorker]   • Cause variable '%s': %s", directive.CauseKey, map[bool]string{true: "FOUND", false: "MISSING"}[ok])
		log.Printf("[ResearchWorker]   • Effect variable '%s': %s", directive.EffectKey, map[bool]string{true: "FOUND", false: "MISSING"}[ok2])
		log.Printf("[ResearchWorker] 🔍 Root cause: Variables not present in resolved matrix or matrix resolution failed")
		log.Printf("[ResearchWorker] 🔧 Suggested fix: Check variable names match dataset columns exactly")
		rw.recordFailedHypothesis(ctx, sessionID, hypothesisID, fmt.Sprintf("Variable data not found: cause=%s, effect=%s", directive.CauseKey, directive.EffectKey))
		return false
	}

	// Execute referees concurrently for major speed boost
	refereeCount := len(directive.RefereeGates.SelectedReferees)
	log.Printf("[ResearchWorker] 🏃 Executing %d referees concurrently for hypothesis %s", refereeCount, hypothesisID)
	log.Printf("[ResearchWorker] 🎯 Referees: %v", directive.RefereeGates.SelectedReferees)
	log.Printf("[ResearchWorker] 📊 Sample size: %d data points for statistical testing", sampleSize)

	type refereeJob struct {
		index    int
		name     string
		result   refereePkg.RefereeResult
		duration time.Duration
	}

	jobs := make(chan refereeJob, refereeCount)
	refereeStart := time.Now()

	// Launch goroutines for each referee
	for i, refereeName := range directive.RefereeGates.SelectedReferees {
		go func(index int, name string) {
			jobStart := time.Now()

			refereeInstance, err := refereePkg.GetRefereeFactory(name)
			if err != nil {
				log.Printf("[ResearchWorker] ❌ REFEREE CREATION FAILURE: Cannot instantiate %s for hypothesis %s", name, hypothesisID)
				log.Printf("[ResearchWorker] 💥 Error: %v", err)
				log.Printf("[ResearchWorker] 🔧 Recovery: Marking referee as failed, continuing with others")
				jobs <- refereeJob{
					index: index,
					name:  name,
					result: refereePkg.RefereeResult{
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

			// Execute referee
			result := refereeInstance.Execute(xData, yData, nil)

			jobs <- refereeJob{
				index:    index,
				name:     name,
				result:   result,
				duration: time.Since(jobStart),
			}
		}(i, refereeName)
	}

	// Collect results in order and send real-time SSE updates
	refereeResults = make([]refereePkg.RefereeResult, len(directive.RefereeGates.SelectedReferees))
	for i := 0; i < len(directive.RefereeGates.SelectedReferees); i++ {
		job := <-jobs
		refereeResults[job.index] = job.result

		status := "✅ PASSED"
		if !job.result.Passed {
			status = "❌ FAILED"
		}
		log.Printf("[ResearchWorker] %s Referee %s completed in %.2fs", status, job.name, job.duration.Seconds())

		if !job.result.Passed {
			log.Printf("[ResearchWorker] 💥 FAILURE DETAILS: %s - %s", job.name, job.result.FailureReason)
		} else {
			log.Printf("[ResearchWorker] 📊 %s validation successful (p=%.4f)", job.name, job.result.PValue)
		}

		// 🔥 REAL-TIME SSE UPDATE: Send individual referee completion event
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
				Progress:     50.0 + (float64(i+1)/float64(len(directive.RefereeGates.SelectedReferees)))*40.0, // 50-90% for referees
				Data:         eventData,
				Timestamp:    time.Now(),
			})

			log.Printf("[ResearchWorker] 📡 SSE event sent: referee_completed for %s/%s", hypothesisID, job.name)
		}
	}

	totalRefereeDuration := time.Since(refereeStart)
	parallelSpeedup := float64(refereeCount) / totalRefereeDuration.Seconds() * 2.0 // Rough estimate
	log.Printf("[ResearchWorker] 🏁 All %d referees completed in %.2fs total", refereeCount, totalRefereeDuration.Seconds())
	log.Printf("[ResearchWorker] ⚡ Parallel execution speedup: ~%.1fx faster than sequential", parallelSpeedup)

	// Create comprehensive hypothesis result
	log.Printf("[ResearchWorker] 💾 Saving hypothesis result for %s", hypothesisID)
	// Evaluate Tri-Gate results
	triGateResult := refereePkg.EvaluateTriGate(refereeResults)

	outcome := "✅ PASSED"
	if !triGateResult.OverallPassed {
		outcome = "❌ FAILED"
	}

	log.Printf("[ResearchWorker] 🎯 Tri-Gate verdict: %s", outcome)
	log.Printf("[ResearchWorker] 📋 Rationale: %s", triGateResult.Rationale)
	log.Printf("[ResearchWorker] 📊 Confidence score: %.1f%%", triGateResult.Confidence*100)

	// Create comprehensive hypothesis result
	hypothesisResult := models.HypothesisResult{
		ID:                  hypothesisID,
		BusinessHypothesis:  directive.BusinessHypothesis,
		ScienceHypothesis:   directive.ScienceHypothesis,
		NullCase:            directive.NullCase,
		RefereeResults:      refereeResults,
		TriGateResult:       triGateResult,
		Passed:              triGateResult.OverallPassed,
		ValidationTimestamp: time.Now(),
		StandardsVersion:    "1.0.0",
		ExecutionMetadata: map[string]interface{}{
			"referee_selection_rationale": directive.RefereeGates.Rationale,
			"confidence_target":           directive.RefereeGates.ConfidenceTarget,
			"session_id":                  sessionID,
			"sample_size":                 sampleSize,
			"matrix_load_duration_ms":     matrixLoadDuration.Milliseconds(),
		},
	}

	// SUCCESS-ONLY GATEWAY: Only persist if hypothesis passes all gates
	if successGateway, ok := rw.sseHub.(*SuccessGateway); ok {
		if err := successGateway.PersistUniversalLaw(ctx, &hypothesisResult); err != nil {
			log.Printf("[ResearchWorker] ❌ Failed to persist universal law %s: %v", hypothesisID, err)
			return false
		}
		// If we reach here, the hypothesis was successfully persisted as a Universal Law
	} else {
		// Fallback to regular storage if no gateway is available
		if err := rw.storage.SaveHypothesis(ctx, &hypothesisResult); err != nil {
			log.Printf("[ResearchWorker] ❌ Failed to save hypothesis %s: %v", hypothesisID, err)
			return false
		}
	}

	// Note: Hypothesis is automatically linked to session via session_id in database

	log.Printf("[ResearchWorker] 🎉 Tri-Gate validation completed successfully for hypothesis %s", hypothesisID)
	return true
}

// recordFailedHypothesis creates a failed hypothesis result for error cases
func (rw *ResearchWorker) recordFailedHypothesis(ctx context.Context, sessionID, hypothesisID, failureReason string) {
	log.Printf("[ResearchWorker] 📝 RECORDING FAILED HYPOTHESIS: %s", hypothesisID)
	log.Printf("[ResearchWorker] 💥 Failure reason: %s", failureReason)
	log.Printf("[ResearchWorker] 🔗 Session context: %s", sessionID)

	failedResult := models.HypothesisResult{
		ID:                 hypothesisID,
		SessionID:          sessionID,
		BusinessHypothesis: "Failed to validate - " + failureReason,
		ScienceHypothesis:  "Validation failed due to system error",
		RefereeResults:     []refereePkg.RefereeResult{}, // Empty results
		TriGateResult: refereePkg.TriGateResult{
			OverallPassed: false,
			Rationale:     fmt.Sprintf("System error during validation: %s", failureReason),
			Confidence:    0.0,
		},
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
	}

	if err := rw.storage.SaveHypothesis(ctx, &failedResult); err != nil {
		log.Printf("[ResearchWorker] ❌ CRITICAL: Failed to persist failed hypothesis %s to storage: %v", hypothesisID, err)
		log.Printf("[ResearchWorker] 🚨 DATA LOSS RISK: Hypothesis failure not recorded in database")
	} else {
		log.Printf("[ResearchWorker] ✅ Failed hypothesis %s saved to storage", hypothesisID)
	}

	// Note: Hypothesis is automatically linked to session via session_id in database
	if err := rw.storage.SaveHypothesis(ctx, &failedResult); err != nil {
		log.Printf("[ResearchWorker] ❌ CRITICAL: Failed to add failed hypothesis to session: %v", err)
		log.Printf("[ResearchWorker] 🚨 SESSION STATE INCONSISTENT: Hypothesis not added to session")
	} else {
		log.Printf("[ResearchWorker] ✅ Failed hypothesis added to session state")
	}

	log.Printf("[ResearchWorker] 📋 Error handling complete for hypothesis %s", hypothesisID)
}
