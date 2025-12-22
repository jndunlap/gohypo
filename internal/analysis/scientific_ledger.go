package analysis

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// EventBroadcaster defines the interface for broadcasting sequenced events
type EventBroadcaster interface {
	Broadcast(sessionID string, event *SequencedEvent)
}

// SequencedEvent represents an event with sequencing metadata
type SequencedEvent struct {
	SessionID     string
	EventType     string
	HypothesisID  string
	Progress      float64
	Data          map[string]interface{}
	Timestamp     time.Time
	SID           int64
	DependencySID int64
	EventCategory string
}

// ScientificLedger manages event sequencing and dependency tracking
// for the research process. Ensures evidence is presented before hypotheses
// that depend on it.
type ScientificLedger struct {
	seqMgr       *SequenceManager
	broadcaster  EventBroadcaster
	sessions     map[string]*LedgerSession
	sessionsMu   sync.RWMutex
	cleanupTimer *time.Timer
}

// LedgerSession tracks events and dependencies for a single research session
type LedgerSession struct {
	SessionID    string
	Events       []*LedgerEvent
	EventBySID   map[int64]*LedgerEvent
	PendingDeps  map[int64][]int64 // dependencySID -> []hypothesisSIDs waiting
	CreatedAt    time.Time
	LastActivity time.Time
	mu           sync.RWMutex
}

// LedgerEvent represents a sequenced event in the scientific ledger
type LedgerEvent struct {
	SID           int64
	DependencySID int64
	EventType     string
	EventCategory string
	HypothesisID  string
	Data          map[string]interface{}
	Timestamp     time.Time
	Broadcast     bool // whether this event has been broadcast
}

// NewScientificLedger creates a new scientific ledger coordinator
func NewScientificLedger(broadcaster EventBroadcaster) *ScientificLedger {
	sl := &ScientificLedger{
		seqMgr:      NewSequenceManager(),
		broadcaster: broadcaster,
		sessions:    make(map[string]*LedgerSession),
	}
	// Start cleanup timer for stale sessions (runs every 5 minutes)
	sl.cleanupTimer = time.AfterFunc(5*time.Minute, sl.cleanupStaleSessions)
	return sl
}

// RecordEvidence records statistical evidence with automatic SID assignment
func (sl *ScientificLedger) RecordEvidence(sessionID, eventType string, data map[string]interface{}) (int64, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("sessionID cannot be empty")
	}
	if eventType == "" {
		return 0, fmt.Errorf("eventType cannot be empty")
	}

	sid := sl.seqMgr.Next()
	session := sl.getOrCreateSession(sessionID)

	event := &LedgerEvent{
		SID:           sid,
		DependencySID: 0, // Evidence has no dependencies
		EventType:     eventType,
		EventCategory: "EVIDENCE",
		Data:          data,
		Timestamp:     time.Now(),
	}

	session.RecordEvent(event)
	if err := sl.broadcastEvent(event); err != nil {
		log.Printf("[ScientificLedger] Failed to broadcast evidence SID:%d: %v", sid, err)
		return sid, fmt.Errorf("failed to broadcast evidence: %w", err)
	}

	log.Printf("[ScientificLedger] Recorded evidence SID:%d for session:%s", sid, sessionID)
	return sid, nil
}

// RecordHypothesis records a hypothesis with its evidence dependency
func (sl *ScientificLedger) RecordHypothesis(sessionID, hypothesisID, eventType string, data map[string]interface{}, evidenceSID int64) (int64, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("sessionID cannot be empty")
	}
	if hypothesisID == "" {
		return 0, fmt.Errorf("hypothesisID cannot be empty")
	}
	if eventType == "" {
		return 0, fmt.Errorf("eventType cannot be empty")
	}

	sid := sl.seqMgr.Next()
	session := sl.getOrCreateSession(sessionID)

	event := &LedgerEvent{
		SID:           sid,
		DependencySID: evidenceSID,
		EventType:     eventType,
		EventCategory: "HYPOTHESIS",
		HypothesisID:  hypothesisID,
		Data:          data,
		Timestamp:     time.Now(),
	}

	session.RecordEvent(event)
	if err := sl.broadcastEvent(event); err != nil {
		log.Printf("[ScientificLedger] Failed to broadcast hypothesis SID:%d: %v", sid, err)
		return sid, fmt.Errorf("failed to broadcast hypothesis: %w", err)
	}

	log.Printf("[ScientificLedger] Recorded hypothesis SID:%d (depends on:%d) for session:%s", sid, evidenceSID, sessionID)
	return sid, nil
}

