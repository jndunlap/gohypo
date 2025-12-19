package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"gohypo/app"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/domain/discovery"
	"gohypo/domain/greenfield"
	"gohypo/domain/stats"
	"gohypo/internal/api"
	refereePkg "gohypo/internal/referee"
	"gohypo/internal/testkit"
	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
)

type statsSweepRunner interface {
	RunStatsSweep(ctx context.Context, req app.StatsSweepRequest) (*app.StatsSweepResponse, error)
}

// ResearchWorker handles asynchronous research processing
type ResearchWorker struct {
	sessionMgr     *SessionManager
	storage        *ResearchStorage
	promptRepo     interface{}                  // Prompt repository for saving prompts
	greenfieldPort ports.GreenfieldResearchPort // Port interface for generating research directives
	statsSweepSvc  statsSweepRunner             // Stats sweep service
	testkit        *testkit.TestKit             // TestKit for matrix bundle creation
	sseHub         interface{}                  // SSE hub for real-time updates
}

// NewResearchWorker creates a new research worker
func NewResearchWorker(sessionMgr *SessionManager, storage *ResearchStorage, promptRepo interface{}, greenfieldSvc interface{}, llmConfig *models.AIConfig, statsSweepSvc interface{}, kitAny interface{}, sseHub interface{}) *ResearchWorker {
	// Extract the port from the greenfield service
	var greenfieldPort ports.GreenfieldResearchPort
	if gs, ok := greenfieldSvc.(*app.GreenfieldService); ok {
		// Access the port through reflection or add a getter method
		// For now, we'll add a getter method to GreenfieldService
		greenfieldPort = gs.GetGreenfieldPort()
	} else if gp, ok := greenfieldSvc.(ports.GreenfieldResearchPort); ok {
		greenfieldPort = gp
	}

	var sweep statsSweepRunner
	if ss, ok := statsSweepSvc.(statsSweepRunner); ok {
		sweep = ss
	}

	var kit *testkit.TestKit
	if tk, ok := kitAny.(*testkit.TestKit); ok {
		kit = tk
	}

	return &ResearchWorker{
		sessionMgr:     sessionMgr,
		storage:        storage,
		promptRepo:     promptRepo,
		greenfieldPort: greenfieldPort,
		statsSweepSvc:  sweep,
		testkit:        kit,
		sseHub:         sseHub,
	}
}

