package research

import (
	"context"
	"fmt"
	"log"
	"time"

	"gohypo/app"
	"gohypo/domain/greenfield"
	"gohypo/internal/api"
	"gohypo/internal/testkit"
	"gohypo/models"
	"gohypo/ports"
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
func NewResearchWorker(sessionMgr *SessionManager, storage *ResearchStorage, promptRepo interface{}, greenfieldSvc interface{}, llmConfig *models.AIConfig, statsSweepSvc statsSweepRunner, kitAny interface{}, sseHub interface{}) *ResearchWorker {
	// Extract the port from the greenfield service
	var greenfieldPort ports.GreenfieldResearchPort
	if gs, ok := greenfieldSvc.(*app.GreenfieldService); ok {
		// Access the port through reflection or add a getter method
		// For now, we'll add a getter method to GreenfieldService
		greenfieldPort = gs.GetGreenfieldPort()
	} else if gp, ok := greenfieldSvc.(ports.GreenfieldResearchPort); ok {
		greenfieldPort = gp
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
		statsSweepSvc:  statsSweepSvc,
		testkit:        kit,
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
		log.Printf("[ResearchWorker] 🛑 Session %s cannot continue - hypothesis generation failed", sessionID)
		log.Printf("[ResearchWorker] 🔧 Suggested actions: Check LLM service connectivity, verify field metadata quality")
		rw.sessionMgr.SetSessionError(ctx, sessionID, fmt.Sprintf("Failed to generate hypotheses: %v", err))
		return
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
