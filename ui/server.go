package ui

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"strings"

	"gohypo/adapters/excel"
	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/internal/testkit"
	"gohypo/ports"

	"github.com/gin-gonic/gin"
)

// Server represents the web server for GoHypo UI
type Server struct {
	router            *gin.Engine
	testkit           *testkit.TestKit
	reader            ports.LedgerReaderPort
	templates         *template.Template
	embeddedFiles     embed.FS
	greenfieldService interface{} // Greenfield research service (when configured)
}

// NewServer creates a new web server instance
func NewServer(embeddedFiles embed.FS) *Server {
	return &Server{
		router:        gin.Default(),
		embeddedFiles: embeddedFiles,
	}
}

// Initialize sets up the server with dependencies
func (s *Server) Initialize(kit *testkit.TestKit, reader ports.LedgerReaderPort, embeddedFiles embed.FS, greenfieldService interface{}) error {
	s.testkit = kit
	s.reader = reader
	s.greenfieldService = greenfieldService

	// Parse templates
	funcMap := template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
		// Back-compat: some templates use `multiply` rather than `mul`.
		"multiply": func(a, b float64) float64 { return a * b },
		// Format sample sizes as e.g. "12k" for 12000.
		// Accepts int/int64/float64 to tolerate JSON + Go struct inputs.
		"kfmt": func(v interface{}) string {
			switch t := v.(type) {
			case int:
				return fmt.Sprintf("%dk", t/1000)
			case int64:
				return fmt.Sprintf("%dk", t/1000)
			case float64:
				return fmt.Sprintf("%dk", int64(t/1000))
			case float32:
				return fmt.Sprintf("%dk", int64(float64(t)/1000))
			default:
				return "—"
			}
		},
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"add": func(a, b int) int { return a + b },
		"minInt": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b float64) float64 {
			if a > b {
				return a
			}
			return b
		},
		"upper": strings.ToUpper,
		"until": func(n int) []int {
			res := make([]int, n)
			for i := range res {
				res[i] = i
			}
			return res
		},
	}

	// Add string manipulation functions
	funcMap["substr"] = func(s string, start, end int) string {
		if start < 0 {
			start = 0
		}
		if end > len(s) {
			end = len(s)
		}
		if start >= end {
			return ""
		}
		return s[start:end]
	}

	// Add time formatting functions
	funcMap["formatTime"] = func(t interface{}) string {
		// Handle different time types that might come from templates
		switch v := t.(type) {
		case string:
			return v // If it's already a string, just return it
		default:
			return "—"
		}
	}

	funcMap["formatDuration"] = func(d interface{}) string {
		// Handle duration formatting
		switch v := d.(type) {
		case string:
			return v
		case float64:
			if v < 1000 {
				return fmt.Sprintf("%.0fms", v)
			} else {
				return fmt.Sprintf("%.2fs", v/1000)
			}
		case int:
			if v < 1000 {
				return fmt.Sprintf("%dms", v)
			} else {
				return fmt.Sprintf("%.2fs", float64(v)/1000)
			}
		default:
			return "—"
		}
	}

	// Create a sub-filesystem rooted at ui/templates for simpler template names
	templatesFS, err := fs.Sub(embeddedFiles, "ui/templates")
	if err != nil {
		return fmt.Errorf("failed to create templates filesystem: %w", err)
	}

	var parseErr error
	s.templates = template.New("").Funcs(funcMap)

	// Parse all template files individually to ensure proper naming
	files1, err := fs.Glob(templatesFS, "*.html")
	if err != nil {
		return fmt.Errorf("failed to glob root templates: %w", err)
	}

	files2, err := fs.Glob(templatesFS, "**/*.html")
	if err != nil {
		return fmt.Errorf("failed to glob nested templates: %w", err)
	}

	files := append(files1, files2...)
	log.Printf("[DEBUG] Found %d template files: %v", len(files), files)

	for _, file := range files {
		content, err := fs.ReadFile(templatesFS, file)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", file, err)
		}

		// For the main template, use just the base name
		name := file
		if file == "index.html" {
			name = "index.html"
		}

		log.Printf("[DEBUG] Parsing template file=%s as name=%s", file, name)
		_, parseErr = s.templates.New(name).Parse(string(content))
		if parseErr != nil {
			return fmt.Errorf("failed to parse template %s: %w", file, parseErr)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	s.setupMiddleware()
	s.setupRoutes()

	return nil
}

