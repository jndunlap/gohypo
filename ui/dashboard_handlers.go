package ui

import (
	"context"
	"fmt"
	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/models"
	"gohypo/ports"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleMissionControl(c *gin.Context) {
	c.Header("Content-Type", "text/html")
	template, err := s.embeddedFiles.ReadFile("ui/templates/dashboard.html")
	if err != nil {
		log.Printf("[MissionControl] Template not found: %v", err)
		c.String(500, "Template not found")
		return
	}
	c.String(200, string(template))
}

func (s *Server) handleIndex(c *gin.Context) {
	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	cacheData := s.datasetCache
	s.cacheMutex.RUnlock()

	// Get stored datasets from database for current workspace
	var storedDatasets []map[string]interface{}
	if s.datasetRepository != nil && s.workspaceRepository != nil {
		userID := core.ID("550e8400-e29b-41d4-a716-446655440000") // Default user

		// Get default workspace for user
		defaultWorkspace, err := s.workspaceRepository.GetDefaultForUser(c.Request.Context(), userID)
		var workspaceID core.ID
		if err != nil {
			log.Printf("[Dashboard] Failed to get default workspace for user %s: %v", userID, err)
			// Fall back to default workspace ID
			workspaceID = core.ID("550e8400-e29b-41d4-a716-446655440001")
		} else {
			workspaceID = defaultWorkspace.ID
		}

		// Get datasets for the workspace
		datasets, err := s.datasetRepository.GetByWorkspace(c.Request.Context(), workspaceID, 50, 0)
		if err == nil {
			storedDatasets = make([]map[string]interface{}, 0, len(datasets))
			for _, ds := range datasets {
				// Get fields for this dataset
				var fields []*FieldStats
				if ds.Metadata.Fields != nil && len(ds.Metadata.Fields) > 0 {
					// Use real field data from dataset metadata
					fields = make([]*FieldStats, 0, len(ds.Metadata.Fields))
					maxFields := 10 // Show max 10 fields in sidebar
					if len(ds.Metadata.Fields) < maxFields {
						maxFields = len(ds.Metadata.Fields)
					}

					for i := 0; i < maxFields; i++ {
						field := ds.Metadata.Fields[i]
						missingRate := 0.0
						if ds.RecordCount > 0 {
							missingRate = float64(field.MissingCount) / float64(ds.RecordCount)
						}

						// Map data_type to display type
						displayType := "text"
						switch field.DataType {
						case "numeric", "integer", "float", "double":
							displayType = "numeric"
						case "categorical", "string", "boolean":
							displayType = "categorical"
						}

						fieldStats := &FieldStats{
							Name:           field.Name,
							Type:           displayType,
							SampleSize:     int(ds.RecordCount),
							MissingRate:    missingRate,
							UniqueCount:    field.UniqueCount,
							MissingRatePct: fmt.Sprintf("%.1f%%", missingRate*100),
						}

						// Add statistics if available
						if field.Statistics != nil {
							if mean, ok := field.Statistics["mean"].(float64); ok {
								fieldStats.Mean = mean
							}
							if stddev, ok := field.Statistics["std"].(float64); ok {
								fieldStats.StdDev = stddev
							}
							if min, ok := field.Statistics["min"].(float64); ok {
								fieldStats.Min = min
							}
							if max, ok := field.Statistics["max"].(float64); ok {
								fieldStats.Max = max
							}
						}

						fields = append(fields, fieldStats)
					}
				} else if ds.FieldCount > 0 {
					// Fallback to placeholder fields if metadata not available
					fields = make([]*FieldStats, 0, ds.FieldCount)
					for i := 0; i < int(ds.FieldCount) && i < 10; i++ {
						fields = append(fields, &FieldStats{
							Name:        fmt.Sprintf("field_%d", i+1),
							Type:        "numeric",
							SampleSize:  int(ds.RecordCount),
							MissingRate: 0.0,
						})
					}
				}

				dataset := map[string]interface{}{
					"ID":          ds.ID.String(),
					"Name":        ds.DisplayName,
					"RecordCount": ds.RecordCount,
					"Fields":      fields,
					"Status":      string(ds.Status),
				}
				storedDatasets = append(storedDatasets, dataset)
			}
		}
	}

	if !cacheLoaded {
		// Get default workspace for template
		var currentWorkspaceID string
		if s.workspaceRepository != nil {
			userID := core.ID("550e8400-e29b-41d4-a716-446655440000")
			if workspace, err := s.workspaceRepository.GetDefaultForUser(c.Request.Context(), userID); err == nil {
				currentWorkspaceID = workspace.ID.String()
			} else {
				currentWorkspaceID = "550e8400-e29b-41d4-a716-446655440001" // Default workspace ID
			}
		}

		// Get any existing hypotheses for the workspace
		var hypotheses []*models.HypothesisResult
		if s.researchStorage != nil {
			workspaceHypotheses, err := s.researchStorage.ListByWorkspace(c.Request.Context(), currentWorkspaceID, 20)
			if err == nil {
				hypotheses = workspaceHypotheses
			}
		}

		// Load AI prompts for debugging
		promptManager := ai.NewPromptManager("prompts")
		greenfieldPrompt, _ := promptManager.LoadPrompt("greenfield_research")
		logicalAuditorPrompt, _ := promptManager.LoadPrompt("logical_auditor")
		falsificationPrompt, _ := promptManager.LoadPrompt("falsification_research")

		data := map[string]interface{}{
			"Title":      "GoHypo - Loading Dataset",
			"Loading":    true,
			"FieldStats": []*FieldStats{},
			"FieldCount": 0,
			"DatasetInfo": map[string]interface{}{
				"name":               "Loading dataset information...",
				"missingnessOverall": 0.0,
			},
			"Datasets":           storedDatasets,
			"CurrentWorkspaceID": currentWorkspaceID,
			"CurrentDataset": map[string]interface{}{
				"ID":          "loading",
				"Name":        "Loading...",
				"RecordCount": 0,
				"Fields":      []*FieldStats{},
			},
			"Hypotheses":           hypotheses,
			"GreenfieldPrompt":     greenfieldPrompt,
			"LogicalAuditorPrompt": logicalAuditorPrompt,
			"FalsificationPrompt":  falsificationPrompt,
		}
		s.renderTemplate(c, "main.html", data)
		return
	}

	// Add stored datasets to cache data
	if cacheData == nil {
		cacheData = make(map[string]interface{})
	}

	// Merge stored datasets with any existing datasets in cache
	existingDatasets, _ := cacheData["Datasets"].([]map[string]interface{})
	allDatasets := append(existingDatasets, storedDatasets...)
	cacheData["Datasets"] = allDatasets

	// Add current workspace ID to template data
	var currentWorkspaceID string
	if s.workspaceRepository != nil {
		userID := core.ID("550e8400-e29b-41d4-a716-446655440000")
		if workspace, err := s.workspaceRepository.GetDefaultForUser(c.Request.Context(), userID); err == nil {
			currentWorkspaceID = workspace.ID.String()
		} else {
			currentWorkspaceID = "550e8400-e29b-41d4-a716-446655440001" // Default workspace ID
		}
	}
	cacheData["CurrentWorkspaceID"] = currentWorkspaceID

	// Get existing hypotheses for the workspace
	var hypotheses []*models.HypothesisResult
	if s.researchStorage != nil {
		workspaceHypotheses, err := s.researchStorage.ListByWorkspace(c.Request.Context(), currentWorkspaceID, 20)
		if err == nil {
			hypotheses = workspaceHypotheses
		}
	}
	cacheData["Hypotheses"] = hypotheses

	// Load AI prompts for debugging
	promptManager := ai.NewPromptManager("prompts")
	greenfieldPrompt, _ := promptManager.LoadPrompt("greenfield_research")
	logicalAuditorPrompt, _ := promptManager.LoadPrompt("logical_auditor")
	falsificationPrompt, _ := promptManager.LoadPrompt("falsification_research")

	cacheData["GreenfieldPrompt"] = greenfieldPrompt
	cacheData["LogicalAuditorPrompt"] = logicalAuditorPrompt
	cacheData["FalsificationPrompt"] = falsificationPrompt
	cacheData["Title"] = "GoHypo - Research Dashboard"
	s.renderTemplate(c, "main.html", cacheData)
}

func (s *Server) handleFieldsList(c *gin.Context) {
	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	var fields []string
	if cacheLoaded && s.datasetCache != nil {
		if varsTotal, ok := s.datasetCache["VariablesTotal"].(int); ok {
			fields = make([]string, varsTotal)
			for i := range fields {
				fields[i] = fmt.Sprintf("field_%d", i+1)
			}
		}
	}
	s.cacheMutex.RUnlock()

	if !cacheLoaded {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset information not yet loaded"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fields": fields,
		"count":  len(fields),
	})
}

