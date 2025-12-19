package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"gohypo/internal/api"
	refereePkg "gohypo/internal/referee"
	"gohypo/models"
	"gohypo/ports"
)

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
		Directives:           3,
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
		isJSONError := isJSONParsingError(err)

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

// isJSONParsingError checks if an error is related to JSON parsing
func isJSONParsingError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "failed to parse JSON content") ||
		strings.Contains(errStr, "json_unmarshal") ||
		strings.Contains(errStr, "invalid character")
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
