package ui

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"gohypo/internal/api"
	"gohypo/internal/research"
	"gohypo/ui/services"

	"github.com/google/uuid"

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

		// Extract workspace ID from request (supports both JSON and form data)
		var workspaceIDStr string
		var err error

		if c.GetHeader("Content-Type") == "application/json" {
			var requestBody struct {
				WorkspaceID string `json:"workspace_id"`
			}
			if err = c.ShouldBindJSON(&requestBody); err != nil {
				log.Printf("[API] ❌ Invalid JSON request body: %v", err)
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "Invalid request body - workspace_id required",
				})
				return
			}
			workspaceIDStr = requestBody.WorkspaceID
		} else {
			// Handle form data from HTMX
			workspaceIDStr = c.PostForm("workspace_id")
			if workspaceIDStr == "" {
				// Try query parameter as fallback
				workspaceIDStr = c.DefaultPostForm("workspace_id", "550e8400-e29b-41d4-a716-446655440001")
			}
		}

		if workspaceIDStr == "" {
			log.Printf("[API] ❌ No workspace ID provided")
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "workspace_id is required",
			})
			return
		}

		workspaceID, err := uuid.Parse(workspaceIDStr)
		if err != nil {
			log.Printf("[API] ❌ Invalid workspace ID: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid workspace_id format",
			})
			return
		}

		// Get workspace datasets only
		fieldMetadata, err := h.dataService.GetFieldMetadataByWorkspace(workspaceID)
		if err != nil {
			log.Printf("[API] ❌ Failed to get field metadata for workspace %s: %v", workspaceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve field metadata for workspace",
			})
			return
		}

		statsArtifacts, err := h.dataService.GetStatisticalArtifactsByWorkspace(workspaceID)
		if err != nil {
			log.Printf("[API] ❌ Failed to get statistical artifacts for workspace %s: %v", workspaceID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve statistical artifacts for workspace",
			})
			return
		}

		log.Printf("[API] 📊 Found %d fields and %d statistical artifacts for workspace %s research analysis", len(fieldMetadata), len(statsArtifacts), workspaceID)

		if len(fieldMetadata) == 0 {
			log.Printf("[API] ⚠️ No fields available in workspace %s - aborting research", workspaceID)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No field metadata available in this workspace",
			})
			return
		}

		session, err := sessionMgr.CreateSessionInWorkspace(c.Request.Context(), workspaceID.String(), map[string]interface{}{
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

		// Check if this is an HTMX request
		if c.GetHeader("HX-Request") == "true" {
			// Return HTMX-compatible HTML for the research status area
			html := fmt.Sprintf(`
				<div class="bg-green-50 border border-green-200 rounded-lg p-4">
					<div class="flex items-center">
						<div class="flex-shrink-0">
							<svg class="h-5 w-5 text-green-400" fill="currentColor" viewBox="0 0 20 20">
								<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/>
							</svg>
						</div>
						<div class="ml-3">
							<h3 class="text-sm font-medium text-green-800">Research Started!</h3>
							<div class="mt-2 text-sm text-green-700">
								<p>Session ID: %s</p>
								<p>Processing %d fields with %d statistical artifacts</p>
								<p>Estimated completion: 30-60 seconds</p>
							</div>
						</div>
					</div>
				</div>
			`, session.ID, len(fieldMetadata), len(statsArtifacts))

			c.Header("HX-Trigger", "researchStarted")
			c.Header("Content-Type", "text/html")
			c.String(http.StatusAccepted, html)
			return
		}

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