func (s *Server) handleDatasetStatus(c *gin.Context) {
	s.cacheMutex.RLock()
	loaded := s.cacheLoaded
	lastUpdated := s.cacheLastUpdated
	s.cacheMutex.RUnlock()

	status := "loading"
	if loaded {
		status = "loaded"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      status,
		"lastUpdated": lastUpdated.Format("2006-01-02 15:04:05"),
	})
}

func (s *Server) handleDatasetInfo(c *gin.Context) {
	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	cacheData := s.datasetCache
	s.cacheMutex.RUnlock()

	if !cacheLoaded {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset information not yet loaded"})
		return
	}

	c.JSON(http.StatusOK, cacheData)
}

// handleLoadMoreFields returns additional field information for progressive loading
func (s *Server) handleLoadMoreFields(c *gin.Context) {
	offsetStr := c.Query("offset")
	limitStr := c.Query("limit")

	offset := 0
	limit := 10

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil {
			offset = o
		}
	}

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 50 {
			limit = l
		}
	}

	excelData, err := s.getExcelData()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load field names")
		return
	}
	excelFields := excelData.Headers

	totalFields := len(excelFields)
	start := offset
	end := offset + limit
	if start >= totalFields {
		c.String(http.StatusOK, "")
		return
	}
	if end > totalFields {
		end = totalFields
	}

	relKind := core.ArtifactRelationship
	relArtifacts, err := s.reader.ListArtifacts(context.Background(), ports.ArtifactFilters{
		Kind:  &relKind,
		Limit: 1000,
	})
	if err != nil {
		relArtifacts = []core.Artifact{}
	}

	fieldStatsMap := make(map[string]*FieldStats)
	for i := start; i < end; i++ {
		fieldName := excelFields[i]
		fieldStatsMap[fieldName] = &FieldStats{
			Name:              fieldName,
			MissingRate:       0.0,
			MissingRatePct:    "—",
			UniqueCount:       0,
			Variance:          0.0,
			Cardinality:       0,
			Type:              "numeric",
			SampleSize:        0,
			InRelationships:   0,
			StrongestCorr:     0.0,
			AvgEffectSize:     0.0,
			SignificantRels:   0,
			TotalRelsAnalyzed: 0,
		}
	}

	for _, artifact := range relArtifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		var varX, varY string
		var missingRateX, missingRateY float64
		var uniqueCountX, uniqueCountY int
		var varianceX, varianceY float64
		var cardinalityX, cardinalityY int
		var sampleSize int

		if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			varX = string(relArtifact.Key.VariableX)
			varY = string(relArtifact.Key.VariableY)
			sampleSize = relArtifact.Metrics.SampleSize
			missingRateX = relArtifact.DataQuality.MissingRateX
			missingRateY = relArtifact.DataQuality.MissingRateY
			uniqueCountX = relArtifact.DataQuality.UniqueCountX
			uniqueCountY = relArtifact.DataQuality.UniqueCountY
			varianceX = relArtifact.DataQuality.VarianceX
			varianceY = relArtifact.DataQuality.VarianceY
			cardinalityX = relArtifact.DataQuality.CardinalityX
			cardinalityY = relArtifact.DataQuality.CardinalityY
		} else if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if vx, ok := payload["variable_x"].(string); ok {
				varX = vx
			}
			if vy, ok := payload["variable_y"].(string); ok {
				varY = vy
			}
			if ss, ok := payload["sample_size"].(float64); ok {
				sampleSize = int(ss)
			} else if ss, ok := payload["sample_size"].(int); ok {
				sampleSize = ss
			}

			if dq, ok := payload["data_quality"].(map[string]interface{}); ok {
				if mrx, ok := dq["missing_rate_x"].(float64); ok {
					missingRateX = mrx
				}
				if mry, ok := dq["missing_rate_y"].(float64); ok {
					missingRateY = mry
				}
				if ucx, ok := dq["unique_count_x"].(float64); ok {
					uniqueCountX = int(ucx)
				} else if ucx, ok := dq["unique_count_x"].(int); ok {
					uniqueCountX = ucx
				}
				if ucy, ok := dq["unique_count_y"].(float64); ok {
					uniqueCountY = int(ucy)
				} else if ucy, ok := dq["unique_count_y"].(int); ok {
					uniqueCountY = ucy
				}
				if vx, ok := dq["variance_x"].(float64); ok {
					varianceX = vx
				}
				if vy, ok := dq["variance_y"].(float64); ok {
					varianceY = vy
				}
				if cx, ok := dq["cardinality_x"].(float64); ok {
					cardinalityX = int(cx)
				} else if cx, ok := dq["cardinality_x"].(int); ok {
					cardinalityX = cx
				}
				if cy, ok := dq["cardinality_y"].(float64); ok {
					cardinalityY = int(cy)
				} else if cy, ok := dq["cardinality_y"].(int); ok {
					cardinalityY = cy
				}
			}
		}

		if statsX, exists := fieldStatsMap[varX]; exists {
			if missingRateX > 0 || sampleSize > 0 {
				statsX.MissingRate = missingRateX
				statsX.MissingRatePct = fmt.Sprintf("%.1f", missingRateX*100)
			}
			if uniqueCountX > 0 {
				statsX.UniqueCount = uniqueCountX
			}
			if varianceX > 0 {
				statsX.Variance = varianceX
				statsX.Type = "numeric"
			}
			if cardinalityX > 0 {
				statsX.Cardinality = cardinalityX
			}
			if sampleSize > 0 {
				statsX.SampleSize = sampleSize
			}
			statsX.InRelationships++
		}

		if statsY, exists := fieldStatsMap[varY]; exists {
			if missingRateY > 0 || sampleSize > 0 {
				statsY.MissingRate = missingRateY
				statsY.MissingRatePct = fmt.Sprintf("%.1f", missingRateY*100)
			}
			if uniqueCountY > 0 {
				statsY.UniqueCount = uniqueCountY
			}
			if varianceY > 0 {
				statsY.Variance = varianceY
				statsY.Type = "numeric"
			}
			if cardinalityY > 0 {
				statsY.Cardinality = cardinalityY
			}
			if sampleSize > 0 {
				statsY.SampleSize = sampleSize
			}
			statsY.InRelationships++
		}
	}

	// Collect field stats for template
	var fields []*FieldStats
	for i := start; i < end; i++ {
		fieldName := excelFields[i]
		if fieldStat, exists := fieldStatsMap[fieldName]; exists {
			fields = append(fields, fieldStat)
		}
	}

	data := map[string]interface{}{
		"Fields": fields,
	}

	s.renderTemplate(c, "field_row.html", data)
}

