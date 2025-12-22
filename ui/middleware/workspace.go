package middleware

import (
	"log"

	"gohypo/domain/core"
	domainDataset "gohypo/domain/dataset"
	"gohypo/ports"

	"github.com/gin-gonic/gin"
)

// EnsureWorkspace is middleware that ensures a default workspace exists for the current user
func EnsureWorkspace(workspaceRepo ports.WorkspaceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if workspaceRepo == nil {
			log.Printf("[EnsureWorkspace] Workspace repository not available, skipping workspace check")
			c.Next()
			return
		}

		// For now, use the default user ID (same as used throughout the app)
		userID := core.ID("550e8400-e29b-41d4-a716-446655440000")

		// Check if default workspace exists
		_, err := workspaceRepo.GetDefaultForUser(c.Request.Context(), userID)
		if err != nil {
			// Default workspace doesn't exist, create it
			log.Printf("[EnsureWorkspace] Default workspace not found for user %s, creating one", userID)

			newWorkspace := domainDataset.NewDefaultWorkspace(userID)
			newWorkspace.ID = core.NewID()

			if err := workspaceRepo.Create(c.Request.Context(), newWorkspace); err != nil {
				log.Printf("[EnsureWorkspace] Failed to create default workspace: %v", err)
				// Don't fail the request, just log and continue
			} else {
				log.Printf("[EnsureWorkspace] Created default workspace %s for user %s", newWorkspace.ID, userID)
			}
		}

		c.Next()
	}
}