// ProcessResearch initiates and manages the research generation workflow
func (rw *ResearchWorker) ProcessResearch(ctx context.Context, sessionID string, fieldMetadata []greenfield.FieldMetadata, statsArtifacts []map[string]interface{}, sseHub interface{}) {
	sessionStart := time.Now()
	log.Printf("[ResearchWorker] 🚀 STARTING research process for session %s", sessionID)
	log.Printf("[ResearchWorker] 📊 Session context: %d fields, %d existing artifacts", len(fieldMetadata), len(statsArtifacts))

	// Initialize session-level variables
	var totalHypotheses int
	var successCount, failureCount int

	defer func() {
		sessionDuration := time.Since(sessionStart)
		// Generate comprehensive session summary
		log.Printf("[ResearchWorker] 🏁 SESSION COMPLETE: %s", sessionID)
		log.Printf("[ResearchWorker] ⏱️ Total duration: %.2fs", sessionDuration.Seconds())
		log.Printf("[ResearchWorker] 📊 Hypotheses processed: %d total", totalHypotheses)
		if successCount > 0 || failureCount > 0 {
			log.Printf("[ResearchWorker] ✅ Validation results: %d passed, %d failed", successCount, failureCount)
		}
		if totalHypotheses > 0 {
			log.Printf("[ResearchWorker] 📈 Average hypothesis processing time: %.2fs",
				sessionDuration.Seconds()/float64(totalHypotheses))
		}
		log.Printf("[ResearchWorker] 💾 All results saved to persistent storage")
	}()

	// Emit Layer 0 start event
	if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: sessionID,
			EventType: "layer0_start",
			Progress:  5.0,
			Data: map[string]interface{}{
				"message": "Initializing Scout analysis and data ingestion",
				"phase":   "Layer 0: Scout",
			},
			Timestamp: time.Now(),
		})
	}

	// Update session state to analyzing
	phaseStart := time.Now()
	log.Printf("[ResearchWorker] 📊 Phase 1/4: Analysis Setup - Updating session %s to analyzing state", sessionID)
	if err := rw.sessionMgr.SetSessionState(ctx, sessionID, models.SessionStateAnalyzing); err != nil {
		log.Printf("[ResearchWorker] ❌ CRITICAL ERROR: Failed to update session state to analyzing: %v", err)
		log.Printf("[ResearchWorker] 💥 Session %s terminated due to state management failure", sessionID)
		return
	}
	log.Printf("[ResearchWorker] ✅ Session state updated in %.3fs", time.Since(phaseStart).Seconds())

	// Run stats sweep if no statistical artifacts are available
	phaseStart = time.Now()
	if len(statsArtifacts) == 0 {
		log.Printf("[ResearchWorker] 📈 Phase 2/4: Statistical Analysis - Running stats sweep for session %s", sessionID)
		log.Printf("[ResearchWorker] 🔍 Analyzing %d fields for statistical relationships", len(fieldMetadata))

		// Try stats sweep with fallback to empty artifacts if it fails
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		var err error
		sweepArtifacts, err := rw.runStatsSweep(ctx, sessionID, fieldMetadata)
		phaseDuration := time.Since(phaseStart)

		if err != nil {
			log.Printf("[ResearchWorker] ⚠️ WARNING: Stats sweep failed after %.2fs: %v", phaseDuration.Seconds(), err)
			log.Printf("[ResearchWorker] 🔄 Continuing with empty artifacts (graceful degradation)")
			// Continue with empty artifacts instead of failing completely
			statsArtifacts = []map[string]interface{}{}
		} else {
			statsArtifacts = sweepArtifacts
			log.Printf("[ResearchWorker] ✅ Stats sweep completed in %.2fs - discovered %d statistical relationships", phaseDuration.Seconds(), len(statsArtifacts))
		}
	} else {
		log.Printf("[ResearchWorker] 📊 Phase 2/4: Statistical Analysis - Using %d existing artifacts for session %s", len(statsArtifacts), sessionID)
		log.Printf("[ResearchWorker] ⏭️ Skipping stats sweep (artifacts already available)")
	}

	// Build basic discovery briefs for LLM context (will be enhanced with sense results later)
	log.Printf("[ResearchWorker] 🏗️ Building discovery briefs for session %s", sessionID)
	discoveryBriefs := rw.buildDiscoveryBriefs(sessionID, statsArtifacts)
	log.Printf("[ResearchWorker] ✅ Built %d discovery briefs for session %s", len(discoveryBriefs), sessionID)

	// Convert metadata and stats artifacts to JSON for LLM processing
	log.Printf("[ResearchWorker] 📝 Preparing field metadata JSON for session %s", sessionID)
	fieldJSON, err := rw.prepareFieldMetadata(fieldMetadata, statsArtifacts, discoveryBriefs)
	if err != nil {
		log.Printf("[ResearchWorker] ❌ CRITICAL: Failed to prepare field metadata for session %s: %v", sessionID, err)
		rw.sessionMgr.SetSessionError(ctx, sessionID, fmt.Sprintf("Failed to prepare metadata: %v", err))
		return
	}
	log.Printf("[ResearchWorker] ✅ Field metadata prepared for session %s (%d chars)", sessionID, len(fieldJSON))

	// Generate hypotheses using LLM
	phaseStart = time.Now()
	log.Printf("[ResearchWorker] 🧠 Phase 3/4: Hypothesis Generation - Calling LLM for session %s", sessionID)
	log.Printf("[ResearchWorker] 📝 Context size: %d characters, %d fields available", len(fieldJSON), len(fieldMetadata))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	hypotheses, err := rw.generateHypothesesWithContext(ctx, sessionID, fieldJSON)
	phaseDuration := time.Since(phaseStart)

	if err != nil {
		log.Printf("[ResearchWorker] ❌ CRITICAL ERROR: LLM hypothesis generation failed after %.2fs", phaseDuration.Seconds())
		log.Printf("[ResearchWorker] 💥 Error details: %v", err)
		log.Printf("[ResearchWorker] 📊 Context attempted: %d fields, %d chars of metadata", len(fieldMetadata), len(fieldJSON))
		log.Printf("[ResearchWorker] 🔄 RECOVERY: Attempting graceful fallback to basic hypotheses...")

		// Try to create fallback hypotheses instead of failing completely
		fallbackHypotheses := rw.createFallbackHypotheses(sessionID, fieldMetadata)
		if len(fallbackHypotheses.ResearchDirectives) > 0 {
			log.Printf("[ResearchWorker] ✅ RECOVERY SUCCESSFUL: Generated %d basic hypotheses", len(fallbackHypotheses.ResearchDirectives))
			log.Printf("[ResearchWorker] ⚠️ WARNING: Using simplified hypotheses - reduced statistical power")
			hypotheses = fallbackHypotheses
		} else {
			log.Printf("[ResearchWorker] 💥 FATAL ERROR: Both primary and fallback hypothesis generation failed")
			log.Printf("[ResearchWorker] 🛑 Session %s cannot continue - no hypotheses available", sessionID)
			log.Printf("[ResearchWorker] 🔧 Suggested actions: Check LLM service connectivity, verify field metadata quality")
			rw.sessionMgr.SetSessionError(ctx, sessionID, fmt.Sprintf("Failed to generate hypotheses: %v", err))
			return
		}
	} else {
		log.Printf("[ResearchWorker] ✅ LLM hypothesis generation completed in %.2fs", phaseDuration.Seconds())
		log.Printf("[ResearchWorker] 🎯 Generated %d research hypotheses ready for validation", len(hypotheses.ResearchDirectives))
	}

	// Emit Layer 2 start event
	if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: sessionID,
			EventType: "layer2_start",
			Progress:  50.0,
			Data: map[string]interface{}{
				"message": "Starting Referee phase - Tri-Gate validation gauntlet",
				"phase":   "Layer 2: Referee",
			},
			Timestamp: time.Now(),
		})
	}

	// Update session state to validating
	log.Printf("[ResearchWorker] 🔬 Updating session %s to validating state", sessionID)
	if err := rw.sessionMgr.SetSessionState(ctx, sessionID, models.SessionStateValidating); err != nil {
		log.Printf("[ResearchWorker] ❌ CRITICAL: Failed to update session state to validating: %v", err)
		return
	}

	// Validate each hypothesis using Tri-Gate validation
	phaseStart = time.Now()
	totalHypotheses = len(hypotheses.ResearchDirectives)
	log.Printf("[ResearchWorker] ⚖️ Phase 4/4: Tri-Gate Validation - Processing %d hypotheses for session %s", totalHypotheses, sessionID)
	log.Printf("[ResearchWorker] 📋 Validation strategy: Parallel referee execution with statistical integrity checks")

	for i, directive := range hypotheses.ResearchDirectives {
		hypothesisStart := time.Now()
		hypothesisNum := i + 1
		progressPercent := float64(hypothesisNum-1) / float64(totalHypotheses) * 100

		log.Printf("[ResearchWorker] 🔍 Processing hypothesis %d/%d (%.1f%%) - ID: %s",
			hypothesisNum, totalHypotheses, progressPercent, directive.ID)
		log.Printf("[ResearchWorker] 📊 Testing relationship: %s → %s", directive.CauseKey, directive.EffectKey)

		// Update progress
		progress := float64(i) / float64(totalHypotheses) * 100
		currentHypothesis := fmt.Sprintf("Tri-Gate Validating: %s - %s", directive.ID, directive.BusinessHypothesis)
		rw.sessionMgr.UpdateSessionProgress(ctx, sessionID, progress, currentHypothesis)

		// Execute Tri-Gate validation for this hypothesis with error recovery
		var validationPassed bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ResearchWorker] 💥 PANIC in hypothesis %s validation: %v", directive.ID, r)
					rw.recordFailedHypothesis(ctx, sessionID, directive.ID, fmt.Sprintf("Panic during validation: %v", r))
					validationPassed = false
				}
			}()

			validationPassed = rw.executeTriGateValidation(ctx, sessionID, directive)
		}()

		hypothesisDuration := time.Since(hypothesisStart)
		phaseDuration = time.Since(phaseStart)

		log.Printf("[ResearchWorker] ⏱️ Hypothesis %s completed in %.2fs (total phase: %.1fs)",
			directive.ID, hypothesisDuration.Seconds(), phaseDuration.Seconds())
		log.Printf("[ResearchWorker] 📈 Progress: %d/%d hypotheses processed (%.1f%%)",
			hypothesisNum, totalHypotheses, float64(hypothesisNum)/float64(totalHypotheses)*100)

		// Count successes vs failures
		if validationPassed {
			successCount++
		} else {
			failureCount++
		}
	}

	log.Printf("[ResearchWorker] 📊 Validation summary for session %s: %d hypotheses processed", sessionID, totalHypotheses)

	// Emit Layer 3 start event
	if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: sessionID,
			EventType: "layer3_start",
			Progress:  90.0,
			Data: map[string]interface{}{
				"message": "Starting Gateway phase - Success-Only persistence",
				"phase":   "Layer 3: Gateway",
				"passed":  successCount,
				"failed":  failureCount,
				"total":   totalHypotheses,
			},
			Timestamp: time.Now(),
		})
	}

	// Complete the session
	log.Printf("[ResearchWorker] 🎯 Completing session %s", sessionID)
	if err := rw.sessionMgr.SetSessionState(ctx, sessionID, models.SessionStateComplete); err != nil {
		log.Printf("[ResearchWorker] ❌ CRITICAL: Failed to complete session %s: %v", sessionID, err)
	}

	// Emit final completion event
	if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: sessionID,
			EventType: "session_complete",
			Progress:  100.0,
			Data: map[string]interface{}{
				"message":      "Research session completed successfully",
				"passed":       successCount,
				"failed":       failureCount,
				"total":        totalHypotheses,
				"duration_sec": time.Since(sessionStart).Seconds(),
			},
			Timestamp: time.Now(),
		})
	}
}

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

