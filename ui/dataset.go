package ui

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	processor "gohypo/internal/dataset"

	"github.com/gin-gonic/gin"
)

func (s *Server) loadDatasetInfo(ctx context.Context) error {
	log.Printf("[loadDatasetInfo] Loading dataset from DATABASE ONLY")

	if s.datasetRepository == nil {
		log.Printf("[loadDatasetInfo] ERROR: dataset repository not available")
		return fmt.Errorf("dataset repository not available")
	}

	// Get the current dataset from database ONLY - NO FALLBACKS TO CSV
	currentDataset, err := s.datasetRepository.GetCurrent(ctx)
	if err != nil {
		log.Printf("[loadDatasetInfo] ERROR: Failed to get current dataset from database: %v", err)
		return fmt.Errorf("failed to load dataset from database: %w", err)
	}

	if currentDataset == nil {
		log.Printf("[loadDatasetInfo] ERROR: No current dataset found in database")
		return fmt.Errorf("no current dataset found in database - please upload or create a dataset first")
	}

	if currentDataset.Status != dataset.StatusReady {
		log.Printf("[loadDatasetInfo] ERROR: Dataset is not ready (status: %s)", currentDataset.Status)
		return fmt.Errorf("dataset is not ready for use (status: %s)", currentDataset.Status)
	}

	log.Printf("[loadDatasetInfo] Successfully loaded dataset from database: %s (%s)", currentDataset.DisplayName, currentDataset.Domain)

	// Use data from the database dataset

	// Dataset info from database
	datasetInfo := map[string]interface{}{
		"name":                currentDataset.DisplayName,
		"domain":              currentDataset.Domain,
		"description":         currentDataset.Description,
		"record_count":        currentDataset.RecordCount,
		"field_count":         currentDataset.FieldCount,
		"missingness_overall": currentDataset.MissingRate,
		"fileSize":            currentDataset.FileSize,
		"mimeType":            currentDataset.MimeType,
		"last_updated":        currentDataset.UpdatedAt.Format(time.RFC3339),
	}
	s.datasetCache["DatasetInfo"] = datasetInfo

	// Create field statistics from dataset metadata
	fieldStats := make([]map[string]interface{}, 0, currentDataset.FieldCount)
	if currentDataset.Metadata.Fields != nil && len(currentDataset.Metadata.Fields) > 0 {
		for _, field := range currentDataset.Metadata.Fields {
			fieldStat := map[string]interface{}{
				"name":       field.Name,
				"type":       string(field.DataType),
				"sampleSize": currentDataset.RecordCount,
			}
			fieldStats = append(fieldStats, fieldStat)
		}
		log.Printf("[loadDatasetInfo] Created %d field stats from dataset metadata", len(fieldStats))
	} else {
		// Fallback: create basic field stats
		for i := 1; i <= currentDataset.FieldCount; i++ {
			fieldStat := map[string]interface{}{
				"name":       fmt.Sprintf("field_%d", i),
				"type":       "numeric",
				"sampleSize": currentDataset.RecordCount,
			}
			fieldStats = append(fieldStats, fieldStat)
		}
		log.Printf("[loadDatasetInfo] Created %d basic field stats", len(fieldStats))
	}
	s.datasetCache["FieldStats"] = fieldStats

	// Create sample rows from dataset metadata
	var sampleRows []map[string]interface{}
	if currentDataset.Metadata.SampleRows != nil && len(currentDataset.Metadata.SampleRows) > 0 {
		sampleRows = currentDataset.Metadata.SampleRows
		log.Printf("[loadDatasetInfo] Using %d sample rows from dataset metadata", len(sampleRows))
	} else {
		// No sample rows available - create empty array
		sampleRows = make([]map[string]interface{}, 0)
		log.Printf("[loadDatasetInfo] No sample rows available in dataset metadata")
	}
	s.datasetCache["SampleRows"] = sampleRows

	// Variables total
	s.datasetCache["VariablesTotal"] = currentDataset.FieldCount

	// Populate dataset cache
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Mark cache as loaded
	s.cacheLoaded = true

	log.Printf("[loadDatasetInfo] Successfully loaded dataset from database: %d columns, %d rows", currentDataset.FieldCount, currentDataset.RecordCount)
	log.Printf("[loadDatasetInfo] Dataset cache loaded successfully - APIs should now work")
	return nil
}