// setupMiddleware configures Gin middleware
func (s *Server) setupMiddleware() {
	// Serve static files
	s.router.StaticFS("/static", http.FS(s.embeddedFiles))
}

// setupRoutes configures the application routes
func (s *Server) setupRoutes() {
	// Blueprint Dashboard - Single Page Application
	s.router.GET("/", s.handleIndex)

	// API endpoints for blueprint dashboard data
	s.router.GET("/api/fields/list", s.handleFieldsList)
}

// Start starts the web server
func (s *Server) Start(addr string) error {
	log.Printf("Starting GoHypo UI on http://%s", addr)
	return s.router.Run(addr)
}

// Template helpers
func (s *Server) renderTemplate(c *gin.Context, templateName string, data interface{}) {
	c.Header("Content-Type", "text/html")
	if err := s.templates.ExecuteTemplate(c.Writer, templateName, data); err != nil {
		log.Printf("Template error: %v", err)
		c.AbortWithStatus(http.StatusInternalServerError)
	}
}

// HTMX helpers are defined in app.go

func (s *Server) renderPartial(c *gin.Context, templateName string, data interface{}) {
	s.renderTemplate(c, templateName, data)
}

// handleIndex renders the main index page with halftone matrix visualization
func (s *Server) handleIndex(c *gin.Context) {
	log.Printf("[handleIndex] Starting index page render")

	// Get all artifacts to extract dataset and field info
	allArtifacts, err := s.reader.ListArtifacts(c.Request.Context(), ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		log.Printf("[handleIndex] ERROR: Failed to load artifacts: %v", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to load artifacts"})
		return
	}
	log.Printf("[handleIndex] Loaded %d total artifacts", len(allArtifacts))

	// Extract dataset information from run artifacts and relationship artifacts
	datasetInfo := map[string]interface{}{
		"name":        "Unknown Dataset",
		"snapshotID":  "",
		"snapshotAt":  "",
		"cutoffAt":    "",
		"cohortSize":  0,
		"runCount":    0,
		"createdAt":   "",
		"source":      "testkit",
		"datasetHash": "",
	}

	// Extract unique fields from relationship artifacts
	fieldSet := make(map[string]bool)

	// First, add ALL fields from Excel file to ensure complete coverage
	if excelFields, err := s.getExcelFieldNames(); err == nil {
		for _, fieldName := range excelFields {
			fieldSet[fieldName] = true
		}
		log.Printf("[handleIndex] Added %d fields from Excel file", len(excelFields))
	}

	relationshipCount := 0
	runIDs := make(map[string]bool)
	earliestTimestamp := core.Timestamp{}
	latestTimestamp := core.Timestamp{}

	for _, artifact := range allArtifacts {
		// Track timestamps
		if earliestTimestamp.IsZero() || artifact.CreatedAt.Before(earliestTimestamp) {
			earliestTimestamp = artifact.CreatedAt
		}
		if latestTimestamp.IsZero() || artifact.CreatedAt.After(latestTimestamp) {
			latestTimestamp = artifact.CreatedAt
		}

		// Extract run ID and dataset info
		if artifact.Kind == core.ArtifactRun {
			runIDs[string(artifact.ID)] = true
			log.Printf("[handleIndex] Found run artifact: %s", artifact.ID)

			// Try to extract dataset info from run manifest
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if snapshotID, ok := payload["snapshot_id"].(string); ok && datasetInfo["snapshotID"] == "" {
					datasetInfo["snapshotID"] = snapshotID
					log.Printf("[handleIndex] Extracted snapshot_id: %s", snapshotID)

					// Derive dataset name from snapshot_id (e.g., "test-snapshot-shopping" -> "Shopping Dataset")
					if datasetInfo["name"] == "Unknown Dataset" {
						if len(snapshotID) > 0 {
							// Try to extract meaningful name from snapshot_id
							if len(snapshotID) > 14 && snapshotID[:14] == "test-snapshot-" {
								namePart := snapshotID[14:]
								if namePart != "" {
									datasetInfo["name"] = fmt.Sprintf("%s Dataset", namePart)
									log.Printf("[handleIndex] Derived dataset name from snapshot_id: %s", datasetInfo["name"])
								}
							} else {
								datasetInfo["name"] = fmt.Sprintf("Dataset %s", snapshotID[:8])
								log.Printf("[handleIndex] Using snapshot_id prefix as dataset name: %s", datasetInfo["name"])
							}
						}
					}
				}
				if snapshotAt, ok := payload["snapshot_at"].(string); ok && datasetInfo["snapshotAt"] == "" {
					datasetInfo["snapshotAt"] = snapshotAt
				}
				if cutoffAt, ok := payload["cutoff_at"].(string); ok && datasetInfo["cutoffAt"] == "" {
					datasetInfo["cutoffAt"] = cutoffAt
				}
				// Try to extract dataset name directly
				if name, ok := payload["dataset_name"].(string); ok && name != "" {
					datasetInfo["name"] = name
					log.Printf("[handleIndex] Found dataset_name in payload: %s", name)
				}
			} else {
				log.Printf("[handleIndex] WARNING: Run artifact payload is not a map[string]interface{}, type: %T", artifact.Payload)
			}
		}

		if artifact.Kind == core.ArtifactRelationship {
			relationshipCount++
			var varX, varY string
			if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				varX = string(relArtifact.Key.VariableX)
				varY = string(relArtifact.Key.VariableY)
				log.Printf("[handleIndex] Found RelationshipArtifact: %s <-> %s", varX, varY)
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				varX = string(relPayload.VariableX)
				varY = string(relPayload.VariableY)
				log.Printf("[handleIndex] Found RelationshipPayload: %s <-> %s", varX, varY)
			} else if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					varX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					varY = vy
				}
				if varX != "" && varY != "" {
					log.Printf("[handleIndex] Found relationship in map payload: %s <-> %s", varX, varY)
				}
			} else {
				log.Printf("[handleIndex] WARNING: Relationship artifact payload type not recognized: %T", artifact.Payload)
			}
			if varX != "" {
				fieldSet[varX] = true
			}
			if varY != "" {
				fieldSet[varY] = true
			}
		}
	}

	// Extract run count and creation time from artifacts
	datasetInfo["runCount"] = len(runIDs)
	if !earliestTimestamp.IsZero() {
		datasetInfo["createdAt"] = earliestTimestamp.Time().Format("2006-01-02 15:04:05")
	}

	// Calculate time span
	if !earliestTimestamp.IsZero() && !latestTimestamp.IsZero() {
		diff := latestTimestamp.Time().Sub(earliestTimestamp.Time())
		days := int(diff.Hours() / 24)
		if days > 0 {
			datasetInfo["timeSpan"] = fmt.Sprintf("%d days", days)
		} else {
			hours := int(diff.Hours())
			if hours > 0 {
				datasetInfo["timeSpan"] = fmt.Sprintf("%d hours", hours)
			}
		}
	}

	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}

	// Determine run status based on artifacts
	runStatus := "NOT_RUN"
	if relationshipCount > 0 {
		runStatus = "COMPLETE"
	} else if len(allArtifacts) > 0 {
		for _, a := range allArtifacts {
			if a.Kind == core.ArtifactRun {
				runStatus = "RUNNING"
				break
			}
		}
	}

	// Count artifacts by kind to determine stage completion
	profileArtifactCount := 0
	pairwiseArtifactCount := relationshipCount
	fdrArtifactCount := 0
	permutationArtifactCount := 0
	stabilityArtifactCount := 0
	batteryArtifactCount := 0

	relKind := core.ArtifactRelationship
	relFilters := ports.ArtifactFilters{
		Kind:  &relKind,
		Limit: 1000,
	}
	relArtifacts, _ := s.reader.ListArtifacts(c.Request.Context(), relFilters)

	for _, artifact := range relArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if qv, ok := payload["q_value"].(float64); ok && qv > 0 {
					fdrArtifactCount++
				} else if qv, ok := payload["fdr_q_value"].(float64); ok && qv > 0 {
					fdrArtifactCount++
				}
			} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				if relArtifact.Metrics.QValue > 0 {
					fdrArtifactCount++
				}
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				if relPayload.QValue > 0 {
					fdrArtifactCount++
				}
			}
		}
	}

	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRun {
			profileArtifactCount++
		}
	}

	// Extract seed/fingerprint from run artifacts
	seed := ""
	fingerprint := ""
	registryHash := ""
	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRun {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if s, ok := payload["seed"].(float64); ok {
					seed = fmt.Sprintf("%.0f", s)
				}
				if fp, ok := payload["fingerprint"].(string); ok {
					fingerprint = fp
				}
				if rh, ok := payload["registry_hash"].(string); ok {
					registryHash = rh
				}
			}
		}
	}

	// Determine significance rule
	significanceRule := "p ≤ 0.05"
	hasQValue := fdrArtifactCount > 0
	if hasQValue {
		significanceRule = "q ≤ 0.05 (BH)"
	}

	// Calculate pairs attempted
	pairsAttempted := len(fields) * (len(fields) - 1) / 2
	if pairsAttempted < 0 {
		pairsAttempted = 0
	}
	pairsTested := relationshipCount
	pairsSkipped := pairsAttempted - pairsTested
	if pairsSkipped < 0 {
		pairsSkipped = 0
	}

	// Count significant relationships
	significantCount := 0
	for _, artifact := range relArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			var pValue float64
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if pv, ok := payload["p_value"].(float64); ok {
					pValue = pv
				}
			} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				pValue = relArtifact.Metrics.PValue
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				pValue = relPayload.PValue
			}
			if pValue > 0 && pValue < 0.05 {
				significantCount++
			}
		}
	}

	// Stage statuses
	stageStatuses := map[string]map[string]interface{}{
		"Profile": {
			"Status":        "NOT_RUN",
			"ArtifactCount": profileArtifactCount,
			"Runtime":       0,
		},
		"Pairwise": {
			"Status":        "NOT_RUN",
			"ArtifactCount": pairwiseArtifactCount,
			"Runtime":       0,
		},
		"FDR": {
			"Status":        "NOT_RUN",
			"ArtifactCount": fdrArtifactCount,
			"Runtime":       0,
		},
		"Permutation": {
			"Status":        "NOT_RUN",
			"ArtifactCount": permutationArtifactCount,
			"Runtime":       0,
		},
		"Stability": {
			"Status":        "NOT_RUN",
			"ArtifactCount": stabilityArtifactCount,
			"Runtime":       0,
		},
		"Battery": {
			"Status":        "NOT_RUN",
			"ArtifactCount": batteryArtifactCount,
			"Runtime":       0,
		},
	}

	if profileArtifactCount > 0 || len(fields) > 0 {
		stageStatuses["Profile"]["Status"] = "COMPLETE"
	}
	if pairwiseArtifactCount > 0 {
		stageStatuses["Pairwise"]["Status"] = "COMPLETE"
	}
	if fdrArtifactCount > 0 {
		stageStatuses["FDR"]["Status"] = "COMPLETE"
	}

	// Extract field-level statistics from relationship artifacts
	type FieldStats struct {
		Name              string
		MissingRate       float64
		MissingRatePct    string
		UniqueCount       int
		Variance          float64
		Cardinality       int
		Type              string
		SampleSize        int
		InRelationships   int
		StrongestCorr     float64
		AvgEffectSize     float64
		SignificantRels   int
		TotalRelsAnalyzed int
	}
	fieldStatsMap := make(map[string]*FieldStats)

	// Initialize field stats
	for _, field := range fields {
		fieldStatsMap[field] = &FieldStats{
			Name:              field,
			MissingRate:       0,
			MissingRatePct:    "—",
			UniqueCount:       0,
			Variance:          0,
			Cardinality:       0,
			Type:              "unknown",
			SampleSize:        0,
			InRelationships:   0,
			StrongestCorr:     0,
			AvgEffectSize:     0,
			SignificantRels:   0,
			TotalRelsAnalyzed: 0,
		}
	}

	// Extract stats from relationship artifacts
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

		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
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
			} else if ss, ok := payload["sample_size"].(int64); ok {
				sampleSize = int(ss)
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
					uniqueCountY = cy
				}
			}
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
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
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			varX = string(relPayload.VariableX)
			varY = string(relPayload.VariableY)
			sampleSize = relPayload.SampleSize
		}

		// Update field stats
		if statsX, exists := fieldStatsMap[varX]; exists {
			if missingRateX > 0 {
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
			if missingRateY > 0 {
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

	// Compute relationship statistics for each field
	effectSizesPerField := make(map[string][]float64)
	significantRelsPerField := make(map[string]int)

	for _, artifact := range relArtifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		var varX, varY string
		var effectSize float64
		var pValue float64

		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if vx, ok := payload["variable_x"].(string); ok {
				varX = vx
			}
			if vy, ok := payload["variable_y"].(string); ok {
				varY = vy
			}
			if es, ok := payload["effect_size"].(float64); ok {
				effectSize = es
			}
			if pv, ok := payload["p_value"].(float64); ok {
				pValue = pv
			}
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			varX = string(relArtifact.Key.VariableX)
			varY = string(relArtifact.Key.VariableY)
			effectSize = relArtifact.Metrics.EffectSize
			pValue = relArtifact.Metrics.PValue
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			varX = string(relPayload.VariableX)
			varY = string(relPayload.VariableY)
			effectSize = relPayload.EffectSize
			pValue = relPayload.PValue
		}

		// Track effect sizes and significance for each field
		if varX != "" {
			effectSizesPerField[varX] = append(effectSizesPerField[varX], effectSize)
			if pValue < 0.05 { // Significant at 5% level
				significantRelsPerField[varX]++
			}
		}
		if varY != "" {
			effectSizesPerField[varY] = append(effectSizesPerField[varY], effectSize)
			if pValue < 0.05 { // Significant at 5% level
				significantRelsPerField[varY]++
			}
		}
	}

	// Compute aggregate statistics for each field
	for field, stats := range fieldStatsMap {
		effectSizes := effectSizesPerField[field]
		if len(effectSizes) > 0 {
			// Find strongest correlation (absolute value)
			strongest := 0.0
			sum := 0.0
			for _, es := range effectSizes {
				absES := math.Abs(es)
				if absES > math.Abs(strongest) {
					strongest = es // Keep sign for direction
				}
				sum += absES // Use absolute for average
			}
			stats.StrongestCorr = strongest
			stats.AvgEffectSize = sum / float64(len(effectSizes))
			stats.TotalRelsAnalyzed = len(effectSizes)
		}

		if sigCount, exists := significantRelsPerField[field]; exists {
			stats.SignificantRels = sigCount
		}
	}

	// Convert to slice
	fieldStats := make([]FieldStats, 0, len(fieldStatsMap))
	for _, stats := range fieldStatsMap {
		fieldStats = append(fieldStats, *stats)
	}

	// Build FieldRelationships array for template
	type FieldRelationship struct {
		FieldX       string
		FieldY       string
		EffectSize   float64
		PValue       float64
		QValue       float64
		TestType     string
		SampleSize   int
		MissingRateX float64
		MissingRateY float64
		Significant  bool
		StrengthDesc string
		IsShadow     bool
	}

	var fieldRelationships []FieldRelationship
	log.Printf("[handleIndex] Building FieldRelationships array from %d relationship artifacts", len(relArtifacts))

	for _, artifact := range relArtifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		var relX, relY string
		var relEffectSize, relPValue, relQValue float64
		var relTestType string
		var relSampleSize int
		var missingRateX, missingRateY float64

		if payload, ok := artifact.Payload.(map[string]interface{}); ok {
			if vx, ok := payload["variable_x"].(string); ok {
				relX = vx
			}
			if vy, ok := payload["variable_y"].(string); ok {
				relY = vy
			}
			if es, ok := payload["effect_size"].(float64); ok {
				relEffectSize = es
			}
			if pv, ok := payload["p_value"].(float64); ok {
				relPValue = pv
			}
			if qv, ok := payload["fdr_q_value"].(float64); ok {
				relQValue = qv
			} else if qv, ok := payload["q_value"].(float64); ok {
				relQValue = qv
			}
			if tt, ok := payload["test_type"].(string); ok {
				relTestType = tt
			}
			if ss, ok := payload["sample_size"].(float64); ok {
				relSampleSize = int(ss)
			}

			// Extract data quality information
			if dq, ok := payload["data_quality"].(map[string]interface{}); ok {
				if mrx, ok := dq["missing_rate_x"].(float64); ok {
					missingRateX = mrx
				}
				if mry, ok := dq["missing_rate_y"].(float64); ok {
					missingRateY = mry
				}
			}
		} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			relX = string(relArtifact.Key.VariableX)
			relY = string(relArtifact.Key.VariableY)
			relEffectSize = relArtifact.Metrics.EffectSize
			relPValue = relArtifact.Metrics.PValue
			relQValue = relArtifact.Metrics.QValue
			relTestType = string(relArtifact.Key.TestType)
			relSampleSize = relArtifact.Metrics.SampleSize
			missingRateX = relArtifact.DataQuality.MissingRateX
			missingRateY = relArtifact.DataQuality.MissingRateY
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			relX = string(relPayload.VariableX)
			relY = string(relPayload.VariableY)
			relEffectSize = relPayload.EffectSize
			relPValue = relPayload.PValue
			relQValue = relPayload.QValue
			relTestType = string(relPayload.TestType)
			relSampleSize = relPayload.SampleSize
			// RelationshipPayload doesn't have DataQuality fields, so leave missingRateX/Y as 0
		}

		if relX != "" && relY != "" {
			// Determine if significant
			significant := false
			if relQValue > 0 {
				significant = relQValue < 0.05
			} else {
				significant = relPValue > 0 && relPValue < 0.05
			}

			// Determine strength description
			strengthDesc := "weak"
			absEffect := relEffectSize
			if absEffect < 0 {
				absEffect = -absEffect
			}
			if absEffect > 0.5 {
				strengthDesc = "strong"
			} else if absEffect > 0.3 {
				strengthDesc = "moderate"
			}

			fieldRelationships = append(fieldRelationships, FieldRelationship{
				FieldX:       relX,
				FieldY:       relY,
				EffectSize:   relEffectSize,
				PValue:       relPValue,
				QValue:       relQValue,
				TestType:     relTestType,
				SampleSize:   relSampleSize,
				MissingRateX: missingRateX,
				MissingRateY: missingRateY,
				Significant:  significant,
				StrengthDesc: strengthDesc,
				IsShadow:     false,
			})
		}
	}

	log.Printf("[handleIndex] Built %d FieldRelationships for display", len(fieldRelationships))

	// Ensure missingnessOverall is always a valid float64
	if datasetInfo == nil {
		datasetInfo = make(map[string]interface{})
	}
	if _, ok := datasetInfo["missingnessOverall"].(float64); !ok {
		datasetInfo["missingnessOverall"] = 0.0
	}

	log.Printf("[handleIndex] Dataset info - name: %v, relationships: %d, fields: %d", datasetInfo["name"], relationshipCount, len(fields))

	// ResearchControl data for control strip
	researchControl := map[string]interface{}{
		"State":           "idle",
		"Progress":        0.0,
		"VariableCount":   len(fields),
		"HypothesisCount": 0,
		"SessionID":       "",
		"Model":           "",
		"ErrorMessage":    "",
	}

	data := map[string]interface{}{
		"Title":              "GoHypo",
		"FieldCount":         len(fields),
		"RelationshipCount":  relationshipCount,
		"Fields":             fields,
		"DatasetInfo":        datasetInfo,
		"RunStatus":          runStatus,
		"PairsAttempted":     pairsAttempted,
		"PairsTested":        pairsTested,
		"PairsSkipped":       pairsSkipped,
		"PairsPassed":        significantCount,
		"SignificanceRule":   significanceRule,
		"Seed":               seed,
		"Fingerprint":        fingerprint,
		"RegistryHash":       registryHash,
		"StageStatuses":      stageStatuses,
		"VariablesTotal":     len(fields),
		"VariablesEligible":  len(fields),
		"VariablesRejected":  0,
		"FieldStats":         fieldStats,
		"FieldRelationships": fieldRelationships,
		"ResearchControl":    researchControl,
	}
	log.Printf("[handleIndex] Rendering template with %d relationships", len(fieldRelationships))
	s.renderTemplate(c, "index.html", data)
}