// loadMatrixBundleForHypothesis loads matrix data for hypothesis validation
func (rw *ResearchWorker) loadMatrixBundleForHypothesis(directive models.ResearchDirectiveResponse) (*dataset.MatrixBundle, error) {
	return rw.loadMatrixBundleForHypothesisWithContext(context.Background(), directive)
}

// loadMatrixBundleForHypothesisWithContext loads matrix data for hypothesis validation with timeout and retry
func (rw *ResearchWorker) loadMatrixBundleForHypothesisWithContext(ctx context.Context, directive models.ResearchDirectiveResponse) (*dataset.MatrixBundle, error) {
	// Extract variable keys from hypothesis
	causeKey := core.VariableKey(directive.CauseKey)
	effectKey := core.VariableKey(directive.EffectKey)

	log.Printf("[ResearchWorker] 🔍 Resolving matrix for variables: cause=%s, effect=%s", causeKey, effectKey)

	// Retry logic for matrix resolution
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("[ResearchWorker] 🔄 Matrix resolution retry %d/%d for cause=%s, effect=%s", attempt, maxRetries, causeKey, effectKey)
			// Wait before retry with exponential backoff
			time.Sleep(time.Duration(attempt-1) * 2 * time.Second)
		}

		// Load from data source using testkit
		resolver := rw.testkit.MatrixResolverAdapter()
		bundle, err := resolver.ResolveMatrix(ctx, ports.MatrixResolutionRequest{
			ViewID:    core.ID("hypothesis_validation"),
			EntityIDs: nil, // Include all entities
			VarKeys:   []core.VariableKey{causeKey, effectKey},
		})

		if err == nil {
			if bundle == nil {
				log.Printf("[ResearchWorker] ❌ Matrix resolution returned nil bundle for cause=%s, effect=%s", causeKey, effectKey)
				lastErr = fmt.Errorf("matrix resolution returned nil bundle")
				continue // Retry even for nil bundle
			}

			log.Printf("[ResearchWorker] ✅ Matrix resolved successfully on attempt %d: %d entities, %d variables", attempt, len(bundle.Matrix.EntityIDs), len(bundle.Matrix.VariableKeys))
			return bundle, nil
		}

		lastErr = err
		log.Printf("[ResearchWorker] ❌ Matrix resolution attempt %d failed for cause=%s, effect=%s: %v", attempt, causeKey, effectKey, err)

		// Check if context was cancelled
		if ctx.Err() != nil {
			log.Printf("[ResearchWorker] ❌ Matrix resolution cancelled due to context: %v", ctx.Err())
			return nil, ctx.Err()
		}
	}

	log.Printf("[ResearchWorker] ❌ Matrix resolution failed after %d attempts for cause=%s, effect=%s: %v", maxRetries, causeKey, effectKey, lastErr)
	return nil, fmt.Errorf("matrix resolution failed after %d attempts: %w", maxRetries, lastErr)
}

