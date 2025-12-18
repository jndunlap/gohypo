package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"time"

	"gohypo/app"
	"gohypo/domain/core"
	"gohypo/domain/discovery"
	"gohypo/domain/greenfield"
	"gohypo/domain/stats"
	"gohypo/models"
	"gohypo/ports"
)

// ResearchWorker handles asynchronous research processing
type ResearchWorker struct {
	sessionMgr     *SessionManager
	storage        *ResearchStorage
	greenfieldPort ports.GreenfieldResearchPort // Port interface for generating research directives
	statsSweepSvc  interface{}                  // Stats sweep service
	testkit        interface{}                  // TestKit for matrix bundle creation
}

// NewResearchWorker creates a new research worker
func NewResearchWorker(sessionMgr *SessionManager, storage *ResearchStorage, greenfieldSvc interface{}, llmConfig *models.AIConfig, statsSweepSvc interface{}, testkit interface{}) *ResearchWorker {
	// Extract the port from the greenfield service
	var greenfieldPort ports.GreenfieldResearchPort
	if gs, ok := greenfieldSvc.(*app.GreenfieldService); ok {
		// Access the port through reflection or add a getter method
		// For now, we'll add a getter method to GreenfieldService
		greenfieldPort = gs.GetGreenfieldPort()
	} else if gp, ok := greenfieldSvc.(ports.GreenfieldResearchPort); ok {
		greenfieldPort = gp
	}

	return &ResearchWorker{
		sessionMgr:     sessionMgr,
		storage:        storage,
		greenfieldPort: greenfieldPort,
		statsSweepSvc:  statsSweepSvc,
		testkit:        testkit,
	}
}

