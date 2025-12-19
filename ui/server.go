package ui

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gohypo/adapters/datareadiness"
	"gohypo/adapters/datareadiness/coercer"
	"gohypo/adapters/excel"
	"gohypo/domain/core"
	"gohypo/domain/datareadiness/ingestion"
	"gohypo/domain/datareadiness/profiling"
	"gohypo/domain/run"
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

	// Dataset info caching for async loading
	datasetCache     map[string]interface{}
	cacheMutex       sync.RWMutex
	cacheLoaded      bool
	cacheLastUpdated time.Time

	// Excel data caching
	excelDataCache      *excel.ExcelData
	excelColumnTypes    map[string]string
	excelCacheMutex     sync.RWMutex
	excelCacheLoaded    bool
	excelCacheTimestamp time.Time
}

// FieldStats represents statistics for a single field/variable
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

// NewServer creates a new web server instance
func NewServer(embeddedFiles embed.FS) *Server {
	return &Server{
		router:           gin.Default(),
		embeddedFiles:    embeddedFiles,
		datasetCache:     make(map[string]interface{}),
		cacheLoaded:      false,
		cacheLastUpdated: time.Now(),
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
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
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

	// Debug: list what's actually in embeddedFiles
	rootFiles, _ := fs.Glob(embeddedFiles, "*")
	log.Printf("[TemplateInit] Root files in embedded FS: %v", rootFiles)

	// Create a sub-filesystem rooted at ui/templates for simpler template names
	templatesFS, err := fs.Sub(embeddedFiles, "ui/templates")
	if err != nil {
		log.Printf("[TemplateInit] Error creating templates filesystem: %v", err)
		templatesFiles, _ := fs.Glob(embeddedFiles, "templates/*")
		log.Printf("[TemplateInit] Templates in embedded FS: %v", templatesFiles)
		return fmt.Errorf("failed to create templates filesystem: %w", err)
	}

	var parseErr error
	s.templates = template.New("").Funcs(funcMap)

	// Parse all template files individually to ensure proper naming
	files1, err := fs.Glob(templatesFS, "*.html")
	if err != nil {
		log.Printf("[TemplateInit] Error globbing root templates: %v", err)
		return fmt.Errorf("failed to glob root templates: %w", err)
	}

	files2, err := fs.Glob(templatesFS, "**/*.html")
	if err != nil {
		log.Printf("[TemplateInit] Error globbing nested templates: %v", err)
		return fmt.Errorf("failed to glob nested templates: %w", err)
	}

	files := append(files1, files2...)
	log.Printf("[TemplateInit] Found %d template files: %v", len(files), files)

	// Debug: list all files in templatesFS
	allFiles, _ := fs.Glob(templatesFS, "*")
	log.Printf("[TemplateInit] All files in templatesFS: %v", allFiles)

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

	// Start background dataset loader
	s.startDatasetLoader()

	return nil
}

// setupMiddleware configures Gin middleware
func (s *Server) setupMiddleware() {
	// Serve static files from embedded filesystem
	// The embed directive includes "ui/static/*" so static files are at "ui/static/" root
	staticFS, err := fs.Sub(s.embeddedFiles, "ui/static")
	if err != nil {
		log.Printf("[setupMiddleware] Error creating static filesystem: %v", err)
		// Fallback: serve individual static files directly
		s.router.GET("/static/css/research.css", func(c *gin.Context) {
			log.Printf("[Static] Serving research.css fallback")
			c.Header("Content-Type", "text/css")
			content, err := s.embeddedFiles.ReadFile("ui/static/css/research.css")
			if err != nil {
				log.Printf("[Static] CSS file not found: %v", err)
				c.String(404, "CSS file not found")
				return
			}
			log.Printf("[Static] Served research.css (%d bytes)", len(content))
			c.String(200, string(content))
		})
		s.router.GET("/static/js/research.js", func(c *gin.Context) {
			log.Printf("[Static] Serving research.js fallback")
			c.Header("Content-Type", "application/javascript")
			content, err := s.embeddedFiles.ReadFile("ui/static/js/research.js")
			if err != nil {
				log.Printf("[Static] JS file not found: %v", err)
				c.String(404, "JS file not found")
				return
			}
			log.Printf("[Static] Served research.js (%d bytes)", len(content))
			c.String(200, string(content))
		})
	} else {
		log.Printf("[Static] Serving static files from embedded FS at /static")
		s.router.StaticFS("/static", http.FS(staticFS))
	}
}

// setupRoutes configures the application routes
func (s *Server) setupRoutes() {
	// Blueprint Dashboard - Single Page Application
	s.router.GET("/", s.handleIndex)
	s.router.GET("/mission-control", s.handleMissionControl)

	// API endpoints for blueprint dashboard data
	s.router.GET("/api/fields/list", s.handleFieldsList)

	// HTMX endpoints for async data loading
	s.router.GET("/api/dataset/status", s.handleDatasetStatus)
	s.router.GET("/api/dataset/info", s.handleDatasetInfo)
	s.router.GET("/api/fields/load-more", s.handleLoadMoreFields)
}

// Start starts the web server
func (s *Server) Start(addr string) error {
	log.Printf("Starting GoHypo UI on http://%s", addr)
	return s.router.Run(addr)
}

// handleMissionControl serves the Mission Control real-time dashboard
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

// loadDatasetInfo loads and processes all dataset information asynchronously
func (s *Server) loadDatasetInfo(ctx context.Context) error {
	loadStart := time.Now()

	// Get all artifacts to extract dataset and field info
	artifactStart := time.Now()
	allArtifacts, err := s.reader.ListArtifacts(ctx, ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		log.Printf("[loadDatasetInfo] ERROR: Failed to load artifacts: %v", err)
		return err
	}
	artifactTime := time.Since(artifactStart)
	_ = artifactTime

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
	profileMap := make(map[string]profiling.FieldProfile)

	// First, add ALL fields from Excel file to ensure complete coverage and profile them
	if excelData, err := s.getExcelData(); err == nil {
		// Profile the data
		coercerInstance := coercer.NewTypeCoercer(coercer.DefaultCoercionConfig())
		profiler := datareadiness.NewProfilerAdapter(coercerInstance)

		// Convert Excel rows to CanonicalEvents for profiling
		events := make([]ingestion.CanonicalEvent, len(excelData.Rows))
		for i, row := range excelData.Rows {
			payload := make(map[string]interface{})
			for k, v := range row {
				payload[k] = v
			}
			events[i] = ingestion.CanonicalEvent{
				RawPayload: payload,
			}
		}

		profileConfig := profiling.DefaultProfilingConfig()
		if result, err := profiler.ProfileSource(ctx, "dataset", events, profileConfig); err == nil {
			for _, profile := range result.Profiles {
				profileMap[profile.FieldKey] = profile
			}
		} else {
			log.Printf("[loadDatasetInfo] Warning: Failed to profile source data: %v", err)
		}

		excelFields := excelData.Headers
		// Show all fields for complete inventory (can be scrolled)
		maxFields := 50
		for i, fieldName := range excelFields {
			if i >= maxFields {
				break
			}
			fieldSet[fieldName] = true
		}
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

			// Try to extract dataset info from run manifest
			// Handle both map[string]interface{} (legacy) and *run.RunManifestArtifact (current)
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if snapshotID, ok := payload["snapshot_id"].(string); ok && datasetInfo["snapshotID"] == "" {
					datasetInfo["snapshotID"] = snapshotID

					// Derive dataset name from snapshot_id (e.g., "test-snapshot-shopping" -> "Shopping Dataset")
					if datasetInfo["name"] == "Unknown Dataset" {
						if len(snapshotID) > 0 {
							// Try to extract meaningful name from snapshot_id
							if len(snapshotID) > 14 && snapshotID[:14] == "test-snapshot-" {
								namePart := snapshotID[14:]
								if namePart != "" {
									datasetInfo["name"] = fmt.Sprintf("%s Dataset", namePart)
								}
							} else {
								datasetInfo["name"] = fmt.Sprintf("Dataset %s", snapshotID[:8])
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
				}
			} else if runManifest, ok := artifact.Payload.(*run.RunManifestArtifact); ok {
				// Handle structured RunManifestArtifact
				if string(runManifest.SnapshotID) != "" && datasetInfo["snapshotID"] == "" {
					datasetInfo["snapshotID"] = string(runManifest.SnapshotID)

					// Derive dataset name from snapshot_id
					if datasetInfo["name"] == "Unknown Dataset" {
						snapshotID := string(runManifest.SnapshotID)
						if len(snapshotID) > 0 {
							if len(snapshotID) > 14 && snapshotID[:14] == "test-snapshot-" {
								namePart := snapshotID[14:]
								if namePart != "" {
									datasetInfo["name"] = fmt.Sprintf("%s Dataset", namePart)
								}
							} else {
								datasetInfo["name"] = fmt.Sprintf("Dataset %s", snapshotID[:8])
							}
						}
					}
				}
				if !runManifest.SnapshotAt.Time().IsZero() && datasetInfo["snapshotAt"] == "" {
					datasetInfo["snapshotAt"] = runManifest.SnapshotAt.String()
				}
				if !runManifest.CutoffAt.Time().IsZero() && datasetInfo["cutoffAt"] == "" {
					datasetInfo["cutoffAt"] = runManifest.CutoffAt.String()
				}
			} else {
				log.Printf("[loadDatasetInfo] WARNING: Run artifact payload type not recognized: %T", artifact.Payload)
			}
		}

		if artifact.Kind == core.ArtifactRelationship {
			relationshipCount++
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
			} else {
				log.Printf("[loadDatasetInfo] WARNING: Relationship artifact payload type not recognized: %T", artifact.Payload)
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
	relArtifacts, _ := s.reader.ListArtifacts(ctx, relFilters)

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

	hasQValue := fdrArtifactCount > 0
	significanceRule := "p ≤ 0.05"
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
	fieldStatsMap := make(map[string]*FieldStats)

	// Initialize field stats with basic info for fast loading
	for _, field := range fields {
		// Use profiled data if available, otherwise default
		var stat *FieldStats
		if profile, ok := profileMap[field]; ok {
			variance := 0.0
			if profile.TypeSpecific.NumericStats != nil {
				stdDev := profile.TypeSpecific.NumericStats.StdDev
				variance = stdDev * stdDev
			}

			// Map inferred type to simple UI type
			uiType := "numeric" // default
			if profile.InferredType == profiling.TypeCategorical {
				uiType = "categorical"
			} else if profile.InferredType == profiling.TypeBoolean {
				uiType = "boolean"
			} else if profile.InferredType == profiling.TypeTimestamp {
				uiType = "timestamp"
			} else if profile.InferredType == profiling.TypeText {
				uiType = "string"
			}

			stat = &FieldStats{
				Name:              field,
				MissingRate:       profile.MissingStats.MissingRate,
				MissingRatePct:    fmt.Sprintf("%.1f", profile.MissingStats.MissingRate*100),
				UniqueCount:       profile.Cardinality.UniqueCount,
				Variance:          variance,
				Cardinality:       profile.Cardinality.UniqueCount,
				Type:              uiType,
				SampleSize:        profile.SampleSize,
				InRelationships:   0,
				StrongestCorr:     0.0,
				AvgEffectSize:     0.0,
				SignificantRels:   0,
				TotalRelsAnalyzed: 0,
			}
		} else {
			// Create lightweight field stats for initial load
			stat = &FieldStats{
				Name:              field,
				MissingRate:       0.0, // Will be calculated later if needed
				MissingRatePct:    "—", // Placeholder
				UniqueCount:       0,
				Variance:          0.0,
				Cardinality:       0,
				Type:              "numeric", // Default assumption
				SampleSize:        0,
				InRelationships:   0,
				StrongestCorr:     0.0,
				AvgEffectSize:     0.0,
				SignificantRels:   0,
				TotalRelsAnalyzed: 0,
			}
		}
		fieldStatsMap[field] = stat
	}

	// Process relationship artifacts to populate field statistics
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
		var pValue float64

		// Extract data from different artifact payload types
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
			pValue = relArtifact.Metrics.PValue
		} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			varX = string(relPayload.VariableX)
			varY = string(relPayload.VariableY)
			sampleSize = relPayload.SampleSize
			pValue = relPayload.PValue
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
			if pv, ok := payload["p_value"].(float64); ok {
				pValue = pv
			}

			// Extract data quality information
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

		// Update field stats for both variables
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
			statsX.TotalRelsAnalyzed++

			// Track effect sizes and significance
			if pValue < 0.05 {
				statsX.SignificantRels++
			}
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
			statsY.TotalRelsAnalyzed++

			// Track effect sizes and significance
			if pValue < 0.05 {
				statsY.SignificantRels++
			}
		}
	}

	// Calculate averages and final statistics for each field
	for _, stat := range fieldStatsMap {
		if stat.TotalRelsAnalyzed > 0 {
			// Calculate average effect size if we have relationships
			// (This would need more complex tracking, for now just set basic stats)
			stat.AvgEffectSize = 0.0 // Placeholder - would need to track all effect sizes
		}

		// Determine strongest correlation (placeholder - would need tracking)
		stat.StrongestCorr = 0.0
	}

	fieldStats := make([]*FieldStats, 0, len(fields))
	totalMissingRate := 0.0
	fieldCount := 0
	for _, stat := range fieldStatsMap {
		fieldStats = append(fieldStats, stat)
		if stat.SampleSize > 0 {
			totalMissingRate += stat.MissingRate
			fieldCount++
		}
	}

	// Calculate overall missingness rate
	missingnessOverall := 0.0
	if fieldCount > 0 {
		missingnessOverall = totalMissingRate / float64(fieldCount)
	}
	datasetInfo["missingnessOverall"] = missingnessOverall

	// Create field relationships placeholder
	fieldRelationships := make([]map[string]interface{}, 0)

	// Research control placeholder
	researchControl := map[string]interface{}{
		"canStart": true,
		"status":   "ready",
	}

	// Cache all the computed data
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.datasetCache = map[string]interface{}{
		"DatasetInfo":        datasetInfo,
		"RunStatus":          runStatus,
		"RelationshipCount":  relationshipCount,
		"SignificanceRule":   significanceRule,
		"PairsAttempted":     pairsAttempted,
		"PairsTested":        pairsTested,
		"PairsSkipped":       pairsSkipped,
		"SignificantCount":   significantCount,
		"Seed":               int64(42), // placeholder
		"Fingerprint":        "test-fingerprint",
		"RegistryHash":       "test-registry-hash",
		"StageStatuses":      stageStatuses,
		"VariablesTotal":     len(fields),
		"VariablesEligible":  len(fields),
		"VariablesRejected":  0,
		"FieldStats":         fieldStats,
		"FieldRelationships": fieldRelationships,
		"ResearchControl":    researchControl,
	}

	s.cacheLoaded = true
	s.cacheLastUpdated = time.Now()

	_ = time.Since(loadStart)
	return nil
}

// startDatasetLoader starts a background goroutine to load dataset information
func (s *Server) startDatasetLoader() {
	go func() {
		ctx := context.Background()
		// Load once immediately
		if err := s.loadDatasetInfo(ctx); err != nil {
			log.Printf("[startDatasetLoader] Error loading dataset info: %v", err)
		}
		// Then reload every 5 minutes (reduced frequency since data doesn't change often)
		for {
			time.Sleep(5 * time.Minute)
			if err := s.loadDatasetInfo(ctx); err != nil {
				log.Printf("[startDatasetLoader] Error loading dataset info: %v", err)
			}
		}
	}()
}

// handleIndex renders the main index page with halftone matrix visualization
func (s *Server) handleIndex(c *gin.Context) {
	log.Printf("[handleIndex] Starting index page render")

	// Check if dataset info is loaded
	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	cacheData := s.datasetCache
	s.cacheMutex.RUnlock()

	if !cacheLoaded {
		// If cache is not loaded yet, show loading page
		log.Printf("[handleIndex] Dataset info not loaded yet, showing loading state")
		data := map[string]interface{}{
			"Loading": true,
			"DatasetInfo": map[string]interface{}{
				"name": "Loading dataset information...",
			},
		}
		s.renderTemplate(c, "index.html", data)
		return
	}

	// Use cached data for fast rendering
	log.Printf("[handleIndex] Using cached dataset info, rendering template")
	s.renderTemplate(c, "index.html", cacheData)
}

// handleFieldsList returns field information for the UI
func (s *Server) handleFieldsList(c *gin.Context) {
	// Use cached data if available
	s.cacheMutex.RLock()
	cacheLoaded := s.cacheLoaded
	var fields []string
	if cacheLoaded && s.datasetCache != nil {
		if varsTotal, ok := s.datasetCache["VariablesTotal"].(int); ok {
			// For now, return a simple response
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

// handleDatasetStatus returns the current loading status of dataset information
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

// handleDatasetInfo returns dataset information for HTMX updates
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

	// Get all Excel fields
	excelData, err := s.getExcelData()
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load field names")
		return
	}
	excelFields := excelData.Headers

	// Calculate pagination
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

	// Get relationship artifacts for field statistics
	relKind := core.ArtifactRelationship
	relArtifacts, err := s.reader.ListArtifacts(context.Background(), ports.ArtifactFilters{
		Kind:  &relKind,
		Limit: 1000,
	})
	if err != nil {
		relArtifacts = []core.Artifact{}
	}

	// Create a map of field stats from relationship data
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

	// Process relationship artifacts to populate field statistics
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

		// Extract data from different artifact payload types
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

			// Extract data quality information
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

		// Update field stats for both variables (only if they're in our requested range)
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

	// Generate HTML for the additional field rows
	var htmlBuilder strings.Builder
	for i := start; i < end; i++ {
		fieldName := excelFields[i]
		fieldStat := fieldStatsMap[fieldName]

		// Determine type display
		typeDisplay := "NUM"
		typeClass := "bg-blue-50 text-blue-700 border-blue-100"
		if fieldStat.Type == "categorical" {
			typeDisplay = "CAT"
			typeClass = "bg-gray-50 text-gray-600 border-gray-200"
		}

		// Generate HTML for each field row (matching the template structure)
		htmlBuilder.WriteString(fmt.Sprintf(`
                    <div class="px-4 py-2 hover:bg-gray-50/80 transition-colors group">
                        <div class="grid grid-cols-8 gap-4 items-center text-xs">
                            <div class="col-span-2 min-w-0">
                                <div class="font-medium text-gray-900 truncate group-hover:text-blue-600 transition-colors" title="%s">%s</div>
                            </div>
                            <div class="text-center">
                                <span class="inline-flex items-center px-1.5 py-0.5 rounded-sm text-[10px] font-medium border %s">
                                    %s
                                </span>
                            </div>
                            <div class="text-center">
                                <span class="font-mono text-[11px] %s">%s%%</span>
                            </div>
                            <div class="text-center">
                                <span class="font-mono text-[11px] text-gray-600">%s</span>
                            </div>
                            <div class="text-center">
                                <span class="font-mono text-[11px] text-gray-600">%d</span>
                            </div>
                            <div class="text-center">
                                <span class="font-mono text-[11px] text-blue-600 font-medium bg-blue-50 px-1.5 py-0.5 rounded-sm">%d</span>
                            </div>
                            <div class="text-center">
                                <span class="font-mono text-[11px] text-gray-300">—</span>
                            </div>
                        </div>
                    </div>`,
			fieldName, fieldName,
			typeClass, typeDisplay,
			func() string {
				if fieldStat.MissingRate > 0.05 {
					return "text-red-600 font-medium"
				}
				return "text-gray-500"
			}(),
			fieldStat.MissingRatePct,
			func() string {
				if fieldStat.Variance > 0 {
					return fmt.Sprintf("%.2f", fieldStat.Variance)
				}
				return "0.00"
			}(),
			fieldStat.UniqueCount,
			fieldStat.InRelationships))
	}

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, htmlBuilder.String())
}

// getExcelData extracts data from Excel/CSV file if available
func (s *Server) getExcelData() (*excel.ExcelData, error) {
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		return nil, fmt.Errorf("EXCEL_FILE not set")
	}

	// Check if file exists
	if _, err := os.Stat(excelFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("Excel file not found: %s", excelFile)
	}

	// Read Excel data to get column information
	reader := excel.NewDataReader(excelFile)
	data, err := reader.ReadData()
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel file: %w", err)
	}

	return data, nil
}
