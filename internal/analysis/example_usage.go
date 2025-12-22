// +build ignore

package main

import (
	"gohypo/internal/analysis"
	"gohypo/internal/api"
)

// Example usage of the Scientific Ledger system
func exampleUsage() {
	// Create SSE hub and broadcaster
	sseHub := api.NewSSEHub()
	broadcaster := api.NewSSEEventBroadcaster(sseHub)

	// Create scientific ledger
	ledger := analysis.NewScientificLedger(broadcaster)

	// Record evidence (no dependencies)
	evidenceSID, err := ledger.RecordEvidence("session-123", "correlation_result", map[string]interface{}{
		"correlation": 0.78,
		"p_value":     0.001,
		"variables":   []string{"page_load_time", "bounce_rate"},
	})
	if err != nil {
		log.Printf("Failed to record evidence: %v", err)
		return
	}

	// Record hypothesis (depends on evidence)
	hypothesisSID, err := ledger.RecordHypothesis("session-123", "H-42", "hypothesis_generated", map[string]interface{}{
		"business_hypothesis": "Increased page load time causes higher bounce rates",
		"confidence":          0.89,
	}, evidenceSID)
	if err != nil {
		log.Printf("Failed to record hypothesis: %v", err)
		return
	}

	// Record progress (no dependencies, always broadcasts)
	_, err = ledger.RecordProgress("session-123", "validation_progress", 75.0, map[string]interface{}{
		"current_test": "t-test",
		"passed":       3,
		"total":        5,
	})
	if err != nil {
		log.Printf("Failed to record progress: %v", err)
	}

	// Mark evidence as rendered (allows dependent hypotheses to show)
	err = ledger.MarkEvidenceRendered("session-123", evidenceSID)
	if err != nil {
		log.Printf("Failed to mark evidence rendered: %v", err)
	}

	// Get complete ledger for traceability
	sessionLedger := ledger.GetSessionLedger("session-123")
	evidenceChain := sessionLedger.GetEvidenceChain(hypothesisSID)

	_ = evidenceChain // Use for UI display

	// Cleanup when session ends
	ledger.CleanupSession("session-123")
}
