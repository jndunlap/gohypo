package ui

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"gohypo/adapters/excel"
	"gohypo/adapters/llm"
	"gohypo/app"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"gohypo/internal/research"
	"gohypo/ports"

	"github.com/gin-gonic/gin"
)

func (s *Server) AddResearchRoutes(sessionMgr *research.SessionManager, storage *research.ResearchStorage, worker *research.ResearchWorker) {
	api := s.router.Group("/api/research")
	{
		api.POST("/initiate", s.handleInitiateResearch(sessionMgr, worker))
		api.GET("/status", s.handleResearchStatus(sessionMgr))
		api.GET("/ledger", s.handleResearchLedger(storage))
		api.GET("/download/:id", s.handleDownloadHypothesis(storage))
		api.GET("/industry-context", s.handleIndustryContext())
	}
}

func (s *Server) handleInitiateResearch(sessionMgr *research.SessionManager, worker *research.ResearchWorker) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Printf("[API] 🚀 INITIATING RESEARCH SESSION - REQUEST RECEIVED")

		fieldMetadata, err := s.getFieldMetadata()
		if err != nil {
			log.Printf("[API] ❌ Failed to get field metadata: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve field metadata",
			})
			return
		}

		statsArtifacts, err := s.getStatisticalArtifacts()
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

		session := sessionMgr.CreateSession(map[string]interface{}{
			"field_count":           len(fieldMetadata),
			"stats_artifacts_count": len(statsArtifacts),
			"timestamp":             time.Now(),
		})

		log.Printf("[API] 🆔 Created new session ID: %s", session.ID)

		go func() {
			log.Printf("[WORKER] 🏁 Starting background research process for session %s", session.ID)
			worker.ProcessResearch(session.ID, fieldMetadata, statsArtifacts)
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

func (s *Server) handleResearchStatus(sessionMgr *research.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		activeSessions := sessionMgr.GetActiveSessions()

		var response gin.H
		if len(activeSessions) == 0 {
			allSessions := sessionMgr.ListSessions(nil)
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

func (s *Server) handleResearchLedger(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		limitStr := c.DefaultQuery("limit", "10")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit < 1 || limit > 100 {
			limit = 10
		}

		hypotheses, err := storage.ListRecent(limit)
		if err != nil {
			log.Printf("[API] Failed to list hypotheses: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve hypotheses",
			})
			return
		}

		if c.GetHeader("HX-Request") == "true" {
			c.Header("Content-Type", "text/html")
			html := s.renderHypothesisCards(hypotheses)
			c.String(http.StatusOK, html)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"hypotheses": hypotheses,
			"count":      len(hypotheses),
		})
	}
}

func (s *Server) handleDownloadHypothesis(storage *research.ResearchStorage) gin.HandlerFunc {
	return func(c *gin.Context) {
		hypothesisID := c.Param("id")

		hypothesis, err := storage.GetByID(hypothesisID)
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

func (s *Server) getFieldMetadata() ([]greenfield.FieldMetadata, error) {
	filters := ports.ArtifactFilters{Limit: 1000}
	allArtifacts, err := s.reader.ListArtifacts(context.Background(), filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	fieldMap := make(map[string]*greenfield.FieldMetadata)
	relationshipFields := 0
	profileFields := 0
	excelFields := 0

	log.Printf("[API] 📊 Analyzing %d artifacts for field metadata", len(allArtifacts))

	if excelData, columnTypes, err := s.getExcelFieldMetadata(); err == nil {
		for _, fieldName := range excelData.Headers {
			if _, exists := fieldMap[fieldName]; !exists {
				dataType := columnTypes[fieldName]
				if dataType == "" {
					dataType = "unknown"
				}
				fieldMap[fieldName] = &greenfield.FieldMetadata{
					Name:     fieldName,
					DataType: dataType,
				}
				excelFields++
			}
		}
		log.Printf("[API] 📊 Added %d fields directly from Excel file with inferred types", excelFields)
	}

	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			var varX, varY string

			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					varX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					varY = vy
				}
			}

			if varX != "" {
				if _, exists := fieldMap[varX]; !exists {
					fieldMap[varX] = &greenfield.FieldMetadata{
						Name:     varX,
						DataType: "numeric", // Default assumption
					}
					relationshipFields++
				}
			}
			if varY != "" {
				if _, exists := fieldMap[varY]; !exists {
					fieldMap[varY] = &greenfield.FieldMetadata{
						Name:     varY,
						DataType: "numeric", // Default assumption
					}
					relationshipFields++
				}
			}
		} else if artifact.Kind == core.ArtifactVariableProfile {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if varKey, ok := payload["variable_key"].(string); ok && varKey != "" {
					if _, exists := fieldMap[varKey]; !exists {
						dataType := "numeric" // Default
						if variance, ok := payload["variance"].(float64); ok && variance == 0 {
							if cardinality, ok := payload["cardinality"].(float64); ok && cardinality > 0 && cardinality < 10 {
								dataType = "categorical"
							}
						}
						fieldMap[varKey] = &greenfield.FieldMetadata{
							Name:     varKey,
							DataType: dataType,
						}
						profileFields++
					}
				}
			}
		}
	}

	var metadata []greenfield.FieldMetadata
	for _, field := range fieldMap {
		metadata = append(metadata, *field)
	}

	log.Printf("[API] 📊 Field collection complete: %d from Excel, %d from relationships, %d from profiles, %d total unique fields",
		excelFields, relationshipFields, profileFields, len(metadata))

	return metadata, nil
}

