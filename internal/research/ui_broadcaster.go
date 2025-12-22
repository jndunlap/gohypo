package research

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"sync"
	"time"

	"gohypo/internal/api"
	"gohypo/models"
)

// ResearchUIBroadcaster handles HTML fragment broadcasting for live UI updates
type ResearchUIBroadcaster struct {
	sseHub        *api.SSEHub
	templates     *template.Template
	sessionMgr    *SessionManager
	bufferMu      sync.RWMutex
	updateBuffers map[string]*UpdateBuffer
	debounceTimer *time.Timer
}

// UpdateBuffer holds pending UI updates for debounced broadcasting
type UpdateBuffer struct {
	SessionID    string
	HypothesisID string
	LastUpdate   time.Time
	PendingData  map[string]interface{}
	HTMLFragment string
}

// NewResearchUIBroadcaster creates a new UI broadcaster
func NewResearchUIBroadcaster(sseHub *api.SSEHub, sessionMgr *SessionManager, templates *template.Template) *ResearchUIBroadcaster {
	return &ResearchUIBroadcaster{
		sseHub:        sseHub,
		templates:     templates,
		sessionMgr:    sessionMgr,
		updateBuffers: make(map[string]*UpdateBuffer),
	}
}

// BroadcastHypothesisPending streams a pending hypothesis card to the UI
func (b *ResearchUIBroadcaster) BroadcastHypothesisPending(sessionID string, hypothesis *models.ResearchDirectiveResponse) error {
	data := struct {
		ID                 string
		BusinessHypothesis string
		ScienceHypothesis  string
	}{
		ID:                 hypothesis.ID,
		BusinessHypothesis: hypothesis.BusinessHypothesis,
		ScienceHypothesis:  hypothesis.ScienceHypothesis,
	}

	html, err := b.renderFragment("hypothesis/hypothesis_pending", data)
	if err != nil {
		return fmt.Errorf("failed to render hypothesis pending fragment: %w", err)
	}

	return b.broadcastHTMLFragment(sessionID, "hypothesis_pending", html, hypothesis.ID)
}

// BroadcastHypothesisRiskAssessed streams risk assessment results
func (b *ResearchUIBroadcaster) BroadcastHypothesisRiskAssessed(sessionID, hypothesisID string, riskProfile interface{}) error {
	// This would be called after the HypothesisAnalysisAgent completes
	data := map[string]interface{}{
		"ID":                 hypothesisID,
		"RiskLevel":          "medium", // Would come from riskProfile
		"RequiredTestCount":  map[string]int{"Min": 3, "Max": 6},
		"CriticalConcerns":   []string{"sample_size", "sparsity"},
		"FeasibilityScore":   0.75,
		"BusinessHypothesis": "Analyzed hypothesis",
		"ScienceHypothesis":  "Scientific analysis",
	}

	html, err := b.renderFragment("hypothesis/hypothesis_risk_assessed", data)
	if err != nil {
		return fmt.Errorf("failed to render risk assessment fragment: %w", err)
	}

	return b.broadcastHTMLFragment(sessionID, "hypothesis_risk_assessed", html, hypothesisID)
}

// BroadcastHypothesisValidating streams validation start
func (b *ResearchUIBroadcaster) BroadcastHypothesisValidating(sessionID, hypothesisID string, totalTests, activeTests int, currentEValue, confidence float64) error {
	data := struct {
		ID                 string
		BusinessHypothesis string
		ScienceHypothesis  string
		TotalTests         int
		ActiveTests        int
		CurrentEValue      float64
		Confidence         float64
	}{
		ID:            hypothesisID,
		TotalTests:    totalTests,
		ActiveTests:   activeTests,
		CurrentEValue: currentEValue,
		Confidence:    confidence,
	}

	html, err := b.renderFragment("hypothesis/hypothesis_validating", data)
	if err != nil {
		return fmt.Errorf("failed to render validation fragment: %w", err)
	}

	return b.broadcastHTMLFragment(sessionID, "hypothesis_validating", html, hypothesisID)
}

// BroadcastHypothesisCompleted streams final results
func (b *ResearchUIBroadcaster) BroadcastHypothesisCompleted(sessionID, hypothesisID string, result *models.HypothesisResult) error {
	data := struct {
		ID                 string
		BusinessHypothesis string
		ScienceHypothesis  string
		FinalVerdict       string
		CurrentEValue      float64
		Confidence         float64
		CompletedTests     int
		TotalTests         int
	}{
		ID:             hypothesisID,
		FinalVerdict:   map[bool]string{true: "accepted", false: "rejected"}[result.Passed],
		CurrentEValue:  0.0, // Would come from result
		Confidence:     0.0, // Would come from result
		CompletedTests: 5,   // Would come from result
		TotalTests:     5,   // Would come from result
	}

	html, err := b.renderFragment("hypothesis/hypothesis_completed", data)
	if err != nil {
		return fmt.Errorf("failed to render completion fragment: %w", err)
	}

	return b.broadcastHTMLFragment(sessionID, "hypothesis_completed", html, hypothesisID)
}