// DataSpaces handlers for hierarchical dataset exploration

func (s *Server) handleDatasetsList(c *gin.Context) {
	if s.datasetRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset repository not available"})
		return
	}

	// Get current user (default for single-user mode)
	userID := core.ID("550e8400-e29b-41d4-a716-446655440000")

	// Get datasets for the user
	storedDatasets, err := s.datasetRepository.GetByUserID(c.Request.Context(), userID, 100, 0) // Limit 100, offset 0
	if err != nil {
		log.Printf("[handleDatasetsList] Error retrieving datasets: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve datasets"})
		return
	}

	// Convert to response format
	datasets := make([]map[string]interface{}, 0, len(storedDatasets))
	for _, ds := range storedDatasets {
		dataset := map[string]interface{}{
			"id":           ds.ID.String(),
			"name":         ds.DisplayName,
			"description":  ds.Description,
			"status":       string(ds.Status),
			"record_count": ds.RecordCount,
			"field_count":  ds.FieldCount,
			"created_at":   ds.CreatedAt,
			"updated_at":   ds.UpdatedAt,
		}
		datasets = append(datasets, dataset)
	}

	// Also include the legacy "current" dataset if cache is loaded (for backward compatibility)
	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	s.cacheMutex.RUnlock()

	if cacheLoaded {
		datasets = append(datasets, map[string]interface{}{
			"id":          "current",
			"name":        "Legacy Dataset",
			"description": "Excel-based dataset (legacy)",
			"status":      "ready",
		})
	}

	c.JSON(http.StatusOK, gin.H{"datasets": datasets})
}