func (s *Server) getStatisticalArtifacts() ([]map[string]interface{}, error) {
	filters := ports.ArtifactFilters{Limit: 1000}
	allArtifacts, err := s.reader.ListArtifacts(context.Background(), filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	var statsArtifacts []map[string]interface{}
	statArtifactCount := 0

	for _, artifact := range allArtifacts {
		switch artifact.Kind {
		case core.ArtifactRelationship:
			artifactData := map[string]interface{}{
				"kind":       string(artifact.Kind),
				"id":         artifact.ID,
				"payload":    artifact.Payload,
				"created_at": artifact.CreatedAt,
			}
			statsArtifacts = append(statsArtifacts, artifactData)
			statArtifactCount++

		case core.ArtifactSweepManifest:
			artifactData := map[string]interface{}{
				"kind":       string(artifact.Kind),
				"id":         artifact.ID,
				"payload":    artifact.Payload,
				"created_at": artifact.CreatedAt,
			}
			statsArtifacts = append(statsArtifacts, artifactData)
			statArtifactCount++

		case core.ArtifactFDRFamily:
			artifactData := map[string]interface{}{
				"kind":       string(artifact.Kind),
				"id":         artifact.ID,
				"payload":    artifact.Payload,
				"created_at": artifact.CreatedAt,
			}
			statsArtifacts = append(statsArtifacts, artifactData)
			statArtifactCount++

		case core.ArtifactVariableHealth:
			artifactData := map[string]interface{}{
				"kind":       string(artifact.Kind),
				"id":         artifact.ID,
				"payload":    artifact.Payload,
				"created_at": artifact.CreatedAt,
			}
			statsArtifacts = append(statsArtifacts, artifactData)
			statArtifactCount++
		}
	}

	log.Printf("[API] 📈 Collected %d statistical artifacts with test scores", statArtifactCount)
	return statsArtifacts, nil
}

func (s *Server) renderHypothesisCards(hypotheses []*research.HypothesisResult) string {
	data := struct {
		Hypotheses []*research.HypothesisResult
	}{
		Hypotheses: hypotheses,
	}

	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "fragments/hypothesis_cards_grid.html", data); err != nil {
		log.Printf("[ERROR] Failed to render hypothesis cards template: %v", err)
		return `<div class="text-center py-12 text-red-600">Error rendering hypothesis cards</div>`
	}

	return buf.String()
}

func (s *Server) handleIndustryContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the greenfield service
		greenfieldSvc, ok := s.greenfieldService.(*app.GreenfieldService)
		if !ok || greenfieldSvc == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Greenfield service not available",
			})
			return
		}

		// Get the port which has the adapter with Scout
		port := greenfieldSvc.GetGreenfieldPort()
		if port == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Greenfield port not available",
			})
			return
		}

		// Access the adapter's Scout directly
		adapter, ok := port.(*llm.GreenfieldAdapter)
		if !ok {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "Unable to access Forensic Scout",
			})
			return
		}

		// Extract industry context using the Scout
		ctx := context.Background()
		industryContext, err := adapter.GetScout().ExtractIndustryContext(ctx)
		if err != nil {
			log.Printf("[API] Failed to extract industry context: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to extract industry context",
				"details": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"industry_context": industryContext,
		})
	}
}

func (s *Server) getExcelFieldMetadata() (*excel.ExcelData, map[string]string, error) {
	// Get Excel file path from environment
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		return nil, nil, fmt.Errorf("EXCEL_FILE environment variable not set")
	}

	// Read Excel data
	reader := excel.NewExcelReader(excelFile)
	data, err := reader.ReadData()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read Excel data: %w", err)
	}

	// Infer column types
	columnTypes, err := reader.InferColumnTypes(data)
	if err != nil {
		log.Printf("[API] ⚠️ Failed to infer column types, using 'unknown': %v", err)
		// Don't fail completely, just use unknown types
		columnTypes = make(map[string]string)
		for _, header := range data.Headers {
			columnTypes[header] = "unknown"
		}
	}

	return data, columnTypes, nil
}
