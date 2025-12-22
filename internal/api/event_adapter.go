package api

import (
	"gohypo/internal/analysis"
)

// SSEEventBroadcaster adapts the SSEHub to implement the EventBroadcaster interface
type SSEEventBroadcaster struct {
	sseHub *SSEHub
}

// NewSSEEventBroadcaster creates a new SSE event broadcaster
func NewSSEEventBroadcaster(sseHub *SSEHub) *SSEEventBroadcaster {
	return &SSEEventBroadcaster{sseHub: sseHub}
}

// Broadcast sends a sequenced event via SSE
func (seb *SSEEventBroadcaster) Broadcast(sessionID string, event *analysis.SequencedEvent) {
	// Convert to ResearchEvent for SSE broadcasting
	researchEvent := ResearchEvent{
		SessionID:     event.SessionID,
		EventType:     event.EventType,
		HypothesisID:  event.HypothesisID,
		Progress:      event.Progress,
		Data:          event.Data,
		Timestamp:     event.Timestamp,
		SID:           event.SID,
		DependencySID: event.DependencySID,
		EventCategory: event.EventCategory,
	}

	// Add sequencing metadata to event data for backward compatibility
	if researchEvent.Data == nil {
		researchEvent.Data = make(map[string]interface{})
	}

	seb.sseHub.Broadcast(researchEvent)
}