// RecordProgress records progress updates (no dependencies, always broadcast)
func (sl *ScientificLedger) RecordProgress(sessionID, eventType string, progress float64, data map[string]interface{}) (int64, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("sessionID cannot be empty")
	}
	if eventType == "" {
		return 0, fmt.Errorf("eventType cannot be empty")
	}

	sid := sl.seqMgr.Next()
	session := sl.getOrCreateSession(sessionID)

	event := &LedgerEvent{
		SID:           sid,
		DependencySID: 0,
		EventType:     eventType,
		EventCategory: "PROGRESS",
		Data:          data,
		Timestamp:     time.Now(),
	}

	session.RecordEvent(event)
	if err := sl.broadcastEvent(event); err != nil {
		log.Printf("[ScientificLedger] Failed to broadcast progress SID:%d: %v", sid, err)
		return sid, fmt.Errorf("failed to broadcast progress: %w", err)
	}

	return sid, nil
}

// MarkEvidenceRendered notifies that evidence has been displayed to the user
func (sl *ScientificLedger) MarkEvidenceRendered(sessionID string, sid int64) error {
	if sessionID == "" {
		return fmt.Errorf("sessionID cannot be empty")
	}
	if sid <= 0 {
		return fmt.Errorf("invalid SID: %d", sid)
	}

	session := sl.getSession(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	session.MarkRendered(sid)
	sl.checkPendingDependencies(sessionID)
	return nil
}

// GetSessionLedger returns the complete ledger for a session
func (sl *ScientificLedger) GetSessionLedger(sessionID string) *LedgerSession {
	sl.sessionsMu.RLock()
	defer sl.sessionsMu.RUnlock()
	return sl.sessions[sessionID]
}

// CleanupSession removes ledger data for completed sessions
func (sl *ScientificLedger) CleanupSession(sessionID string) {
	sl.sessionsMu.Lock()
	defer sl.sessionsMu.Unlock()

	if _, exists := sl.sessions[sessionID]; exists {
		delete(sl.sessions, sessionID)
		log.Printf("[ScientificLedger] Cleaned up session:%s", sessionID)
	}
}

// cleanupStaleSessions removes sessions that haven't had activity for more than 30 minutes
func (sl *ScientificLedger) cleanupStaleSessions() {
	sl.sessionsMu.Lock()
	defer sl.sessionsMu.Unlock()

	now := time.Now()
	staleThreshold := 30 * time.Minute
	cleaned := 0

	for sessionID, session := range sl.sessions {
		if now.Sub(session.LastActivity) > staleThreshold {
			delete(sl.sessions, sessionID)
			cleaned++
		}
	}

	if cleaned > 0 {
		log.Printf("[ScientificLedger] Cleaned up %d stale sessions", cleaned)
	}

	// Schedule next cleanup
	sl.cleanupTimer = time.AfterFunc(5*time.Minute, sl.cleanupStaleSessions)
}

// getOrCreateSession creates or retrieves a session ledger
func (sl *ScientificLedger) getOrCreateSession(sessionID string) *LedgerSession {
	sl.sessionsMu.Lock()
	defer sl.sessionsMu.Unlock()

	if sl.sessions[sessionID] == nil {
		now := time.Now()
		sl.sessions[sessionID] = &LedgerSession{
			SessionID:    sessionID,
			Events:       []*LedgerEvent{},
			EventBySID:   make(map[int64]*LedgerEvent),
			PendingDeps:  make(map[int64][]int64),
			CreatedAt:    now,
			LastActivity: now,
		}
	} else {
		// Update last activity time
		sl.sessions[sessionID].LastActivity = time.Now()
	}

	return sl.sessions[sessionID]
}

// getSession retrieves a session ledger (read-only)
func (sl *ScientificLedger) getSession(sessionID string) *LedgerSession {
	sl.sessionsMu.RLock()
	defer sl.sessionsMu.RUnlock()
	return sl.sessions[sessionID]
}

// broadcastEvent sends the event via the broadcaster interface
func (sl *ScientificLedger) broadcastEvent(event *LedgerEvent) error {
	if sl.broadcaster == nil {
		return fmt.Errorf("broadcaster not configured")
	}

	sessionID := sl.getSessionIDFromEvent(event)
	if sessionID == "" {
		return fmt.Errorf("could not determine session ID for event SID:%d", event.SID)
	}

	sequencedEvent := &SequencedEvent{
		SessionID:     sessionID,
		EventType:     event.EventType,
		HypothesisID:  event.HypothesisID,
		Progress:      sl.extractProgress(event),
		Data:          event.Data,
		Timestamp:     event.Timestamp,
		SID:           event.SID,
		DependencySID: event.DependencySID,
		EventCategory: event.EventCategory,
	}

	sl.broadcaster.Broadcast(sessionID, sequencedEvent)
	event.Broadcast = true
	return nil
}

// checkPendingDependencies releases hypotheses that were waiting for evidence
func (sl *ScientificLedger) checkPendingDependencies(sessionID string) {
	session := sl.getSession(sessionID)
	if session == nil {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Find all hypotheses waiting for dependencies that are now satisfied
	var toRelease []int64

	for depSID, waitingSIDs := range session.PendingDeps {
		if event := session.EventBySID[depSID]; event != nil && event.Broadcast {
			// This dependency is now satisfied
			toRelease = append(toRelease, waitingSIDs...)
			delete(session.PendingDeps, depSID)
		}
	}

	// Release the waiting hypotheses
	for _, hypothesisSID := range toRelease {
		if hypothesis := session.EventBySID[hypothesisSID]; hypothesis != nil && !hypothesis.Broadcast {
			if err := sl.broadcastEvent(hypothesis); err != nil {
				log.Printf("[ScientificLedger] Failed to release hypothesis SID:%d: %v", hypothesisSID, err)
			} else {
				log.Printf("[ScientificLedger] Released pending hypothesis SID:%d", hypothesisSID)
			}
		}
	}
}

// getSessionIDFromEvent extracts session ID from event context
func (sl *ScientificLedger) getSessionIDFromEvent(event *LedgerEvent) string {
	// Find the session this event belongs to
	sl.sessionsMu.RLock()
	defer sl.sessionsMu.RUnlock()

	for sessionID, session := range sl.sessions {
		if session.EventBySID[event.SID] != nil {
			return sessionID
		}
	}
	return ""
}

// extractProgress attempts to extract progress from event data
func (sl *ScientificLedger) extractProgress(event *LedgerEvent) float64 {
	if progress, ok := event.Data["progress"].(float64); ok {
		return progress
	}
	return 0.0
}

// LedgerSession methods

func (ls *LedgerSession) RecordEvent(event *LedgerEvent) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.Events = append(ls.Events, event)
	ls.EventBySID[event.SID] = event

	// Track pending dependencies for hypotheses
	if event.EventCategory == "HYPOTHESIS" && event.DependencySID > 0 {
		ls.PendingDeps[event.DependencySID] = append(ls.PendingDeps[event.DependencySID], event.SID)
	}
}

func (ls *LedgerSession) MarkRendered(sid int64) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if event := ls.EventBySID[sid]; event != nil {
		event.Broadcast = true
		ls.LastActivity = time.Now()
	}
}


// GetEvidenceChain returns the evidence chain for a hypothesis
func (ls *LedgerSession) GetEvidenceChain(hypothesisSID int64) []*LedgerEvent {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	hypothesis := ls.EventBySID[hypothesisSID]
	if hypothesis == nil || hypothesis.EventCategory != "HYPOTHESIS" {
		return nil
	}

	chain := []*LedgerEvent{}

	// Add the hypothesis itself
	chain = append(chain, hypothesis)

	// Add the primary evidence
	if hypothesis.DependencySID > 0 {
		if evidence := ls.EventBySID[hypothesis.DependencySID]; evidence != nil {
			chain = append(chain, evidence)
		}
	}

	return chain
}