func (s *Server) handleDatasetFields(c *gin.Context) {
	datasetID := c.Param("id")

	// Handle "current" dataset (cached)
	if datasetID == "current" {
		s.cacheMutex.RLock()
		cacheLoaded := s.cacheLoaded
		datasetInfoInterface := s.datasetCache["DatasetInfo"]
		s.cacheMutex.RUnlock()

		if !cacheLoaded {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset not loaded"})
			return
		}

		// Type assert datasetInfo to map
		datasetInfo, ok := datasetInfoInterface.(map[string]interface{})
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid dataset info format"})
			return
		}

		// Return full dataset information for data modal compatibility
		response := map[string]interface{}{
			"id":          "current",
			"name":        datasetInfo["name"],
			"domain":      datasetInfo["domain"],
			"description": datasetInfo["description"],
			"recordCount": datasetInfo["record_count"],
			"fieldCount":  datasetInfo["field_count"],
			"missingRate": datasetInfo["missingness_overall"],
			"fileSize":    datasetInfo["fileSize"],
			"mimeType":    datasetInfo["mimeType"],
			"status":      "ready",
		}

		c.JSON(http.StatusOK, response)
		return
	}

	// Handle stored datasets
	if s.datasetRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset repository not available"})
		return
	}

	ctx := c.Request.Context()
	ds, err := s.datasetRepository.GetByID(ctx, core.ID(datasetID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dataset not found"})
		return
	}

	if !ds.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset is still processing"})
		return
	}

	// Convert field info to API format
	fields := make([]map[string]interface{}, len(ds.Metadata.Fields))
	for i, field := range ds.Metadata.Fields {
		fields[i] = map[string]interface{}{
			"name":         field.Name,
			"type":         string(field.DataType),
			"sampleSize":   len(field.SampleValues),
			"missingCount": field.MissingCount,
			"uniqueCount":  field.UniqueCount,
			"nullable":     field.Nullable,
		}
	}

	c.JSON(http.StatusOK, gin.H{"fields": fields})
}

