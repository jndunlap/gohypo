package api

import (
	"encoding/json"
	"io"
	"log"
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
	Progress     float64                `json:"progress"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
}

// SSEHub manages Server-Sent Events for real-time research updates
type SSEHub struct {
	clients    map[string]map[chan ResearchEvent]bool
	clientsMu  sync.RWMutex
	register   chan SSEClient
	unregister chan SSEClient
	broadcast  chan ResearchEvent
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
	return hub
}

// run processes SSE hub operations
func (h *SSEHub) run() {
	for {
		select {
		case client := <-h.register:
			h.clientsMu.Lock()
			if h.clients[client.SessionID] == nil {
				h.clients[client.SessionID] = make(map[chan ResearchEvent]bool)
			}
			h.clients[client.SessionID][client.Channel] = true
			log.Printf("[SSE] Client registered for session %s (total clients: %d)",
				client.SessionID, len(h.clients[client.SessionID]))
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
	if sessionID == "" {
		c.JSON(400, gin.H{"error": "session_id parameter required"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Create client channel
	clientChan := make(chan ResearchEvent, 10)

	// Register client
	select {
	case h.register <- SSEClient{SessionID: sessionID, Channel: clientChan}:
	default:
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
			eventJSON, err := json.Marshal(event)
			if err != nil {
				log.Printf("[SSE] Failed to marshal event: %v", err)
				return true
			}

			c.SSEvent("research", string(eventJSON))
			return true

		case <-time.After(30 * time.Second):
			// Send ping to keep connection alive
			c.SSEvent("ping", `{"status": "alive", "timestamp": "`+time.Now().Format(time.RFC3339)+`"}`)
			return true

		case <-ctx.Done():
			// Client disconnected
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

