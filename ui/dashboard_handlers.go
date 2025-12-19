package ui

import (
	"context"
	"fmt"
	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleMissionControl(c *gin.Context) {
	c.Header("Content-Type", "text/html")
	template, err := s.embeddedFiles.ReadFile("ui/templates/mission_control.html")
	if err != nil {
		log.Printf("[MissionControl] Template not found: %v", err)
		c.String(500, "Template not found")
		return
	}
	c.String(200, string(template))
}

func (s *Server) handleIndex(c *gin.Context) {
	log.Printf("[handleIndex] Starting index page render")

	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	cacheData := s.datasetCache
	s.cacheMutex.RUnlock()

	if !cacheLoaded {
		log.Printf("[handleIndex] Dataset info not loaded yet, showing loading state")
		data := map[string]interface{}{
			"Title":      "GoHypo - Loading Dataset",
			"Loading":    true,
			"FieldStats": []*FieldStats{},
			"FieldCount": 0,
			"DatasetInfo": map[string]interface{}{
				"name":               "Loading dataset information...",
				"missingnessOverall": 0.0,
			},
		}
		s.renderTemplate(c, "index.html", data)
		return
	}

	log.Printf("[handleIndex] Using cached dataset info, rendering template")
	cacheData["Title"] = "GoHypo - Research Dashboard"
	s.renderTemplate(c, "index.html", cacheData)
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
	// For now, just return the current dataset
	// In a full implementation, this would list all available datasets
	datasets := []map[string]interface{}{
		{
			"id":   "current",
			"name": "Current Dataset",
		},
	}

	c.JSON(http.StatusOK, gin.H{"datasets": datasets})
}

func (s *Server) handleDatasetFields(c *gin.Context) {
	datasetID := c.Param("id")

	// For now, only support "current" dataset
	if datasetID != "current" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dataset not found"})
		return
	}

	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	fieldStats := s.datasetCache["FieldStats"]
	s.cacheMutex.RUnlock()

	if !cacheLoaded {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset not loaded"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"fields": fieldStats})
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
				s.renderTemplate(c, "field_details.html", field)
				return
			}
		}
	}

	c.String(http.StatusNotFound, "Field not found")
}
