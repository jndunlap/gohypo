package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SSEClient represents a connected SSE client
type SSEClient struct {
	SessionID string
	Channel   chan ResearchEvent
}

// ResearchEvent represents a research event for SSE streaming
type ResearchEvent struct {
	SessionID    string                 `json:"session_id"`
	EventType    string                 `json:"event_type"`
	HypothesisID string                 `json:"hypothesis_id,omitempty"`
	DatasetID    string                 `json:"dataset_id,omitempty"`
	Progress     float64                `json:"progress"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// SSEHub manages Server-Sent Events for real-time research updates
type SSEHub struct {
	clients      map[string]map[chan ResearchEvent]bool
	clientsMu    sync.RWMutex
	register     chan SSEClient
	unregister   chan SSEClient
	broadcast    chan ResearchEvent
	sessionMgr   interface{} // Will hold session manager reference
	cleanupTimer *time.Timer
}

// NewSSEHub creates a new SSE hub
func NewSSEHub() *SSEHub {
	hub := &SSEHub{
		clients:    make(map[string]map[chan ResearchEvent]bool),
		register:   make(chan SSEClient, 10),
		unregister: make(chan SSEClient, 10),
		broadcast:  make(chan ResearchEvent, 100),
	}

	go hub.run()
	go hub.startSessionCleanup()

	return hub
}

// SetSessionManager sets the session manager for validation
func (h *SSEHub) SetSessionManager(sessionMgr interface{}) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()
	h.sessionMgr = sessionMgr
}

// run processes SSE hub operations
func (h *SSEHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			sessionID := client.SessionID
			if sessionID == "" {
				sessionID = "" // Use empty string as key for global listeners
			}
			if h.clients[sessionID] == nil {
				h.clients[sessionID] = make(map[chan ResearchEvent]bool)
			}
			h.clients[sessionID][client.Channel] = true
			log.Printf("[SSE] Client registered for session %s (total clients: %d)",
				sessionID, len(h.clients[sessionID]))
			h.clientsMu.Unlock()

		case client := <-h.unregister:
			h.clientsMu.Lock()
			if clients, exists := h.clients[client.SessionID]; exists {
				delete(clients, client.Channel)
				close(client.Channel)
				log.Printf("[SSE] Client unregistered from session %s (remaining clients: %d)",
					client.SessionID, len(clients))
				if len(clients) == 0 {
					delete(h.clients, client.SessionID)
				}
			}
			h.clientsMu.Unlock()

		case event := <-h.broadcast:
			h.clientsMu.RLock()

			// Send to clients registered for the specific session
			if event.SessionID != "" {
				if clients, exists := h.clients[event.SessionID]; exists {
					for clientChan := range clients {
						select {
						case clientChan <- event:
							// Event sent successfully
						default:
							// Client channel is full, skip
							log.Printf("[SSE] Client channel full for session %s, skipping event",
								event.SessionID)
						}
					}
				}
			}

			// Also send to clients registered without a session ID (global listeners)
			if clients, exists := h.clients[""]; exists {
				for clientChan := range clients {
					select {
					case clientChan <- event:
						// Event sent successfully
					default:
						// Client channel is full, skip
						log.Printf("[SSE] Global client channel full, skipping event")
					}
				}
			}

			h.clientsMu.RUnlock()
		}
	}
}

// Broadcast sends an event to all clients listening to a session
func (h *SSEHub) Broadcast(event ResearchEvent) {
	select {
	case h.broadcast <- event:
	default:
		log.Printf("[SSE] Broadcast channel full, dropping event: %s", event.EventType)
	}
}

// HandleSSE handles Server-Sent Events endpoint
func (h *SSEHub) HandleSSE(c *gin.Context) {
	sessionID := c.Query("session_id")
	reconnectParam := c.Query("reconnect")

	// Validate session if provided
	if sessionID != "" && h.sessionMgr != nil {
		// Check if session manager has IsSessionActive method
		if sessionValidator, ok := h.sessionMgr.(interface{ IsSessionActive(string) bool }); ok {
			if !sessionValidator.IsSessionActive(sessionID) {
				log.Printf("[SSE] Rejecting connection for inactive session: %s", sessionID)
				c.JSON(400, gin.H{"error": "Session not found or inactive"})
				return
			}
		}
	}

	// Log reconnection attempts
	if reconnectParam != "" {
		log.Printf("[SSE] Reconnection attempt for session: %s", sessionID)
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Create client channel
	clientChan := make(chan ResearchEvent, 10)

	// Register client (sessionID is optional now)
	client := SSEClient{Channel: clientChan}
	if sessionID != "" {
		client.SessionID = sessionID
	}

	select {
	case h.register <- client:
		log.Printf("[SSE] Client registered successfully for session: %s", client.SessionID)
	default:
		log.Printf("[SSE] SSE hub registration failed - channel full")
		c.JSON(500, gin.H{"error": "SSE hub registration failed"})
		return
	}

	defer func() {
		select {
		case h.unregister <- SSEClient{SessionID: sessionID, Channel: clientChan}:
		default:
			// Hub might be overloaded, just close channel
		}
	}()

	// Keep connection alive and stream events
	ctx := c.Request.Context()
	c.Stream(func(w io.Writer) bool {
		select {
		case event := <-clientChan:
			eventJSON, err := json.Marshal(event.Data)
			if err != nil {
				log.Printf("[SSE] Failed to marshal event data: %v", err)
				return true // Continue streaming despite marshal error
			}

			// Send HTMX-compatible SSE event using event type as the event name
			log.Printf("[SSE] Broadcasting event: %s to clients", event.EventType)
			c.SSEvent(event.EventType, string(eventJSON))
			// Force flush to ensure data is sent immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return true

		case <-time.After(30 * time.Second):
			// Send ping to keep connection alive with properly formatted JSON
			pingData := map[string]interface{}{
				"status":    "alive",
				"timestamp": time.Now().Format(time.RFC3339),
			}
			pingJSON, err := json.Marshal(pingData)
			if err != nil {
				log.Printf("[SSE] Failed to marshal ping: %v", err)
				return true
			}
			c.SSEvent("ping", string(pingJSON))
			// Force flush to ensure data is sent immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return true

		case <-ctx.Done():
			// Client disconnected
			log.Printf("[SSE] Client disconnected for session %s", sessionID)
			return false
		}
	})
}

// GetActiveSessions returns sessions with active SSE clients
func (h *SSEHub) GetActiveSessions() []string {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	sessions := make([]string, 0, len(h.clients))
	for sessionID := range h.clients {
		sessions = append(sessions, sessionID)
	}
	return sessions
}

// GetClientCount returns the number of active clients for a session
func (h *SSEHub) GetClientCount(sessionID string) int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	if clients, exists := h.clients[sessionID]; exists {
		return len(clients)
	}
	return 0
}

// UploadProgressEvent represents an upload progress event
type UploadProgressEvent struct {
	SessionID string                 `json:"session_id"`
	EventType string                 `json:"event_type"` // "upload_started", "upload_progress", "upload_completed", "upload_failed"
	DatasetID string                 `json:"dataset_id"`
	Progress  float64                `json:"progress"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// BroadcastUploadProgress sends upload progress to connected clients
func (h *SSEHub) BroadcastUploadProgress(event UploadProgressEvent) {
	// Convert to ResearchEvent for compatibility
	researchEvent := ResearchEvent{
		SessionID: event.SessionID,
		EventType: event.EventType,
		DatasetID: event.DatasetID,
		Progress:  event.Progress,
		Data: map[string]interface{}{
			"message": event.Message,
			"data":    event.Data,
		},
		Timestamp: event.Timestamp,
	}

	h.Broadcast(researchEvent)
}

// ===== SESSION CLEANUP =====

// startSessionCleanup begins periodic cleanup of stale sessions
func (h *SSEHub) startSessionCleanup() {
	h.cleanupTimer = time.AfterFunc(30*time.Second, h.performSessionCleanup)
}

// performSessionCleanup removes connections for inactive sessions
func (h *SSEHub) performSessionCleanup() {
	defer func() {
		// Schedule next cleanup
		if h.cleanupTimer != nil {
			h.cleanupTimer = time.AfterFunc(30*time.Second, h.performSessionCleanup)
		}
	}()

	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if h.sessionMgr == nil {
		return // No session manager, skip cleanup
	}

	// Check if session manager has IsSessionActive method
	sessionValidator, ok := h.sessionMgr.(interface{ IsSessionActive(string) bool })
	if !ok {
		return // Session manager doesn't support validation
	}

	cleanedSessions := 0
	cleanedConnections := 0

	for sessionID, clients := range h.clients {
		// Skip empty session ID (global listeners)
		if sessionID == "" {
			continue
		}

		// Check if session is still active
		if !sessionValidator.IsSessionActive(sessionID) {
			log.Printf("[SSE] Cleaning up stale session: %s (%d connections)", sessionID, len(clients))

			// Close all client connections for this session
			for clientChan := range clients {
				select {
				case <-clientChan:
					// Channel already closed
				default:
					close(clientChan)
				}
			}

			delete(h.clients, sessionID)
			cleanedSessions++
			cleanedConnections += len(clients)
		}
	}

	if cleanedSessions > 0 {
		log.Printf("[SSE] Session cleanup completed: %d sessions, %d connections removed", cleanedSessions, cleanedConnections)
	}
}

// GetActiveSessionCount returns the number of active sessions with connections
func (h *SSEHub) GetActiveSessionCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	activeSessions := 0
	for sessionID, clients := range h.clients {
		if sessionID != "" && len(clients) > 0 {
			activeSessions++
		}
	}

	return activeSessions
}
