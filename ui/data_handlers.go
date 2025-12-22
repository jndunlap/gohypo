package ui

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

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

			if c.GetHeader("HX-Request") == "true" {
				c.Header("Content-Type", "text/html")
				html := h.renderService.RenderHypothesisError("Failed to retrieve hypotheses from database")
				c.String(http.StatusInternalServerError, html)
				return
			}

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

func (h *DataHandler) HandleCurrentPrompt(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Look for the most recent prompt file in the research_prompts directory
		promptFiles, err := filepath.Glob("research_prompts/*.txt")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to scan prompt files",
			})
			return
		}

		if len(promptFiles) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "No prompt files found",
			})
			return
		}

		// Find the most recent prompt file
		var latestFile string
		var latestTime time.Time

		for _, file := range promptFiles {
			info, err := os.Stat(file)
			if err != nil {
				continue
			}
			if info.ModTime().After(latestTime) {
				latestTime = info.ModTime()
				latestFile = file
			}
		}

		if latestFile == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "No valid prompt files found",
			})
			return
		}

		// Read and return the prompt content
		content, err := os.ReadFile(latestFile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to read prompt file",
			})
			return
		}

		c.Header("Content-Type", "text/plain; charset=utf-8")
		c.String(http.StatusOK, string(content))
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

func (h *DataHandler) HandleHypothesisCard(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")

		hypothesis, err := storage.GetByID(c.Request.Context(), idStr)
		if err != nil {
			log.Printf("[API] Failed to get hypothesis %s: %v", idStr, err)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Hypothesis not found",
			})
			return
		}

		if c.GetHeader("HX-Request") == "true" {
			c.Header("Content-Type", "text/html")
			html := h.renderService.RenderHypothesisCard(hypothesis)
			c.String(http.StatusOK, html)
			return
		}

		c.JSON(http.StatusOK, hypothesis)
	}
}

// HandleHypothesisToggle handles expanding/collapsing hypothesis cards
func (h *DataHandler) HandleHypothesisToggle(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")

		hypothesis, err := storage.GetByID(c.Request.Context(), idStr)
		if err != nil {
			log.Printf("[API] Failed to get hypothesis %s: %v", idStr, err)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Hypothesis not found",
			})
			return
		}

		if c.GetHeader("HX-Request") == "true" {
			c.Header("Content-Type", "text/html")
			// Return the expanded/collapsed card HTML
			html := h.renderService.RenderHypothesisCardExpanded(hypothesis)
			c.String(http.StatusOK, html)
			return
		}

		c.JSON(http.StatusOK, hypothesis)
	}
}

// HandleHypothesisEvidence handles showing/hiding evidence drawer
func (h *DataHandler) HandleHypothesisEvidence(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")

		hypothesis, err := storage.GetByID(c.Request.Context(), idStr)
		if err != nil {
			log.Printf("[API] Failed to get hypothesis %s: %v", idStr, err)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Hypothesis not found",
			})
			return
		}

		if c.GetHeader("HX-Request") == "true" {
			c.Header("Content-Type", "text/html")
			// Return the evidence drawer HTML
			html := h.renderService.RenderHypothesisEvidence(hypothesis)
			c.String(http.StatusOK, html)
			return
		}

		c.JSON(http.StatusOK, hypothesis)
	}
}
