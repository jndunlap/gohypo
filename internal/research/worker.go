package research

import (
	"context"
	"fmt"
	"log"
	"time"

	"gohypo/ai"
	"gohypo/app"
	"gohypo/domain/greenfield"
	"gohypo/internal"
	"gohypo/internal/analysis"
	"gohypo/internal/api"
	refereePkg "gohypo/internal/referee"
	"gohypo/internal/testkit"
	"gohypo/internal/validation"
	"gohypo/models"
	"gohypo/ports"
)

type statsSweepRunner interface {
	RunStatsSweep(ctx context.Context, req app.StatsSweepRequest) (*app.StatsSweepResponse, error)
}

// ResearchWorker handles asynchronous research processing
type ResearchWorker struct {
	sessionMgr      *SessionManager
	storage         *ResearchStorage
	promptRepo      interface{}                  // Prompt repository for saving prompts
	greenfieldPort  ports.GreenfieldResearchPort // Port interface for generating research directives
	statsSweepSvc   statsSweepRunner             // Stats sweep service
	testkit         *testkit.TestKit             // TestKit for matrix bundle creation
	sseHub          interface{}                  // SSE hub for real-time updates
	logger          *internal.Logger             // Logger for controlled verbosity
	evalueValidator *EValueValidator             // E-value based validator
	dataPartitioner *analysis.DataPartitioner    // Sample splitting for rigor

	// New AI and validation components
	uiBroadcaster      *ResearchUIBroadcaster      // HTML fragment broadcaster
	hypothesisAnalyzer *ai.HypothesisAnalysisAgent // Hypothesis analysis agent
	validationEngine   interface{}                 // Validation engine (placeholder)
	dynamicSelector    interface{}                 // Dynamic test selector (placeholder)

	// Validated hypothesis summarizer for feedback learning
	hypothesisSummarizer *app.ValidatedHypothesisSummarizer // Summarizes validated hypotheses for prompt feedback

	// Industrial-grade validation components
	validationOrchestrator *validation.ValidationOrchestrator // Advanced validation orchestrator

	// Dataset repository for accessing uploaded datasets
	datasetRepo ports.DatasetRepository // Dataset repository for uploaded files
}

// NewResearchWorker creates a new research worker
func NewResearchWorker(sessionMgr *SessionManager, storage *ResearchStorage, promptRepo interface{}, greenfieldSvc interface{}, llmConfig *models.AIConfig, statsSweepSvc statsSweepRunner, kitAny interface{}, sseHub interface{}, uiBroadcaster *ResearchUIBroadcaster, hypothesisAnalyzer *ai.HypothesisAnalysisAgent, validationEngine interface{}, dynamicSelector interface{}, hypothesisRepo ports.HypothesisRepository, validationOrchestrator *validation.ValidationOrchestrator, datasetRepo ports.DatasetRepository) *ResearchWorker {
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

	// Initialize statistical rigor components
	evalueCalibrator := analysis.NewEValueCalibrator()
	evalueValidator := NewEValueValidator(evalueCalibrator)
	dataPartitioner := analysis.NewDataPartitioner()

	// Initialize hypothesis summarizer for feedback learning
	hypothesisSummarizer := app.NewValidatedHypothesisSummarizer(hypothesisRepo)

	return &ResearchWorker{
		sessionMgr:            sessionMgr,
		storage:               storage,
		promptRepo:            promptRepo,
		greenfieldPort:        greenfieldPort,
		statsSweepSvc:         statsSweepSvc,
		testkit:               kit,
		sseHub:                sseHub,
		logger:                internal.NewDefaultLogger(),
		evalueValidator:       evalueValidator,
		dataPartitioner:       dataPartitioner,
		uiBroadcaster:         uiBroadcaster,
		hypothesisAnalyzer:    hypothesisAnalyzer,
		validationEngine:      validationEngine,
		dynamicSelector:       dynamicSelector,
		hypothesisSummarizer:  hypothesisSummarizer,
		validationOrchestrator: validationOrchestrator,
		datasetRepo:           datasetRepo,
	}
}

// RunStatsSweep executes statistical analysis and returns artifacts
func (rw *ResearchWorker) RunStatsSweep(ctx context.Context, sessionID string, fieldMetadata []greenfield.FieldMetadata) ([]map[string]interface{}, error) {
	return rw.runStatsSweep(ctx, sessionID, fieldMetadata)
}