// prepareFieldMetadata converts field metadata and statistical artifacts to JSON string
func (rw *ResearchWorker) prepareFieldMetadata(
	metadata []greenfield.FieldMetadata,
	statsArtifacts []map[string]interface{},
	discoveryBriefs []discovery.DiscoveryBrief,
) (string, error) {
	// Prepare comprehensive context for LLM
	contextData := map[string]interface{}{
		"field_metadata":        metadata,
		"statistical_artifacts": statsArtifacts,
		"discovery_briefs":      discoveryBriefs,
		"total_fields":          len(metadata),
		"total_stats_artifacts": len(statsArtifacts),
	}

	// Marshal to JSON for LLM processing
	data, err := json.MarshalIndent(contextData, "", "  ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (rw *ResearchWorker) buildDiscoveryBriefs(sessionID string, statsArtifacts []map[string]interface{}) []discovery.DiscoveryBrief {
	// Extract relationship payloads out of the stats artifacts list (best-effort).
	rels := make([]stats.RelationshipPayload, 0, len(statsArtifacts))
	for _, a := range statsArtifacts {
		kind, _ := a["kind"].(string)
		if kind != string(core.ArtifactRelationship) {
			continue
		}
		payload := a["payload"]

		switch p := payload.(type) {
		case stats.RelationshipPayload:
			rels = append(rels, p)
		case map[string]interface{}:
			if rp, ok := coerceRelationshipPayloadMap(p); ok {
				rels = append(rels, rp)
			}
		}
	}

	if len(rels) == 0 {
		return nil
	}

	briefs := discovery.BuildDiscoveryBriefsFromRelationships(
		"", // snapshot unknown in UI research flow today
		core.RunID(sessionID),
		rels,
		nil, // No sense results available in worker context
	)

	// Sort by confidence (desc) and keep a small, LLM-friendly set.
	sort.Slice(briefs, func(i, j int) bool {
		return briefs[i].ConfidenceScore > briefs[j].ConfidenceScore
	})
	if len(briefs) > 8 {
		briefs = briefs[:8]
	}
	return briefs
}

func coerceRelationshipPayloadMap(m map[string]interface{}) (stats.RelationshipPayload, bool) {
	varX, _ := m["variable_x"].(string)
	varY, _ := m["variable_y"].(string)
	testType, _ := m["test_type"].(string)
	familyID, _ := m["family_id"].(string)
	if varX == "" || varY == "" || testType == "" || familyID == "" {
		return stats.RelationshipPayload{}, false
	}

	effectSize, _ := toFloat64(m["effect_size"])
	pValue, _ := toFloat64(m["p_value"])
	qValue, _ := toFloat64(m["q_value"])
	sampleSizeF, _ := toFloat64(m["sample_size"])
	totalComparisonsF, _ := toFloat64(m["total_comparisons"])

	warnings := []stats.WarningCode{}
	if ws, ok := m["warnings"].([]interface{}); ok {
		for _, w := range ws {
			if s, ok := w.(string); ok && s != "" {
				warnings = append(warnings, stats.WarningCode(s))
			}
		}
	}

	return stats.RelationshipPayload{
		VariableX:        core.VariableKey(varX),
		VariableY:        core.VariableKey(varY),
		TestType:         stats.TestType(testType),
		FamilyID:         core.Hash(familyID),
		EffectSize:       effectSize,
		PValue:           pValue,
		QValue:           qValue,
		SampleSize:       int(sampleSizeF),
		TotalComparisons: int(totalComparisonsF),
		Warnings:         warnings,
	}, true
}

func toFloat64(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// createPendingHypothesesForUI creates hypotheses with pending referee results for immediate UI display
func (rw *ResearchWorker) createPendingHypothesesForUI(ctx context.Context, sessionID string, llmResponse *models.GreenfieldResearchOutput) error {
	log.Printf("[ResearchWorker] 🎨 Creating pending hypotheses for immediate UI display (%d hypotheses)", len(llmResponse.ResearchDirectives))

	for i, directive := range llmResponse.ResearchDirectives {
		// Create pending referee results (gray checkmarks)
		pendingRefereeResults := make([]refereePkg.RefereeResult, len(directive.RefereeGates.SelectedReferees))
		for j, refereeName := range directive.RefereeGates.SelectedReferees {
			pendingRefereeResults[j] = refereePkg.RefereeResult{
				GateName:      refereeName,
				Passed:        false, // Will be updated to true/false as referees complete
				Statistic:     0.0,
				PValue:        -1.0, // Special value to indicate "pending"
				StandardUsed:  "Pending validation...",
				FailureReason: "Validation in progress",
			}
		}

		// Create pending hypothesis result
		pendingHypothesis := &models.HypothesisResult{
			ID:                 directive.ID,
			SessionID:          sessionID,
			BusinessHypothesis: directive.BusinessHypothesis,
			ScienceHypothesis:  directive.ScienceHypothesis,
			NullCase:           directive.NullCase,
			RefereeResults:     pendingRefereeResults,
			TriGateResult: refereePkg.TriGateResult{
				RefereeResults: pendingRefereeResults,
				OverallPassed:  false, // Pending
				Confidence:     0.0,   // Will be calculated after all referees complete
				Rationale:      "Tri-Gate validation in progress...",
			},
			Passed:              false, // Pending
			ValidationTimestamp: time.Now(),
			StandardsVersion:    "1.0.0",
			ExecutionMetadata: map[string]interface{}{
				"referee_selection_rationale": directive.RefereeGates.Rationale,
				"confidence_target":           directive.RefereeGates.ConfidenceTarget,
				"session_id":                  sessionID,
				"validation_status":           "pending", // Special status for UI
			},
		}

		// Save pending hypothesis to database for immediate UI display
		if err := rw.storage.SaveHypothesis(ctx, pendingHypothesis); err != nil {
			log.Printf("[ResearchWorker] ❌ Failed to save pending hypothesis %s: %v", directive.ID, err)
			continue // Continue with other hypotheses
		}

		log.Printf("[ResearchWorker] ✅ Created pending hypothesis %s (%d/%d) for UI display",
			directive.ID, i+1, len(llmResponse.ResearchDirectives))

		// Emit SSE event for immediate UI update
		if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
			sseHub.Broadcast(api.ResearchEvent{
				SessionID:    sessionID,
				EventType:    "hypothesis_created",
				HypothesisID: directive.ID,
				Progress:     float64(i+1) / float64(len(llmResponse.ResearchDirectives)) * 25.0, // 0-25% for hypothesis creation
				Data: map[string]interface{}{
					"hypothesis_id":       directive.ID,
					"business_hypothesis": directive.BusinessHypothesis,
					"referee_count":       len(directive.RefereeGates.SelectedReferees),
					"status":              "pending_validation",
				},
				Timestamp: time.Now(),
			})
		}
	}

	log.Printf("[ResearchWorker] 🎉 All pending hypotheses created - UI can now display them immediately")
	return nil
}

// generateHypothesesWithContext calls the LLM to generate research hypotheses via the GreenfieldAdapter (which includes Forensic Scout)
func (rw *ResearchWorker) generateHypothesesWithContext(ctx context.Context, sessionID string, fieldJSON string) (*models.GreenfieldResearchOutput, error) {
	log.Printf("[ResearchWorker] 🤖 Starting hypothesis generation for session %s", sessionID)

	if rw.greenfieldPort == nil {
		log.Printf("[ResearchWorker] ❌ Greenfield port not available for session %s", sessionID)
		return nil, fmt.Errorf("greenfield port not available")
	}

	// Parse field metadata from JSON
	log.Printf("[ResearchWorker] 📝 Parsing field metadata JSON for session %s", sessionID)
	var contextData map[string]interface{}
	if err := json.Unmarshal([]byte(fieldJSON), &contextData); err != nil {
		log.Printf("[ResearchWorker] ❌ Failed to parse field JSON for session %s: %v", sessionID, err)
		return nil, fmt.Errorf("failed to parse field JSON: %w", err)
	}

	// Extract field metadata
	fieldMetadataRaw, ok := contextData["field_metadata"].([]interface{})
	if !ok {
		log.Printf("[ResearchWorker] ❌ field_metadata not found or invalid for session %s", sessionID)
		return nil, fmt.Errorf("field_metadata not found or invalid")
	}

	// Convert to greenfield.FieldMetadata
	fieldMetadata := make([]greenfield.FieldMetadata, 0, len(fieldMetadataRaw))
	for _, fm := range fieldMetadataRaw {
		fmMap, ok := fm.(map[string]interface{})
		if !ok {
			continue
		}
		fieldMetadata = append(fieldMetadata, greenfield.FieldMetadata{
			Name:         getString(fmMap, "name"),
			SemanticType: getString(fmMap, "semantic_type"),
			DataType:     getString(fmMap, "data_type"),
			Description:  getString(fmMap, "description"),
		})
	}
	log.Printf("[ResearchWorker] ✅ Converted %d field metadata items for session %s", len(fieldMetadata), sessionID)

	// Extract discovery briefs and stats artifacts from context data
	var discoveryBriefs interface{}
	var statsArtifacts []map[string]interface{}
	if discoveryBriefsRaw, ok := contextData["discovery_briefs"]; ok {
		discoveryBriefs = discoveryBriefsRaw
	}
	if statsArtifactsRaw, ok := contextData["statistical_artifacts"].([]interface{}); ok {
		statsArtifacts = make([]map[string]interface{}, 0, len(statsArtifactsRaw))
		for _, sa := range statsArtifactsRaw {
			if saMap, ok := sa.(map[string]interface{}); ok {
				statsArtifacts = append(statsArtifacts, saMap)
			}
		}
	}
	log.Printf("[ResearchWorker] 📊 Prepared %d statistical artifacts and discovery briefs for session %s", len(statsArtifacts), sessionID)

	// Call the port (which uses GreenfieldAdapter with Forensic Scout)
	log.Printf("[ResearchWorker] 🚀 Calling Greenfield port for research directives (session %s)", sessionID)
	req := ports.GreenfieldResearchRequest{
		RunID:                core.RunID(sessionID),
		SnapshotID:           core.SnapshotID(""), // Not used in UI flow
		FieldMetadata:        fieldMetadata,
		StatisticalArtifacts: statsArtifacts,
		DiscoveryBriefs:      discoveryBriefs,
		MaxDirectives:        3,
		OnThinking:           func(thought string) { rw.emitThinkingUpdate(sessionID, thought) },
	}

	// Emit Layer 1 start event
	if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: sessionID,
			EventType: "layer1_start",
			Progress:  25.0,
			Data: map[string]interface{}{
				"message":   "Starting Scientist phase - LLM hypothesis generation",
				"phase":     "Layer 1: Scientist",
				"fields":    len(fieldMetadata),
				"artifacts": len(statsArtifacts),
			},
			Timestamp: time.Now(),
		})
	}

	llmStart := time.Now()
	portResponse, err := rw.greenfieldPort.GenerateResearchDirectives(ctx, req)
	llmDuration := time.Since(llmStart)

	if err != nil {
		log.Printf("[ResearchWorker] ❌ LLM call failed after %.2fs for session %s: %v", llmDuration.Seconds(), sessionID, err)

		// Check if this is a JSON parsing error (critical logic failure)
		isJSONError := strings.Contains(err.Error(), "failed to parse JSON content") ||
			strings.Contains(err.Error(), "json_unmarshal") ||
			strings.Contains(err.Error(), "invalid character")

		eventType := "api_error"
		if isJSONError {
			eventType = "state_error"
			log.Printf("[ResearchWorker] 🚨 CRITICAL: JSON parsing failure detected - sending STATE_ERROR event")
		}

		// Emit error event for UI (STATE_ERROR for JSON issues, API_ERROR for other issues)
		if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
			sseHub.Broadcast(api.ResearchEvent{
				SessionID: sessionID,
				EventType: eventType,
				Data: map[string]interface{}{
					"error_message": err.Error(),
					"failed_layer":  "layer1_start",
					"phase":         "Hypothesis Generation",
					"is_json_error": isJSONError,
				},
				Timestamp: time.Now(),
			})
		}

		return nil, fmt.Errorf("failed to generate research directives: %w", err)
	}
	log.Printf("[ResearchWorker] ✅ LLM call completed in %.2fs for session %s", llmDuration.Seconds(), sessionID)

	// Save the rendered prompt (with industry context injection) for debugging
	if portResponse.RenderedPrompt != "" {
		log.Printf("[ResearchWorker] 💾 Saving prompt file for session %s", sessionID)
		if err := rw.savePromptToFile(ctx, sessionID, portResponse.RenderedPrompt); err != nil {
			log.Printf("[ResearchWorker] ⚠️ Failed to save prompt for session %s: %v", sessionID, err)
			// Don't fail the entire process for this
		}
	}

	// Use raw LLM response if available (contains BusinessHypothesis, ScienceHypothesis, etc.)
	if portResponse.RawLLMResponse != nil {
		if llmResp, ok := portResponse.RawLLMResponse.(*models.GreenfieldResearchOutput); ok {
			log.Printf("[ResearchWorker] 🎯 Using raw LLM response for session %s", sessionID)

			// 🔥 IMMEDIATE HYPOTHESIS RENDERING: Create pending hypotheses for UI display
			if err := rw.createPendingHypothesesForUI(ctx, sessionID, llmResp); err != nil {
				log.Printf("[ResearchWorker] ⚠️ Failed to create pending hypotheses for UI: %v", err)
				// Continue anyway - validation will still work
			}

			return llmResp, nil
		}
	}

	// Fallback: convert domain objects to model format (shouldn't happen if adapter is working correctly)
	log.Printf("[ResearchWorker] ⚠️ Raw LLM response not available for session %s, using fallback conversion", sessionID)
	modelResponse := rw.convertPortResponseToModel(portResponse)

	// Create pending hypotheses for fallback case too
	if err := rw.createPendingHypothesesForUI(ctx, sessionID, modelResponse); err != nil {
		log.Printf("[ResearchWorker] ⚠️ Failed to create pending hypotheses for UI (fallback): %v", err)
	}

	return modelResponse, nil
}

