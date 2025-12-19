package ui

import (
	"context"
	"log"
	"net/http"
	"time"

	"gohypo/internal/api"
	"gohypo/internal/research"
	"gohypo/ui/services"

	"github.com/gin-gonic/gin"
)

type ResearchHandler struct {
	dataService *services.DataService
}

func NewResearchHandler(dataService *services.DataService) *ResearchHandler {
	return &ResearchHandler{
		dataService: dataService,
	}
}

func (h *ResearchHandler) HandleInitiateResearch(sessionMgr *research.SessionManager, worker *research.ResearchWorker, sseHub *api.SSEHub) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] 🚀 INITIATING RESEARCH SESSION - REQUEST RECEIVED")

		fieldMetadata, err := h.dataService.GetFieldMetadata()
		if err != nil {
			log.Printf("[API] ❌ Failed to get field metadata: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve field metadata",
			})
			return
		}

		statsArtifacts, err := h.dataService.GetStatisticalArtifacts()
		if err != nil {
			log.Printf("[API] ❌ Failed to get statistical artifacts: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve statistical artifacts",
			})
			return
		}

		log.Printf("[API] 📊 Found %d fields and %d statistical artifacts for research analysis", len(fieldMetadata), len(statsArtifacts))

		if len(fieldMetadata) == 0 {
			log.Printf("[API] ⚠️ No fields available - aborting research")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No field metadata available for research",
			})
			return
		}

		session, err := sessionMgr.CreateSession(c.Request.Context(), map[string]interface{}{
			"field_count":           len(fieldMetadata),
			"stats_artifacts_count": len(statsArtifacts),
			"timestamp":             time.Now(),
		})
		if err != nil {
			log.Printf("[API] ❌ Failed to create session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to create research session",
			})
			return
		}

		log.Printf("[API] 🆔 Created new session ID: %s", session.ID)

		// Emit SSE event for session creation
		sseHub.Broadcast(api.ResearchEvent{
			SessionID: session.ID.String(),
			EventType: "session_created",
			Progress:  0.0,
			Data: map[string]interface{}{
				"field_count":           len(fieldMetadata),
				"stats_artifacts_count": len(statsArtifacts),
				"message":               "Research session initialized",
			},
			Timestamp: time.Now(),
		})

		go func() {
			log.Printf("[WORKER] 🏁 Starting background research process for session %s", session.ID)
			worker.ProcessResearch(context.Background(), session.ID.String(), fieldMetadata, statsArtifacts, sseHub)
		}()

		log.Printf("[API] ✅ Research session %s successfully scheduled", session.ID)

		c.Header("HX-Trigger", "researchStarted")
		c.JSON(http.StatusAccepted, gin.H{
			"session_id":            session.ID,
			"status":                "accepted",
			"field_count":           len(fieldMetadata),
			"stats_artifacts_count": len(statsArtifacts),
			"estimated_duration":    "30-60 seconds",
		})
	}
}

func (h *ResearchHandler) HandleResearchStatus(sessionMgr *research.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		activeSessions, err := sessionMgr.GetActiveSessions(c.Request.Context())
		if err != nil {
			log.Printf("[API] ❌ Failed to get active sessions: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve session status",
			})
			return
		}

		var response gin.H
		if len(activeSessions) == 0 {
			allSessions, err := sessionMgr.ListSessions(c.Request.Context(), nil)
			if err != nil {
				log.Printf("[API] ❌ Failed to get all sessions: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Failed to retrieve session status",
				})
				return
			}
			if len(allSessions) > 0 {
				session := allSessions[0]
				status := session.GetStatus()
				response = gin.H{
					"session_id":         status["id"],
					"state":              status["state"],
					"progress":           status["progress"],
					"current_hypothesis": status["current_hypothesis"],
					"completed_count":    status["completed_count"],
					"error":              status["error"],
				}
			} else {
				response = gin.H{
					"state":    "idle",
					"progress": 0,
				}
			}
		} else {
			session := activeSessions[0]
			status := session.GetStatus()
			response = gin.H{
				"session_id":         status["id"],
				"state":              status["state"],
				"progress":           status["progress"],
				"current_hypothesis": status["current_hypothesis"],
				"completed_count":    status["completed_count"],
			}
		}

		c.JSON(http.StatusOK, response)
	}
}