// startDatasetLoader starts a background goroutine to load dataset information
func (s *Server) startDatasetLoader() {
	log.Printf("[startDatasetLoader] Starting dataset loader background process")
	go func() {
		// Small delay to ensure server is fully started before heavy dataset processing
		time.Sleep(100 * time.Millisecond)

		ctx := context.Background()
		log.Printf("[startDatasetLoader] Performing initial dataset load")
		if err := s.loadDatasetInfo(ctx); err != nil {
			log.Printf("[startDatasetLoader] CRITICAL - Initial dataset load failed: %v", err)
			log.Printf("[startDatasetLoader] Dataset will remain in loading state until next retry")
		}
		for {
			time.Sleep(5 * time.Minute)
			log.Printf("[startDatasetLoader] Starting periodic dataset refresh")
			if err := s.loadDatasetInfo(ctx); err != nil {
				log.Printf("[startDatasetLoader] ERROR - Dataset refresh failed: %v", err)
			} else {
				log.Printf("[startDatasetLoader] Dataset refresh completed successfully")
			}
		}
	}()
}

// handleFileUpload handles dataset file uploads
func (s *Server) handleFileUpload(c *gin.Context) {
	log.Printf("[handleFileUpload] Starting file upload process")

	// Get the uploaded file
	file, header, err := c.Request.FormFile("dataset")
	if err != nil {
		log.Printf("[handleFileUpload] FAILED - No file uploaded: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}
	defer file.Close()

	// Validate file size (50MB limit)
	const maxFileSize = 50 * 1024 * 1024 // 50MB
	if header.Size > maxFileSize {
		log.Printf("[handleFileUpload] FAILED - File too large: %d bytes", header.Size)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("File size (%.1f MB) exceeds the 50MB limit", float64(header.Size)/(1024*1024))})
		return
	}

	// Validate file type
	filename := header.Filename
	contentType := header.Header.Get("Content-Type")

	// Check file extension
	validExtensions := []string{".xlsx", ".xls", ".csv"}
	hasValidExtension := false
	for _, ext := range validExtensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			hasValidExtension = true
			break
		}
	}

	if !hasValidExtension {
		log.Printf("[handleFileUpload] FAILED - Invalid file extension: %s", filename)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only Excel (.xlsx, .xls) and CSV (.csv) files are allowed"})
		return
	}

	// Validate MIME type for additional security
	validMimeTypes := []string{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", // .xlsx
		"application/vnd.ms-excel", // .xls
		"text/csv",
		"application/csv",
		"text/plain", // Some CSV files might be detected as plain text
	}

	isValidMimeType := false
	for _, mimeType := range validMimeTypes {
		if contentType == mimeType {
			isValidMimeType = true
			break
		}
	}

	if !isValidMimeType && !strings.Contains(contentType, "excel") && !strings.Contains(contentType, "csv") {
		log.Printf("[handleFileUpload] WARNING - Unexpected MIME type: %s for file: %s", contentType, filename)
		// Don't reject yet, but log the warning - some systems might not detect MIME types correctly
	}

	// Get user ID from context (for now, use default user)
	userID := core.ID("550e8400-e29b-41d4-a716-446655440000") // Default user for single-user mode

	// Get workspace ID from form data, default to user's default workspace
	workspaceIDStr := c.PostForm("workspace_id")
	workspaceID := core.ID(workspaceIDStr)

	// If no workspace specified, ensure user has a default workspace and use it
	if workspaceID == "" {
		if s.workspaceRepository != nil {
			defaultWorkspace, err := s.ensureDefaultWorkspace(c.Request.Context(), userID)
			if err != nil {
				log.Printf("[handleFileUpload] Failed to ensure default workspace: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to setup workspace"})
				return
			}
			workspaceID = defaultWorkspace.ID
		} else {
			// Fallback if workspace repository not available
			workspaceID = core.ID("550e8400-e29b-41d4-a716-446655440001")
		}
	}

	// Create upload object for processor
	upload := &dataset.DatasetUpload{
		UserID:      userID,
		WorkspaceID: workspaceID,
		Filename:    filename,
		File:        file,
		MimeType:    header.Header.Get("Content-Type"),
	}

	// Process the dataset using the new processor
	ctx := context.Background()
	datasetID, err := s.datasetProcessor.ProcessUpload(ctx, upload)
	if err != nil {
		log.Printf("[handleFileUpload] FAILED - Dataset processing failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to process dataset: %v", err)})
		return
	}

	// Return success response with dataset ID
	c.JSON(http.StatusOK, gin.H{
		"message":      "Dataset uploaded and processing started",
		"dataset_id":   datasetID,
		"dataset_name": "", // Will be available after processing completes
		"workspace_id": workspaceID,
	})
}