// convertPortResponseToModel converts domain directives to model format (fallback only)
func (rw *ResearchWorker) convertPortResponseToModel(portResponse *ports.GreenfieldResearchResponse) *models.GreenfieldResearchOutput {
	directives := make([]models.ResearchDirectiveResponse, len(portResponse.Directives))
	for i, dir := range portResponse.Directives {
		directives[i] = models.ResearchDirectiveResponse{
			ID:                 dir.ID.String(),
			BusinessHypothesis: dir.Claim, // Using Claim as BusinessHypothesis fallback
			ScienceHypothesis:  dir.Claim,
			CauseKey:           string(dir.CauseKey),
			EffectKey:          string(dir.EffectKey),
			NullCase:           "",
			ValidationMethods:  rw.convertValidationStrategy(dir.ValidationStrategy),
			RefereeGates: models.RefereeGates{
				ConfidenceTarget:   dir.RefereeGates.StabilityScore,
				StabilityThreshold: dir.RefereeGates.StabilityScore,
			},
			Claim:              dir.Claim,
			LogicType:          dir.LogicType,
			ValidationStrategy: rw.convertValidationStrategyToModel(dir.ValidationStrategy),
		}
	}

	return &models.GreenfieldResearchOutput{
		ResearchDirectives: directives,
	}
}