// handleGetDataset returns information about a specific dataset
func (s *Server) handleGetDataset(c *gin.Context) {
	datasetID := c.Param("id")

	// Handle "current" dataset (cached)
	if datasetID == "current" {
		s.cacheMutex.RLock()
		cacheLoaded := s.cacheLoaded
		datasetInfo := s.datasetCache["DatasetInfo"]
		s.cacheMutex.RUnlock()

		if !cacheLoaded {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset not loaded"})
			return
		}

		c.JSON(http.StatusOK, datasetInfo)
		return
	}

	// Handle stored datasets
	if s.datasetRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset repository not available"})
		return
	}

	ctx := c.Request.Context()
	ds, err := s.datasetRepository.GetByID(ctx, core.ID(datasetID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dataset not found"})
		return
	}

	// Format dataset information
	datasetInfo := map[string]interface{}{
		"id":          ds.ID.String(),
		"name":        ds.DisplayName,
		"domain":      ds.Domain,
		"description": ds.Description,
		"recordCount": ds.RecordCount,
		"fieldCount":  ds.FieldCount,
		"missingRate": ds.MissingRate,
		"fileSize":    ds.FileSize,
		"mimeType":    ds.MimeType,
		"status":      string(ds.Status),
		"createdAt":   ds.CreatedAt,
		"updatedAt":   ds.UpdatedAt,
	}

	c.JSON(http.StatusOK, datasetInfo)
}

func (s *Server) handleDatasetPreview(c *gin.Context) {
	datasetID := c.Param("id")

	// Handle different dataset sources
	if datasetID == "current" {
		// Use the existing Excel-based dataset
		s.handleCurrentDatasetPreview(c)
		return
	}

	// Handle stored datasets
	s.handleStoredDatasetPreview(c, datasetID)
}