// handleFieldsList returns field information for the UI
func (s *Server) handleFieldsList(c *gin.Context) {
	// Get all artifacts to extract field info
	allArtifacts, err := s.reader.ListArtifacts(c.Request.Context(), ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load artifacts"})
		return
	}

	// Extract unique fields from relationship artifacts
	fieldSet := make(map[string]bool)
	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			var varX, varY string
			if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				varX = string(relArtifact.Key.VariableX)
				varY = string(relArtifact.Key.VariableY)
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				varX = string(relPayload.VariableX)
				varY = string(relPayload.VariableY)
			} else if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					varX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					varY = vy
				}
			}
			if varX != "" {
				fieldSet[varX] = true
			}
			if varY != "" {
				fieldSet[varY] = true
			}
		}
	}

	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}

	c.JSON(http.StatusOK, gin.H{
		"fields": fields,
		"count":  len(fields),
	})
}

// getExcelFieldNames reads all field names directly from the Excel file
func (a *Server) getExcelFieldNames() ([]string, error) {
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		return nil, fmt.Errorf("EXCEL_FILE environment variable not set")
	}

	// Read Excel data to get column information
	reader := excel.NewExcelReader(excelFile)
	data, err := reader.ReadData()
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel file: %w", err)
	}

	return data.Headers, nil
}