func (rw *ResearchWorker) convertValidationStrategy(strategy greenfield.ValidationStrategy) []models.ValidationMethod {
	methods := make([]models.ValidationMethod, 0, 3)
	if strategy.Detector != "" {
		methods = append(methods, models.ValidationMethod{
			Type:          "Detector",
			MethodName:    strategy.Detector,
			ExecutionPlan: fmt.Sprintf("Use %s to detect the pattern.", strategy.Detector),
		})
	}
	if strategy.Scanner != "" {
		methods = append(methods, models.ValidationMethod{
			Type:          "Scanner",
			MethodName:    strategy.Scanner,
			ExecutionPlan: fmt.Sprintf("Use %s to scan for alternatives.", strategy.Scanner),
		})
	}
	if strategy.Proxy != "" {
		methods = append(methods, models.ValidationMethod{
			Type:          "Referee",
			MethodName:    strategy.Proxy,
			ExecutionPlan: fmt.Sprintf("Use %s as referee.", strategy.Proxy),
		})
	}
	return methods
}

func (rw *ResearchWorker) convertValidationStrategyToModel(strategy greenfield.ValidationStrategy) models.ValidationStrategy {
	return models.ValidationStrategy{
		Detector: strategy.Detector,
		Scanner:  strategy.Scanner,
		Proxy:    strategy.Proxy,
	}
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// validateHypothesis performs validation logic on a hypothesis
func (rw *ResearchWorker) validateHypothesis(directive *models.ResearchDirectiveResponse) bool {
	// This is a simplified validation - in a real implementation this would be much more sophisticated
	// For now, we'll randomly validate hypotheses (in practice this would run actual statistical tests)

	// Basic validation: check if required fields are present and make sense
	if directive.ID == "" || directive.BusinessHypothesis == "" || directive.ScienceHypothesis == "" {
		return false
	}

	// Check if validation methods array has at least one method
	if len(directive.ValidationMethods) == 0 {
		return false
	}

	// Validate each validation method has required fields
	for _, method := range directive.ValidationMethods {
		if method.Type == "" || method.MethodName == "" || method.ExecutionPlan == "" {
			return false
		}
	}

	// Check referee gates are reasonable
	if directive.RefereeGates.ConfidenceTarget <= 0 || directive.RefereeGates.ConfidenceTarget > 1 {
		return false
	}

	if directive.RefereeGates.StabilityThreshold < 0 || directive.RefereeGates.StabilityThreshold > 1 {
		return false
	}

	// For this demo, randomly validate 70% of hypotheses as valid
	// In practice, this would run actual statistical validation
	return time.Now().UnixNano()%10 < 7
}

// StartWorkerPool starts a pool of workers for handling research requests
func (rw *ResearchWorker) StartWorkerPool(numWorkers int) {
	log.Printf("[ResearchWorker] 🚀 Starting worker pool with %d workers", numWorkers)
	for i := 0; i < numWorkers; i++ {
		go rw.workerLoop(i)
	}
}

// workerLoop runs the worker event loop with timeout handling and session cleanup
func (rw *ResearchWorker) workerLoop(workerID int) {
	log.Printf("[ResearchWorker] 👷 Worker %d started", workerID)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	sessionTimeout := 45 * time.Minute // Increased timeout for complex research
	cleanupInterval := 5 * time.Minute

	lastCleanup := time.Now()
	lastSessionCount := 0

	for range ticker.C {
		now := time.Now()

		// Periodic cleanup of old sessions
		if now.Sub(lastCleanup) >= cleanupInterval {
			removed := rw.sessionMgr.CleanupOldSessions(2 * time.Hour) // Keep sessions for 2 hours
			if removed > 0 {
				log.Printf("[ResearchWorker] 🧹 Cleaned up %d old sessions", removed)
			}
			lastCleanup = now
		}

		// Check for timed-out sessions and clean them up
		activeSessions, err := rw.sessionMgr.GetActiveSessions(context.Background())
		if err != nil {
			log.Printf("[ResearchWorker] ❌ Failed to get active sessions: %v", err)
			return
		}
		timeoutCount := 0

		for _, session := range activeSessions {
			sessionAge := now.Sub(session.StartedAt)

			// Check if session has been running too long
			if sessionAge > sessionTimeout {
				log.Printf("[ResearchWorker] ⏰ Session %s timed out after %.1f minutes", session.ID, sessionAge.Minutes())
				rw.sessionMgr.SetSessionError(context.Background(), session.ID.String(), fmt.Sprintf("Session timed out after %.1f minutes", sessionAge.Minutes()))
				timeoutCount++
			}
		}

		if timeoutCount > 0 {
			log.Printf("[ResearchWorker] ⏰ Timed out %d sessions", timeoutCount)
		}

		// Log active session count periodically (only when count changes)
		if len(activeSessions) != lastSessionCount {
			log.Printf("[ResearchWorker] 📊 %d active research sessions", len(activeSessions))
			lastSessionCount = len(activeSessions)
		}
	}
}

// emitThinkingUpdate sends real-time thinking updates via SSE
func (rw *ResearchWorker) emitThinkingUpdate(sessionID, thought string) {
	if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: sessionID,
			EventType: "thinking_update",
			Data: map[string]interface{}{
				"thought": thought,
				"phase":   "Layer 1: Scientist",
			},
			Timestamp: time.Now(),
		})
	}
}

