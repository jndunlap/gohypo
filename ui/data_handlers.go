package ui

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"gohypo/internal/research"
	"gohypo/ui/services"

	"github.com/gin-gonic/gin"
)

type DataHandler struct {
	renderService *services.RenderService
}

func NewDataHandler(renderService *services.RenderService) *DataHandler {
	return &DataHandler{
		renderService: renderService,
	}
}

func (h *DataHandler) HandleResearchLedger(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		limitStr := c.DefaultQuery("limit", "10")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			limit = 10
		}

		hypotheses, err := storage.ListRecent(c.Request.Context(), limit)
		if err != nil {
			log.Printf("[API] Failed to list hypotheses: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve hypotheses",
			})
			return
		}

		if c.GetHeader("HX-Request") == "true" {
			c.Header("Content-Type", "text/html")
			html := h.renderService.RenderHypothesisCards(hypotheses)
			c.String(http.StatusOK, html)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"hypotheses": hypotheses,
			"count":      len(hypotheses),
		})
	}
}

func (h *DataHandler) HandleDownloadHypothesis(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		hypothesisID := c.Param("id")

		hypothesis, err := storage.GetByID(c.Request.Context(), hypothesisID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Hypothesis not found",
			})
			return
		}

		filename := fmt.Sprintf("hypothesis_%s.json", hypothesisID)
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		c.Header("Content-Type", "application/json")

		c.JSON(http.StatusOK, hypothesis)
	}
}