// BroadcastTestStatusUpdate streams individual test completion
func (b *ResearchUIBroadcaster) BroadcastTestStatusUpdate(sessionID, hypothesisID, testName, shortName string, passed, completed bool) error {
	data := struct {
		HypothesisID string
		TestName     string
		ShortName    string
		Passed       bool
		Completed    bool
	}{
		HypothesisID: hypothesisID,
		TestName:     testName,
		ShortName:    shortName,
		Passed:       passed,
		Completed:    completed,
	}

	html, err := b.renderFragment("status/test_status_update", data)
	if err != nil {
		return fmt.Errorf("failed to render test status fragment: %w", err)
	}

	return b.broadcastHTMLFragment(sessionID, "test_status_update", html, hypothesisID)
}

// BroadcastEntityDetectionFailure streams the intervention UI
func (b *ResearchUIBroadcaster) BroadcastEntityDetectionFailure(sessionID string, availableFields []string) error {
	data := struct {
		SessionID       string
		AvailableFields []string
	}{
		SessionID:       sessionID,
		AvailableFields: availableFields,
	}

	html, err := b.renderFragment("modals/entity_detection_failure", data)
	if err != nil {
		return fmt.Errorf("failed to render entity detection failure fragment: %w", err)
	}

	return b.broadcastHTMLFragment(sessionID, "entity_detection_failure", html, "")
}

// BroadcastProgressUpdate streams progress bar updates
func (b *ResearchUIBroadcaster) BroadcastProgressUpdate(sessionID, phase, message string, progress float64, activeHypotheses int) error {
	data := struct {
		Phase            string
		Message          string
		Progress         float64
		ActiveHypotheses int
	}{
		Phase:            phase,
		Message:          message,
		Progress:         progress,
		ActiveHypotheses: activeHypotheses,
	}

	html, err := b.renderFragment("status/progress_update", data)
	if err != nil {
		return fmt.Errorf("failed to render progress update fragment: %w", err)
	}

	return b.broadcastHTMLFragment(sessionID, "progress_update", html, "")
}

// renderFragment renders a template fragment with the given data
func (b *ResearchUIBroadcaster) renderFragment(templateName string, data interface{}) (string, error) {
	var buf bytes.Buffer
	if err := b.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// broadcastHTMLFragment sends an HTML fragment via SSE with HTMX headers
func (b *ResearchUIBroadcaster) broadcastHTMLFragment(sessionID, eventType, html, hypothesisID string) error {
	event := api.ResearchEvent{
		SessionID:    sessionID,
		EventType:    eventType,
		HypothesisID: hypothesisID,
		Data: map[string]interface{}{
			"html_fragment": html,
			"timestamp":     time.Now().Unix(),
		},
		Timestamp: time.Now(),
	}

	b.sseHub.Broadcast(event)
	log.Printf("[UI Broadcast] Sent %s fragment for session %s (%d chars)", eventType, sessionID, len(html))
	return nil
}

// BufferEvidenceUpdate debounces evidence updates for performance
func (b *ResearchUIBroadcaster) BufferEvidenceUpdate(sessionID, hypothesisID string, data map[string]interface{}) {
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()

	key := sessionID + ":" + hypothesisID

	if b.updateBuffers[key] == nil {
		b.updateBuffers[key] = &UpdateBuffer{
			SessionID:    sessionID,
			HypothesisID: hypothesisID,
			PendingData:  make(map[string]interface{}),
		}
	}

	buffer := b.updateBuffers[key]
	buffer.PendingData = data
	buffer.LastUpdate = time.Now()

	// Reset debounce timer (500ms delay)
	if b.debounceTimer != nil {
		b.debounceTimer.Stop()
	}
	b.debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
		b.flushPendingUpdates()
	})
}

// flushPendingUpdates sends batched updates
func (b *ResearchUIBroadcaster) flushPendingUpdates() {
	b.bufferMu.Lock()
	defer b.bufferMu.Unlock()

	for key, buffer := range b.updateBuffers {
		// Generate HTML fragment for evidence update
		html := fmt.Sprintf(`
<div id="evidence-%s" hx-swap-oob="outerHTML:#evidence-%s">
    <div class="evidence-bar" style="width: %.1f%%"></div>
    <div class="status">E-value: %.2f | Confidence: %.1f%%</div>
</div>`, buffer.HypothesisID, buffer.HypothesisID, 75.0, 2.1, 85.0)

		b.broadcastHTMLFragment(buffer.SessionID, "evidence_update", html, buffer.HypothesisID)
		delete(b.updateBuffers, key)
	}
}
