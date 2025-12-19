package ui

import (
	"context"
	"log"
	"net/http"

	"gohypo/adapters/llm"
	"gohypo/app"

	"github.com/gin-gonic/gin"
)

type IndustryHandler struct {
	greenfieldService interface{} // We'll keep this flexible
}

func NewIndustryHandler(greenfieldService interface{}) *IndustryHandler {
	return &IndustryHandler{
		greenfieldService: greenfieldService,
	}
}

func (h *IndustryHandler) HandleIndustryContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[IndustryContext] API called - fetching industry intelligence")

		// Get the greenfield service
		greenfieldSvc, ok := h.greenfieldService.(*app.GreenfieldService)
		if !ok || greenfieldSvc == nil {
			log.Printf("[IndustryContext] Greenfield service not available")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Greenfield service not available",
			})
			return
		}

		// Get the port which has the adapter with Scout
		port := greenfieldSvc.GetGreenfieldPort()
		if port == nil {
			log.Printf("[IndustryContext] Greenfield port not available")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Greenfield port not available",
			})
			return
		}

		// Access the adapter's Scout directly
		adapter, ok := port.(*llm.GreenfieldAdapter)
		if !ok {
			log.Printf("[IndustryContext] Unable to access Forensic Scout adapter")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Unable to access Forensic Scout",
			})
			return
		}

		// Extract industry context using the Scout
		ctx := context.Background()
		log.Printf("[IndustryContext] Calling Forensic Scout to extract intelligence")
		scoutResponse, err := adapter.GetScout().ExtractIndustryContext(ctx)
		if err != nil {
			log.Printf("[IndustryContext] Failed to extract industry intelligence: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to extract industry intelligence",
				"details": err.Error(),
			})
			return
		}

		log.Printf("[IndustryContext] Successfully extracted intelligence: Domain='%s', Context='%s'",
			scoutResponse.Domain, scoutResponse.Context)

		c.JSON(http.StatusOK, gin.H{
			"domain":     scoutResponse.Domain,
			"context":    scoutResponse.Context,
			"bottleneck": scoutResponse.Bottleneck,
			"physics":    scoutResponse.Physics,
			"map":        scoutResponse.Map,
		})
	}
}