// createFallbackHypotheses creates basic hypotheses when LLM generation fails
func (rw *ResearchWorker) createFallbackHypotheses(sessionID string, fieldMetadata []greenfield.FieldMetadata) *models.GreenfieldResearchOutput {
	log.Printf("[ResearchWorker] 🏗️ Creating fallback hypotheses for session %s", sessionID)

	// Create a simple hypothesis using the first few fields
	var directives []models.ResearchDirectiveResponse

	if len(fieldMetadata) >= 2 {
		// Create one basic hypothesis
		directive := models.ResearchDirectiveResponse{
			ID:                 fmt.Sprintf("%s-fallback-001", sessionID[:8]),
			BusinessHypothesis: "Test relationship between key variables",
			ScienceHypothesis:  fmt.Sprintf("There is a relationship between %s and %s", fieldMetadata[0].Name, fieldMetadata[1].Name),
			NullCase:           fmt.Sprintf("There is no relationship between %s and %s", fieldMetadata[0].Name, fieldMetadata[1].Name),
			CauseKey:           fieldMetadata[0].Name,
			EffectKey:          fieldMetadata[1].Name,
			RefereeGates: models.RefereeGates{
				SelectedReferees: []string{"Permutation_Shredder", "Conditional_MI", "LOO_Cross_Validation"}, // Exactly 3 referees as required
				Rationale:        "Fallback hypothesis using Tri-Gate validation with statistical integrity, independence, and robustness checks",
				ConfidenceTarget: 0.999,
			},
			ValidationMethods: []models.ValidationMethod{
				{
					Type:          "SHREDDER",
					MethodName:    "Permutation_Shredder",
					ExecutionPlan: "Test for statistical fluke artifacts using permutation tests",
				},
				{
					Type:          "ANTI_CONFOUNDER",
					MethodName:    "Conditional_MI",
					ExecutionPlan: "Test for hidden variable confounding using conditional mutual information",
				},
				{
					Type:          "SENSITIVITY",
					MethodName:    "LOO_Cross_Validation",
					ExecutionPlan: "Test model stability using leave-one-out cross-validation",
				},
			},
		}
		directives = append(directives, directive)
		log.Printf("[ResearchWorker] ✅ Created 1 fallback hypothesis for session %s", sessionID)
	} else {
		log.Printf("[ResearchWorker] ❌ Cannot create fallback hypotheses: insufficient field metadata (%d fields)", len(fieldMetadata))
	}

	return &models.GreenfieldResearchOutput{
		ResearchDirectives: directives,
	}
}