// ProcessResearch initiates and manages the research generation workflow
func (rw *ResearchWorker) ProcessResearch(sessionID string, fieldMetadata []greenfield.FieldMetadata, statsArtifacts []map[string]interface{}) {
	log.Printf("[ResearchWorker] Starting research session: %s", sessionID)

	// Update session state to analyzing
	if err := rw.sessionMgr.SetSessionState(sessionID, SessionStateAnalyzing); err != nil {
		log.Printf("[ResearchWorker] Failed to update session state: %v", err)
		return
	}

	// Run stats sweep if no statistical artifacts are available
	if len(statsArtifacts) == 0 {
		log.Printf("[ResearchWorker] No statistical artifacts found, running stats sweep")
		if err := rw.runStatsSweep(sessionID); err != nil {
			log.Printf("[ResearchWorker] Failed to run stats sweep: %v", err)
			rw.sessionMgr.SetSessionError(sessionID, fmt.Sprintf("Failed to run stats sweep: %v", err))
			return
		}

		// Re-fetch statistical artifacts after sweep
		var err error
		statsArtifacts, err = rw.getStatisticalArtifacts()
		if err != nil {
			log.Printf("[ResearchWorker] Failed to fetch updated stats artifacts: %v", err)
			rw.sessionMgr.SetSessionError(sessionID, fmt.Sprintf("Failed to fetch stats artifacts: %v", err))
			return
		}
		log.Printf("[ResearchWorker] Stats sweep completed, found %d statistical artifacts", len(statsArtifacts))
	}

	// Build basic discovery briefs for LLM context (will be enhanced with sense results later)
	discoveryBriefs := rw.buildDiscoveryBriefs(sessionID, statsArtifacts)

	// Convert metadata and stats artifacts to JSON for LLM processing
	fieldJSON, err := rw.prepareFieldMetadata(fieldMetadata, statsArtifacts, discoveryBriefs)
	if err != nil {
		log.Printf("[ResearchWorker] Failed to prepare field metadata: %v", err)
		rw.sessionMgr.SetSessionError(sessionID, fmt.Sprintf("Failed to prepare metadata: %v", err))
		return
	}

	// Generate hypotheses using LLM
	hypotheses, err := rw.generateHypotheses(sessionID, fieldJSON)
	if err != nil {
		log.Printf("[ResearchWorker] Failed to generate hypotheses: %v", err)
		rw.sessionMgr.SetSessionError(sessionID, fmt.Sprintf("Failed to generate hypotheses: %v", err))
		return
	}

	// Update session state to validating
	if err := rw.sessionMgr.SetSessionState(sessionID, SessionStateValidating); err != nil {
		log.Printf("[ResearchWorker] Failed to update session state: %v", err)
		return
	}

	// Validate each hypothesis
	totalHypotheses := len(hypotheses.ResearchDirectives)
	for i, directive := range hypotheses.ResearchDirectives {
		// Update progress
		progress := float64(i) / float64(totalHypotheses) * 100
		currentHypothesis := fmt.Sprintf("Validating: %s - %s", directive.ID, directive.BusinessHypothesis)
		rw.sessionMgr.UpdateSessionProgress(sessionID, progress, currentHypothesis)

		// Perform validation (simplified for now - in real implementation this would be complex)
		validated := rw.validateHypothesis(&directive)

		// Create result with simulated metrics (in real implementation, these would come from validation)
		result := HypothesisResult{
			ID:                 directive.ID,
			BusinessHypothesis: directive.BusinessHypothesis,
			ScienceHypothesis:  directive.ScienceHypothesis,
			NullCase:           directive.NullCase,
			ValidationMethods:  directive.ValidationMethods,
			RefereeGates:       directive.RefereeGates,
			Validated:          validated,
			Rejected:           !validated,
			EffectSize:         rw.generateEffectSize(),      // Simulated effect size
			PValue:             rw.generatePValue(validated), // Simulated p-value
			SampleSize:         rw.generateSampleSize(),      // Simulated sample size
			CreatedAt:          time.Now(),
			ProcessingTime:     time.Since(time.Now()), // This would be tracked properly
			Metadata: map[string]interface{}{
				"session_id": sessionID,
			},
			DiscoveryBriefs: discoveryBriefs,
		}

		// Save to storage
		if err := rw.storage.SaveHypothesis(&result); err != nil {
			log.Printf("[ResearchWorker] Failed to save hypothesis %s: %v", directive.ID, err)
			continue
		}

		// Add to session
		if err := rw.sessionMgr.AddHypothesisToSession(sessionID, result); err != nil {
			log.Printf("[ResearchWorker] Failed to add hypothesis to session: %v", err)
		}
	}

	// Complete the session
	if err := rw.sessionMgr.SetSessionState(sessionID, SessionStateComplete); err != nil {
		log.Printf("[ResearchWorker] Failed to complete session: %v", err)
	}

	log.Printf("[ResearchWorker] Completed research session: %s with %d hypotheses", sessionID, totalHypotheses)
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

	log.Printf("[ResearchWorker] Prepared comprehensive context: %d fields, %d statistical artifacts", len(metadata), len(statsArtifacts))
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

// generateHypotheses calls the LLM to generate research hypotheses via the GreenfieldAdapter (which includes Forensic Scout)
func (rw *ResearchWorker) generateHypotheses(sessionID string, fieldJSON string) (*models.GreenfieldResearchOutput, error) {
	log.Printf("[ResearchWorker] Generating hypotheses for session %s", sessionID)

	if rw.greenfieldPort == nil {
		return nil, fmt.Errorf("greenfield port not available")
	}

	// Parse field metadata from JSON
	var contextData map[string]interface{}
	if err := json.Unmarshal([]byte(fieldJSON), &contextData); err != nil {
		return nil, fmt.Errorf("failed to parse field JSON: %w", err)
	}

	// Extract field metadata
	fieldMetadataRaw, ok := contextData["field_metadata"].([]interface{})
	if !ok {
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

	// Call the port (which uses GreenfieldAdapter with Forensic Scout)
	ctx := context.Background()
	req := ports.GreenfieldResearchRequest{
		RunID:                core.RunID(sessionID),
		SnapshotID:           core.SnapshotID(""), // Not used in UI flow
		FieldMetadata:        fieldMetadata,
		StatisticalArtifacts: statsArtifacts,
		DiscoveryBriefs:      discoveryBriefs,
		MaxDirectives:        3,
	}

	portResponse, err := rw.greenfieldPort.GenerateResearchDirectives(ctx, req)
	if err != nil {
		log.Printf("[ResearchWorker] Failed to generate research directives: %v", err)
		return nil, fmt.Errorf("failed to generate research directives: %w", err)
	}

	// Save the rendered prompt (with industry context injection) for debugging
	if portResponse.RenderedPrompt != "" {
		if err := rw.savePromptToFile(sessionID, portResponse.RenderedPrompt); err != nil {
			log.Printf("[ResearchWorker] Failed to save prompt: %v", err)
			// Don't fail the entire process for this
		}
	}

	// Use raw LLM response if available (contains BusinessHypothesis, ScienceHypothesis, etc.)
	if portResponse.RawLLMResponse != nil {
		if llmResp, ok := portResponse.RawLLMResponse.(*models.GreenfieldResearchOutput); ok {
			log.Printf("[ResearchWorker] Successfully generated %d hypotheses for session %s", len(llmResp.ResearchDirectives), sessionID)
			return llmResp, nil
		}
	}

	// Fallback: convert domain objects to model format (shouldn't happen if adapter is working correctly)
	log.Printf("[ResearchWorker] Warning: Raw LLM response not available, using fallback conversion")
	modelResponse := rw.convertPortResponseToModel(portResponse)
	log.Printf("[ResearchWorker] Successfully generated %d hypotheses for session %s", len(modelResponse.ResearchDirectives), sessionID)
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
	for i := 0; i < numWorkers; i++ {
		go rw.workerLoop()
	}
	log.Printf("[ResearchWorker] Started %d research workers", numWorkers)
}

// workerLoop runs the worker event loop (placeholder for now)
func (rw *ResearchWorker) workerLoop() {
	// In a real implementation, this would listen for work requests
	// For now, it's a placeholder
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Check for timed-out sessions and clean them up
		activeSessions := rw.sessionMgr.GetActiveSessions()
		for _, session := range activeSessions {
			// Check if session has been running too long
			if time.Since(session.StartedAt) > 30*time.Minute {
				log.Printf("[ResearchWorker] Session %s timed out", session.ID)
				rw.sessionMgr.SetSessionError(session.ID, "Session timed out")
			}
		}
	}
}

// savePromptToFile saves the rendered prompt to a timestamped file
func (rw *ResearchWorker) savePromptToFile(sessionID, prompt string) error {
	// Create filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("research_prompts/%s_%s_prompt.txt", timestamp, sessionID[:8])

	// Write prompt to file
	err := os.WriteFile(filename, []byte(prompt), 0644)
	if err != nil {
		return fmt.Errorf("failed to write prompt file %s: %w", filename, err)
	}

	log.Printf("[ResearchWorker] Saved prompt to file: %s", filename)
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

// runStatsSweep executes statistical analysis on the data
func (rw *ResearchWorker) runStatsSweep(sessionID string) error {
	// For now, we'll simulate running stats sweep by creating some basic statistical artifacts
	// In a full implementation, this would:
	// 1. Create a MatrixBundle from Excel data
	// 2. Run the stats sweep service
	// 3. Store the results in the ledger

	log.Printf("[ResearchWorker] Simulating stats sweep for session %s", sessionID)

	// Simulate some basic statistical artifacts that would be generated by stats sweep
	// In reality, these would come from actual correlation analysis
	simulatedArtifacts := []map[string]interface{}{
		{
			"kind": "ArtifactRelationship",
			"id":   "rel_001",
			"payload": map[string]interface{}{
				"variable_x":       "brand_search_volume",
				"variable_y":       "campaign_sub_metric_1",
				"test_used":        "pearson",
				"effect_size":      0.65,
				"p_value":          0.001,
				"correlation_type": "positive",
			},
		},
		{
			"kind": "ArtifactRelationship",
			"id":   "rel_002",
			"payload": map[string]interface{}{
				"variable_x":       "conversion_rate",
				"variable_y":       "campaign_sub_metric_2",
				"test_used":        "spearman",
				"effect_size":      0.45,
				"p_value":          0.01,
				"correlation_type": "positive",
			},
		},
		{
			"kind": "ArtifactRelationship",
			"id":   "rel_003",
			"payload": map[string]interface{}{
				"variable_x":       "top_funnel_spend_usd",
				"variable_y":       "impressions_per_user",
				"test_used":        "pearson",
				"effect_size":      0.78,
				"p_value":          0.0001,
				"correlation_type": "positive",
			},
		},
	}

	// Store these simulated artifacts in the research storage
	// In a real implementation, they'd be stored in the ledger
	for _, artifact := range simulatedArtifacts {
		if err := rw.storage.StoreSimulatedArtifact(sessionID, artifact); err != nil {
			log.Printf("[ResearchWorker] Failed to store simulated artifact: %v", err)
		}
	}

	log.Printf("[ResearchWorker] Simulated stats sweep completed for session %s", sessionID)
	return nil
}

// getStatisticalArtifacts fetches current statistical artifacts
func (rw *ResearchWorker) getStatisticalArtifacts() ([]map[string]interface{}, error) {
	// In a real implementation, this would query the ledger
	// For now, return simulated artifacts
	return rw.storage.GetSimulatedArtifacts(), nil
}