func (s *Server) handleCurrentDatasetPreview(c *gin.Context) {

	// Parse pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50 // Default to 50 rows per page
	}

	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	datasetInfo := s.datasetCache["DatasetInfo"]
	fieldStats := s.datasetCache["FieldStats"]
	sampleRows := s.datasetCache["SampleRows"]
	s.cacheMutex.RUnlock()

	if !cacheLoaded {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset not loaded"})
		return
	}

	// Prepare field information
	fields := make([]map[string]interface{}, 0)
	if fieldStatsSlice, ok := fieldStats.([]map[string]interface{}); ok {
		for _, field := range fieldStatsSlice {
			fields = append(fields, map[string]interface{}{
				"name": field["name"],
				"type": field["type"],
			})
		}
	}

	// Extract dataset metadata
	recordCount := 0
	missingRate := 0.0
	var lastUpdated interface{}

	if datasetInfoMap, ok := datasetInfo.(map[string]interface{}); ok {
		if rc, ok := datasetInfoMap["record_count"].(int); ok {
			recordCount = rc
		}
		if mr, ok := datasetInfoMap["missingness_overall"].(float64); ok {
			missingRate = mr
		}
		lastUpdated = datasetInfoMap["last_updated"]
	}

	// Prepare paginated data
	rows := make([]map[string]string, 0)
	totalRows := 0
	var sampleRowsSlice []map[string]string

	if sampleRowsData, ok := sampleRows.([]map[string]string); ok {
		sampleRowsSlice = sampleRowsData
		totalRows = len(sampleRowsSlice)

		// Calculate pagination
		startIndex := (page - 1) * limit
		endIndex := startIndex + limit

		if startIndex < totalRows {
			if endIndex > totalRows {
				endIndex = totalRows
			}
			rows = sampleRowsSlice[startIndex:endIndex]
		}
	}

	// Calculate pagination metadata
	totalPages := (totalRows + limit - 1) / limit // Ceiling division
	if totalPages == 0 {
		totalPages = 1
	}

	response := gin.H{
		"id":          "current",
		"name":        "Current Dataset",
		"recordCount": recordCount,
		"fieldCount":  len(fields),
		"missingRate": missingRate,
		"size":        "Unknown",
		"lastUpdated": lastUpdated,
		"fields":      fields,
		"rows":        rows,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"totalRows":  totalRows,
			"totalPages": totalPages,
			"hasNext":    page < totalPages,
			"hasPrev":    page > 1,
		},
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleStoredDatasetPreview(c *gin.Context, datasetID string) {
	if s.datasetRepository == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset repository not available"})
		return
	}

	// Get dataset from repository
	ctx := c.Request.Context()
	ds, err := s.datasetRepository.GetByID(ctx, core.ID(datasetID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dataset not found"})
		return
	}

	if !ds.IsReady() {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset is still processing"})
		return
	}

	// Parse pagination parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "50")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 50
	}

	// Get paginated sample rows from metadata
	sampleRows := ds.Metadata.SampleRows
	totalRows := len(sampleRows)

	// Calculate pagination
	startIndex := (page - 1) * limit
	endIndex := startIndex + limit

	var paginatedRows []map[string]interface{}
	if startIndex < totalRows {
		if endIndex > totalRows {
			endIndex = totalRows
		}
		paginatedRows = sampleRows[startIndex:endIndex]
	}

	// Calculate pagination metadata
	totalPages := (totalRows + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	// Convert fields to API format
	fields := make([]map[string]interface{}, len(ds.Metadata.Fields))
	for i, field := range ds.Metadata.Fields {
		fields[i] = map[string]interface{}{
			"name": field.Name,
			"type": field.DataType,
		}
	}

	response := gin.H{
		"id":          ds.ID,
		"name":        ds.GetDisplayName(),
		"recordCount": ds.RecordCount,
		"fieldCount":  ds.FieldCount,
		"missingRate": ds.MissingRate,
		"size":        "Unknown", // Could calculate from file size
		"lastUpdated": ds.UpdatedAt,
		"fields":      fields,
		"rows":        paginatedRows,
		"pagination": gin.H{
			"page":       page,
			"limit":      limit,
			"totalRows":  totalRows,
			"totalPages": totalPages,
			"hasNext":    page < totalPages,
			"hasPrev":    page > 1,
		},
	}

	c.JSON(http.StatusOK, response)
}

func (s *Server) handleFieldDetails(c *gin.Context) {
	fieldName := c.Param("name")

	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	fieldStatsInterface := s.datasetCache["FieldStats"]
	s.cacheMutex.RUnlock()

	if !cacheLoaded {
		c.String(http.StatusServiceUnavailable, "Dataset not loaded")
		return
	}

	// Find the specific field
	if fieldStats, ok := fieldStatsInterface.([]*FieldStats); ok {
		for _, field := range fieldStats {
			if field.Name == fieldName {
				c.Header("Content-Type", "text/html")
				s.renderTemplate(c, "field_detail.html", field)
				return
			}
		}
	}

	c.String(http.StatusNotFound, "Field not found")
}