// handleMergeDatasets handles dataset merging requests
func (s *Server) handleMergeDatasets(c *gin.Context) {
	if s.datasetProcessor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Dataset processor not available"})
		return
	}

	var req struct {
		DatasetIDs    []string `json:"dataset_ids" binding:"required"`
		OutputName    string   `json:"output_name" binding:"required"`
		WorkspaceID   string   `json:"workspace_id"`
		MergeStrategy string   `json:"merge_strategy"`
		JoinType      string   `json:"join_type"`
		MergeConfig   struct {
			Strategy       string `json:"strategy"`
			JoinType       string `json:"join_type"`
			AutoMode       bool   `json:"auto_mode"`
			TemporalConfig struct {
				TimeColumn       string  `json:"time_column"`
				TimeFormat       string  `json:"time_format"`
				SourceTimeZone   string  `json:"source_time_zone"`
				TargetTimeZone   string  `json:"target_time_zone"`
				Frequency        string  `json:"frequency"`
				DetectFrequency  bool    `json:"detect_frequency"`
				GapFillStrategy  string  `json:"gap_fill_strategy"`
				Interpolation    string  `json:"interpolation"`
				MaxGapDuration   string  `json:"max_gap_duration"`
				SortByTime       bool    `json:"sort_by_time"`
				DeduplicateBy    string  `json:"deduplicate_by"`
				OutlierDetection bool    `json:"outlier_detection"`
				OutlierThreshold float64 `json:"outlier_threshold"`
			} `json:"temporal_config"`
		} `json:"merge_config"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if len(req.DatasetIDs) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least 2 datasets required for merging"})
		return
	}

	// Convert string IDs to core.ID
	sourceIDs := make([]core.ID, len(req.DatasetIDs))
	for i, idStr := range req.DatasetIDs {
		sourceIDs[i] = core.ID(idStr)
	}

	// Default workspace if not specified
	workspaceID := core.ID(req.WorkspaceID)
	if workspaceID == "" {
		userID := core.ID("550e8400-e29b-41d4-a716-446655440000") // Default user
		if s.workspaceRepository != nil {
			if defaultWorkspace, err := s.ensureDefaultWorkspace(c.Request.Context(), userID); err == nil {
				workspaceID = defaultWorkspace.ID
			}
		}
	}

	// Set default merge configuration
	config := &processor.MergeConfig{
		Strategy:       processor.HybridMerge,
		JoinType:       processor.UnionJoin,
		ValidateSchema: true,
	}

	// Override based on merge_config if provided (auto-merge mode)
	if req.MergeConfig.Strategy != "" {
		switch req.MergeConfig.Strategy {
		case "hybrid":
			config.Strategy = processor.HybridMerge
		case "streaming":
			config.Strategy = processor.StreamingMerge
		case "in_memory":
			config.Strategy = processor.InMemoryMerge
		case "database":
			config.Strategy = processor.DatabaseMerge
		}
	}

	if req.MergeConfig.JoinType != "" {
		switch req.MergeConfig.JoinType {
		case "union":
			config.JoinType = processor.UnionJoin
		case "inner":
			config.JoinType = processor.InnerJoin
		case "left":
			config.JoinType = processor.LeftJoin
		case "outer":
			config.JoinType = processor.OuterJoin
		}
	}

	// Handle temporal configuration
	if req.MergeConfig.TemporalConfig.TimeColumn != "" {
		temporalConfig := &processor.TemporalMergeConfig{
			TimeColumn:       req.MergeConfig.TemporalConfig.TimeColumn,
			TimeFormat:       req.MergeConfig.TemporalConfig.TimeFormat,
			SourceTimeZone:   req.MergeConfig.TemporalConfig.SourceTimeZone,
			TargetTimeZone:   req.MergeConfig.TemporalConfig.TargetTimeZone,
			DetectFrequency:  req.MergeConfig.TemporalConfig.DetectFrequency,
			SortByTime:       req.MergeConfig.TemporalConfig.SortByTime,
			DeduplicateBy:    processor.DeduplicateByTime(req.MergeConfig.TemporalConfig.DeduplicateBy),
			OutlierDetection: req.MergeConfig.TemporalConfig.OutlierDetection,
			OutlierThreshold: req.MergeConfig.TemporalConfig.OutlierThreshold,
		}

		// Set frequency
		switch req.MergeConfig.TemporalConfig.Frequency {
		case "second":
			temporalConfig.Frequency = processor.FrequencySecond
		case "minute":
			temporalConfig.Frequency = processor.FrequencyMinute
		case "hour":
			temporalConfig.Frequency = processor.FrequencyHour
		case "day":
			temporalConfig.Frequency = processor.FrequencyDay
		case "week":
			temporalConfig.Frequency = processor.FrequencyWeek
		case "month":
			temporalConfig.Frequency = processor.FrequencyMonth
		case "year":
			temporalConfig.Frequency = processor.FrequencyYear
		default:
			temporalConfig.Frequency = processor.FrequencyUnknown
		}

		// Set gap fill strategy
		switch req.MergeConfig.TemporalConfig.GapFillStrategy {
		case "forward":
			temporalConfig.GapFillStrategy = processor.GapFillForward
		case "backward":
			temporalConfig.GapFillStrategy = processor.GapFillBackward
		case "interpolate":
			temporalConfig.GapFillStrategy = processor.GapFillInterpolate
		case "zero":
			temporalConfig.GapFillStrategy = processor.GapFillZero
		default:
			temporalConfig.GapFillStrategy = processor.GapFillNone
		}

		// Set interpolation type
		switch req.MergeConfig.TemporalConfig.Interpolation {
		case "linear":
			temporalConfig.Interpolation = processor.InterpolateLinear
		case "spline":
			temporalConfig.Interpolation = processor.InterpolateSpline
		default:
			temporalConfig.Interpolation = processor.InterpolateNone
		}

		// Parse max gap duration
		if req.MergeConfig.TemporalConfig.MaxGapDuration != "" {
			if duration, err := time.ParseDuration(req.MergeConfig.TemporalConfig.MaxGapDuration); err == nil {
				temporalConfig.MaxGapDuration = duration
			}
		}

		config.TemporalConfig = temporalConfig
	}

	// Override based on legacy request fields (for backward compatibility)
	if req.MergeStrategy == "streaming" && req.MergeConfig.Strategy == "" {
		config.Strategy = processor.StreamingMerge
	}
	if req.JoinType == "inner" && req.MergeConfig.JoinType == "" {
		config.JoinType = processor.InnerJoin
	} else if req.JoinType == "left" && req.MergeConfig.JoinType == "" {
		config.JoinType = processor.LeftJoin
	}

	// Auto-merge mode optimizations
	if req.MergeConfig.AutoMode {
		// For auto-merge, use more aggressive validation and optimization
		config.ValidateSchema = true
		config.DuplicatePolicy = processor.KeepFirst // Default policy for auto-merge
	}

	// Start merge operation
	ctx := context.Background()
	mergeResult, err := s.datasetProcessor.Merger.MergeDatasets(ctx, sourceIDs, req.OutputName, config)
	if err != nil {
		log.Printf("[handleMergeDatasets] Merge failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Merge operation failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Merge operation completed successfully",
		"output_path":  mergeResult.OutputPath,
		"status":       "completed",
		"row_count":    mergeResult.RowCount,
		"column_count": mergeResult.ColumnCount,
		"dataset_ids":  req.DatasetIDs,
	})
}

// handleMergeStatus checks the status of a merge operation
func (s *Server) handleMergeStatus(c *gin.Context) {
	mergeID := c.Param("id")
	if mergeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Merge ID is required"})
		return
	}

	// For now, return a placeholder status
	// TODO: Implement actual merge status tracking
	c.JSON(http.StatusOK, gin.H{
		"merge_id": mergeID,
		"status":   "completed", // Assume completed for now
		"progress": 100,
		"message":  "Merge operation completed successfully",
	})
}