// ProcessResearch initiates and manages the research generation workflow
func (rw *ResearchWorker) ProcessResearch(ctx context.Context, sessionID string, fieldMetadata []greenfield.FieldMetadata, statsArtifacts []map[string]interface{}, sseHub interface{}) {
	sessionStart := time.Now()
	rw.logger.Info("Starting research process for session %s (%d fields, %d artifacts)", sessionID, len(fieldMetadata), len(statsArtifacts))

	// Initialize session-level variables
	var totalHypotheses int
	var successCount, failureCount int

	defer func() {
		sessionDuration := time.Since(sessionStart)
		rw.logger.Info("Session %s completed: %d hypotheses in %.2fs", sessionID, totalHypotheses, sessionDuration.Seconds())
		if rw.logger.GetLevel() >= internal.LogLevelDebug && (successCount > 0 || failureCount > 0) {
			rw.logger.Debug("Validation results: %d passed, %d failed", successCount, failureCount)
		}
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
	if err := rw.sessionMgr.SetSessionState(ctx, sessionID, models.SessionStateAnalyzing); err != nil {
		log.Printf("[ResearchWorker] ERROR: Failed to start analysis for session %s: %v", sessionID, err)
		return
	}

	// Handle statistical artifacts - attempt stats sweep when no pre-computed artifacts available
	if len(statsArtifacts) == 0 {
		log.Printf("[ResearchWorker] 📊 Phase 2/4: Statistical Analysis - No pre-computed artifacts available for session %s", sessionID)
		log.Printf("[ResearchWorker] 🔄 Attempting stats sweep to generate statistical artifacts...")

		// Attempt to run stats sweep to generate artifacts
		newArtifacts, err := rw.RunStatsSweep(ctx, sessionID, fieldMetadata)
		if err != nil {
			log.Printf("[ResearchWorker] ⚠️ Stats sweep failed, proceeding with field metadata only: %v", err)
			statsArtifacts = []map[string]interface{}{} // Empty artifacts - LLM will work with field metadata only
		} else {
			statsArtifacts = newArtifacts
			log.Printf("[ResearchWorker] ✅ Stats sweep completed, generated %d artifacts", len(statsArtifacts))
		}
	} else {
		log.Printf("[ResearchWorker] 📊 Phase 2/4: Statistical Analysis - Using %d existing artifacts for session %s", len(statsArtifacts), sessionID)
		log.Printf("[ResearchWorker] 🔄 Running additional stats sweep to augment existing artifacts...")
		// Run stats sweep to get additional artifacts
		newArtifacts, err := rw.RunStatsSweep(ctx, sessionID, fieldMetadata)
		if err != nil {
			log.Printf("[ResearchWorker] ⚠️ Additional stats sweep failed, continuing with existing artifacts: %v", err)
		} else {
			statsArtifacts = append(statsArtifacts, newArtifacts...)
			log.Printf("[ResearchWorker] ✅ Additional stats sweep completed, total artifacts: %d", len(statsArtifacts))
		}
	}

	// Convert metadata and stats artifacts to JSON for LLM processing
	log.Printf("[ResearchWorker] 📝 Preparing field metadata JSON for session %s", sessionID)
	fieldJSON, err := rw.prepareFieldMetadata(fieldMetadata, statsArtifacts, nil)
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
		log.Printf("[ResearchWorker] Generated %d hypotheses for validation", len(hypotheses.ResearchDirectives))

		// Emit hypothesis generation events for chat interface
		if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
			for i, directive := range hypotheses.ResearchDirectives {
				hypothesisData := map[string]interface{}{
					"id":                   directive.ID,
					"phenomenon_name":      directive.PhenomenonName,
					"business_hypothesis":  directive.BusinessHypothesis,
					"science_hypothesis":   directive.ScienceHypothesis,
					"null_case":           directive.NullCase,
					"cause_key":           directive.CauseKey,
					"effect_key":          directive.EffectKey,
					"opportunity_topology": directive.OpportunityTopology,
					"explanation_markdown": directive.ExplanationMarkdown,
					"sequence":            i + 1,
					"total":              len(hypotheses.ResearchDirectives),
				}

				sseHub.Broadcast(api.ResearchEvent{
					SessionID: sessionID,
					EventType: "hypothesis_generated",
					Progress:  float64(i+1) / float64(len(hypotheses.ResearchDirectives)) * 30.0 + 20.0, // 20-50% range for hypothesis generation
					Data:      hypothesisData,
					Timestamp: time.Now(),
				})

				// Small delay between hypothesis events for better UX
				time.Sleep(200 * time.Millisecond)
			}
		}
	}

	// Skip to validation phase - no intermediate analysis needed

	// Emit Layer 2 start event
	if sseHub, ok := rw.sseHub.(*api.SSEHub); ok {
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: sessionID,
			EventType: "layer2_start",
			Progress:  50.0,
			Data: map[string]interface{}{
				"message": "Starting Referee phase - E-value dynamic validation",
				"phase":   "Layer 2: Referee",
			},
			Timestamp: time.Now(),
		})
	}

	// Update session state to validating
	log.Printf("[ResearchWorker] Updating session %s to validating state", sessionID)
	if err := rw.sessionMgr.SetSessionState(ctx, sessionID, models.SessionStateValidating); err != nil {
		log.Printf("[ResearchWorker] ERROR: Failed to update session state to validating: %v", err)
		return
	}

	// Validate each hypothesis using e-value dynamic validation
	phaseStart = time.Now()
	totalHypotheses = len(hypotheses.ResearchDirectives)
	log.Printf("[ResearchWorker] Starting validation phase for %d hypotheses in session %s", totalHypotheses, sessionID)

	for i, directive := range hypotheses.ResearchDirectives {
		hypothesisStart := time.Now()
		hypothesisNum := i + 1
		progressPercent := float64(hypothesisNum-1) / float64(totalHypotheses) * 100

		log.Printf("[ResearchWorker] Processing hypothesis %d/%d (%.1f%%) - ID: %s", hypothesisNum, totalHypotheses, progressPercent, directive.ID)

		// Update progress
		progress := float64(i) / float64(totalHypotheses) * 100
		currentHypothesis := fmt.Sprintf("E-value Validating: %s - %s", directive.ID, directive.BusinessHypothesis)
		rw.sessionMgr.UpdateSessionProgress(ctx, sessionID, progress, currentHypothesis)

		// Execute E-value validation with Q-value continuity and sample partitioning
		var validationPassed bool
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ResearchWorker] ERROR: Panic in hypothesis %s validation: %v", directive.ID, r)
					rw.recordFailedHypothesis(ctx, sessionID, directive.ID, fmt.Sprintf("Panic during validation: %v", r))
					validationPassed = false
				}
			}()

			validationPassed = rw.executeEValueValidation(ctx, sessionID, directive)
		}()

		hypothesisDuration := time.Since(hypothesisStart)
		phaseDuration = time.Since(phaseStart)

		log.Printf("[ResearchWorker] Hypothesis %s validation completed in %.2fs", directive.ID, hypothesisDuration.Seconds())

		// Count successes vs failures
		if validationPassed {
			successCount++
		} else {
			failureCount++
		}
	}

	log.Printf("[ResearchWorker] Validation completed for session %s: %d hypotheses processed", sessionID, totalHypotheses)

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