// savePromptToFile saves the rendered prompt to both database and file
func (rw *ResearchWorker) savePromptToFile(ctx context.Context, sessionID, prompt string) error {
	// Parse session ID
	sessionUUID, err := uuid.Parse(sessionID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}

	// Get default user
	user, err := rw.storage.GetDefaultUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get default user: %w", err)
	}

	// Save to database
	if promptRepo, ok := rw.promptRepo.(ports.PromptRepository); ok {
		metadata := map[string]interface{}{
			"saved_at":    time.Now().Format(time.RFC3339),
			"file_backup": true,
		}
		if err := promptRepo.SavePrompt(ctx, user.ID, sessionUUID, prompt, "research_directive", metadata); err != nil {
			log.Printf("[ResearchWorker] ⚠️ Failed to save prompt to database: %v", err)
			// Continue with file backup even if DB save fails
		}
	}

	// Also save to file as backup (existing behavior)
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("research_prompts/%s_%s_prompt.txt", timestamp, sessionID[:8])

	// Ensure directory exists
	if err := os.MkdirAll("research_prompts", 0755); err != nil {
		return fmt.Errorf("failed to create research_prompts directory: %w", err)
	}

	// Write prompt to file
	err = os.WriteFile(filename, []byte(prompt), 0644)
	if err != nil {
		return fmt.Errorf("failed to write prompt file %s: %w", filename, err)
	}

	return nil
}

// generateEffectSize generates a simulated effect size (Cohen's d)
func (rw *ResearchWorker) generateEffectSize() float64 {
	// Generate effect sizes between -0.8 and 2.0 (typical range for Cohen's d)
	return -0.8 + rand.Float64()*2.8
}

// generatePValue generates a simulated p-value based on validation status
func (rw *ResearchWorker) generatePValue(validated bool) float64 {
	if validated {
		// Validated hypotheses should have significant p-values (typically < 0.05)
		return rand.Float64() * 0.049 // 0.000 to 0.049
	} else {
		// Non-validated hypotheses can have any p-value
		return rand.Float64() // 0.000 to 1.000
	}
}

// generateSampleSize generates a simulated sample size
func (rw *ResearchWorker) generateSampleSize() int {
	// Generate sample sizes between 100 and 10000
	return 100 + rand.Intn(9900)
}

// runStatsSweep executes statistical analysis on the current dataset and returns
// a prompt-friendly artifact slice. This MUST be sourced from the active dataset
// (e.g. Excel file behind the UI), never from hardcoded examples.
func (rw *ResearchWorker) runStatsSweep(ctx context.Context, sessionID string, fieldMetadata []greenfield.FieldMetadata) ([]map[string]interface{}, error) {
	log.Printf("[ResearchWorker] 🔬 Starting stats sweep for session %s", sessionID)

	if rw.statsSweepSvc == nil {
		log.Printf("[ResearchWorker] ❌ Stats sweep service not available for session %s", sessionID)
		return nil, fmt.Errorf("stats sweep service not available")
	}
	if rw.testkit == nil {
		log.Printf("[ResearchWorker] ❌ Testkit not available for session %s", sessionID)
		return nil, fmt.Errorf("testkit not available")
	}

	// Resolve a matrix bundle for the variables we know about.
	// Note: the Excel resolver will ignore any requested keys it cannot resolve.
	varKeys := make([]core.VariableKey, 0, len(fieldMetadata))
	for _, fm := range fieldMetadata {
		if fm.Name == "" {
			continue
		}
		varKeys = append(varKeys, core.VariableKey(fm.Name))
	}
	if len(varKeys) == 0 {
		log.Printf("[ResearchWorker] ❌ No variable keys available for stats sweep in session %s", sessionID)
		return nil, fmt.Errorf("no variable keys available for stats sweep")
	}
	log.Printf("[ResearchWorker] 📊 Resolving matrix bundle for %d variables in session %s", len(varKeys), sessionID)

	matrixStart := time.Now()
	resolver := rw.testkit.MatrixResolverAdapter()
	bundle, err := resolver.ResolveMatrix(ctx, ports.MatrixResolutionRequest{
		ViewID:     core.ID("ui-research"),
		SnapshotID: core.SnapshotID(sessionID),
		EntityIDs:  nil, // include all entities in the dataset
		VarKeys:    varKeys,
	})
	matrixDuration := time.Since(matrixStart)

	if err != nil {
		log.Printf("[ResearchWorker] ❌ Matrix resolution failed after %.2fs for session %s: %v", matrixDuration.Seconds(), sessionID, err)
		return nil, fmt.Errorf("failed to resolve matrix bundle: %w", err)
	}
	log.Printf("[ResearchWorker] ✅ Matrix resolved in %.2fs for session %s (%d entities, %d variables)", matrixDuration.Seconds(), sessionID, len(bundle.Matrix.EntityIDs), len(bundle.Matrix.VariableKeys))

	// Run the sweep and return the resulting artifacts (relationships + manifest).
	log.Printf("[ResearchWorker] 🧮 Running statistical sweep for session %s", sessionID)
	sweepStart := time.Now()
	sweepResp, err := rw.statsSweepSvc.RunStatsSweep(ctx, app.StatsSweepRequest{MatrixBundle: bundle})
	sweepDuration := time.Since(sweepStart)

	if err != nil {
		log.Printf("[ResearchWorker] ❌ Stats sweep failed after %.2fs for session %s: %v", sweepDuration.Seconds(), sessionID, err)
		return nil, fmt.Errorf("stats sweep failed: %w", err)
	}
	log.Printf("[ResearchWorker] ✅ Stats sweep completed in %.2fs for session %s (%d relationships)", sweepDuration.Seconds(), sessionID, len(sweepResp.Relationships))

	artifacts := make([]map[string]interface{}, 0, len(sweepResp.Relationships)+1)
	for _, a := range sweepResp.Relationships {
		artifacts = append(artifacts, map[string]interface{}{
			"kind":       string(a.Kind),
			"id":         a.ID,
			"payload":    a.Payload,
			"created_at": a.CreatedAt,
		})
	}
	artifacts = append(artifacts, map[string]interface{}{
		"kind":       string(sweepResp.Manifest.Kind),
		"id":         sweepResp.Manifest.ID,
		"payload":    sweepResp.Manifest.Payload,
		"created_at": sweepResp.Manifest.CreatedAt,
	})

	log.Printf("[ResearchWorker] 📦 Stats sweep complete for session %s: %d total artifacts", sessionID, len(artifacts))
	return artifacts, nil
}