// buildDiscoveryEvidenceFromStats extracts FDR-corrected evidence from statistical artifacts
func (rw *ResearchWorker) buildDiscoveryEvidenceFromStats(
	statsArtifacts []map[string]interface{},
	directive models.ResearchDirectiveResponse,
) []refereePkg.DiscoveryEvidence {

	var evidence []refereePkg.DiscoveryEvidence

	for _, artifact := range statsArtifacts {
		kind, _ := artifact["kind"].(string)
		if kind != "relationship" {
			continue
		}

		payload, ok := artifact["payload"].(map[string]interface{})
		if !ok {
			continue
		}

		// Extract relationship data
		metrics, ok := payload["metrics"].(map[string]interface{})
		if !ok {
			continue
		}

		// Check if this relationship matches our hypothesis variables
		varX, _ := payload["variable_x"].(string)
		varY, _ := payload["variable_y"].(string)

		if varX == directive.CauseKey && varY == directive.EffectKey {
			// This is relevant evidence - extract Q-values and other data
			discoveryEv := refereePkg.DiscoveryEvidence{
				CauseKey:         varX,
				EffectKey:        varY,
				TestType:         getString(metrics, "test_type"),
				PValue:           getFloat64(metrics, "p_value"),
				QValue:           getFloat64(metrics, "q_value"),
				SampleSize:       int(getFloat64(metrics, "sample_size")),
				TotalComparisons: int(getFloat64(metrics, "total_comparisons")),
				FDRMethod:        getString(metrics, "fdr_method"),
			}

			evidence = append(evidence, discoveryEv)
		}
	}

	return evidence
}

// performSamplePartitioningForValidation creates discovery and validation partitions
func (rw *ResearchWorker) performSamplePartitioningForValidation(
	ctx context.Context,
	directive models.ResearchDirectiveResponse,
) (*analysis.PartitionResult, error) {

	// Get full dataset for partitioning
	matrixBundle, err := rw.loadMatrixBundleForHypothesisWithContext(ctx, directive)
	if err != nil {
		return nil, fmt.Errorf("failed to load matrix for partitioning: %w", err)
	}

	// Extract entity IDs and data matrix
	entityIDs := matrixBundle.Matrix.EntityIDs
	variableKeys := matrixBundle.Matrix.VariableKeys
	dataMatrix := matrixBundle.Matrix // Simplified - would need actual data extraction

	// Perform sample partitioning
	partitionConfig := analysis.DefaultPartitionConfig()
	partitionResult, err := rw.dataPartitioner.PartitionDataset(
		entityIDs,
		variableKeys,
		dataMatrix,
		partitionConfig,
	)

	if err != nil {
		return nil, fmt.Errorf("sample partitioning failed: %w", err)
	}

	// Validate partition quality
	if err := rw.dataPartitioner.ValidatePartitions(partitionResult); err != nil {
		return nil, fmt.Errorf("partition validation failed: %w", err)
	}

	return partitionResult, nil
}

// convertPartitionToMatrixBundle converts a partition to matrix bundle format
func (rw *ResearchWorker) convertPartitionToMatrixBundle(partition analysis.DatasetPartition) MatrixBundle {
	return MatrixBundle{
		Matrix:          partition.DataMatrix,
		EntityIDs:       partition.EntityIDs,
		VariableKeys:    partition.VariableKeys,
		IsValidationSet: partition.IsDiscovery, // Note: inverted logic for naming
	}
}

// Helper functions for type conversion
func getFloat64(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case int64:
			return float64(v)
		}
	}
	return 0.0
}
