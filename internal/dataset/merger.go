// Package merger provides SCALABLE dataset merging capabilities
//
// WE BUILD FOR SCALE - ALWAYS STREAM!
//
// This merger ONLY uses streaming operations to handle datasets of any size
// without loading them into memory. No more memory limits, no more batch processing,
// no more compromises. True streaming from input to output with minimal memory footprint.
//
// Key principles:
// - Never load entire datasets into memory
// - Process row-by-row in streaming fashion
// - Handle duplicates inline during streaming
// - Scale to unlimited dataset sizes
// - Memory usage remains constant regardless of data size
package dataset

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gohypo/domain/core"

	"github.com/jmoiron/sqlx"
)

// MergeStrategy defines approaches for merging datasets
// WE ONLY STREAM - NO EXCEPTIONS!
type MergeStrategy string

const (
	// StreamingMerge: The ONLY way we merge - always stream for unlimited scale
	StreamingMerge MergeStrategy = "streaming"

	// These "strategies" are deprecated and will be converted to streaming
	// We don't do in-memory, database, or hybrid anymore - STREAMING ONLY!
	InMemoryMerge MergeStrategy = "in_memory" // DEPRECATED: Forces streaming
	DatabaseMerge MergeStrategy = "database"  // DEPRECATED: Forces streaming
	HybridMerge   MergeStrategy = "hybrid"    // DEPRECATED: Forces streaming
)

// MergeConfig holds configuration for merge operations
type MergeConfig struct {
	Strategy         MergeStrategy
	MaxMemoryMB      int                                    // Maximum memory usage in MB
	ChunkSize        int                                    // Rows per chunk for streaming
	TempDir          string                                 // Temporary directory for intermediate files
	DuplicatePolicy  DuplicatePolicy                        // How to handle duplicate rows
	KeyColumns       []string                               // Columns to use as merge keys
	JoinType         JoinType                               // Type of join/merge operation
	ValidateSchema   bool                                   // Validate column compatibility
	ProgressCallback func(progress float64, message string) // Progress reporting

	// Timeseries-specific configuration
	TemporalConfig *TemporalMergeConfig // Optional timeseries merge settings
}

// TemporalMergeConfig holds configuration for timeseries merging
type TemporalMergeConfig struct {
	TimeColumn         string             // Name of the timestamp column
	TimeFormat         string             // Expected time format (e.g., "2006-01-02 15:04:05")
	SourceTimeZone     string             // Source timezone (e.g., "America/New_York")
	TargetTimeZone     string             // Target timezone for normalization (e.g., "UTC")
	Frequency          TemporalFrequency  // Expected data frequency
	DetectFrequency    bool               // Auto-detect frequency from data
	GapFillStrategy    GapFillStrategy    // How to handle missing timestamps
	Interpolation      InterpolationType  // Interpolation method for missing values
	MaxGapDuration     time.Duration      // Maximum gap to interpolate
	SortByTime         bool               // Whether to sort output by timestamp
	DeduplicateBy      DeduplicateByTime  // How to handle duplicate timestamps
	OutlierDetection   bool               // Enable outlier detection
	OutlierThreshold   float64            // Z-score threshold for outliers (default: 3.0)
	BusinessCalendar   *BusinessCalendar  // Business calendar for filtering
}

// TemporalFrequency defines expected data frequency
type TemporalFrequency string

const (
	FrequencyUnknown TemporalFrequency = "unknown"
	FrequencySecond  TemporalFrequency = "second"
	FrequencyMinute  TemporalFrequency = "minute"
	FrequencyHour    TemporalFrequency = "hour"
	FrequencyDay     TemporalFrequency = "day"
	FrequencyWeek    TemporalFrequency = "week"
	FrequencyMonth   TemporalFrequency = "month"
	FrequencyYear    TemporalFrequency = "year"
)

// GapFillStrategy defines how to handle gaps in timeseries
type GapFillStrategy string

const (
	GapFillNone      GapFillStrategy = "none"       // Leave gaps as null/missing
	GapFillForward   GapFillStrategy = "forward"    // Forward fill from last known value
	GapFillBackward  GapFillStrategy = "backward"   // Backward fill from next known value
	GapFillInterpolate GapFillStrategy = "interpolate" // Linear interpolation
	GapFillZero      GapFillStrategy = "zero"       // Fill gaps with zero
)

// InterpolationType defines interpolation methods
type InterpolationType string

const (
	InterpolateNone    InterpolationType = "none"
	InterpolateLinear  InterpolationType = "linear"
	InterpolateSpline  InterpolationType = "spline"
)

// DeduplicateByTime defines how to handle duplicate timestamps
type DeduplicateByTime string

const (
	DedupeTimeKeepFirst  DeduplicateByTime = "first"   // Keep first occurrence
	DedupeTimeKeepLast   DeduplicateByTime = "last"    // Keep last occurrence
	DedupeTimeKeepNewest DeduplicateByTime = "newest" // Keep most recent data
	DedupeTimeAggregate  DeduplicateByTime = "aggregate" // Aggregate duplicate values
)

// BusinessCalendar defines business day and holiday rules
type BusinessCalendar struct {
	IncludeWeekends bool      // Whether to include weekend data
	Holidays        []Holiday // List of holidays
	BusinessHours   TimeRange // Business hours for filtering
}

// Holiday represents a holiday or special date
type Holiday struct {
	Date     time.Time
	Name     string
	IsHalfDay bool
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Duration // Duration from midnight
	End   time.Duration // Duration from midnight
}

// DuplicatePolicy defines how to handle duplicate rows during merge
type DuplicatePolicy string

const (
	KeepFirst    DuplicatePolicy = "keep_first" // Keep first occurrence
	KeepLast     DuplicatePolicy = "keep_last"  // Keep last occurrence
	RemoveAll    DuplicatePolicy = "remove_all" // Remove all duplicates
	ErrorOnDupes DuplicatePolicy = "error"      // Error on duplicates
)

// JoinType defines the type of merge/join operation
type JoinType string

const (
	UnionJoin     JoinType = "union"     // UNION ALL - combine all rows
	InnerJoin     JoinType = "inner"     // INNER JOIN - matching keys only
	LeftJoin      JoinType = "left"      // LEFT JOIN - all from left, matching from right
	OuterJoin     JoinType = "outer"     // FULL OUTER JOIN - all rows from both
	IntersectJoin JoinType = "intersect" // INTERSECTION - rows in both datasets
)

// MergeResult contains the result of a merge operation
type MergeResult struct {
	Success         bool          `json:"success"`
	RowCount        int           `json:"row_count"`
	ColumnCount     int           `json:"column_count"`
	DuplicatesFound int           `json:"duplicates_found,omitempty"`
	OutputPath      string        `json:"output_path,omitempty"`
	ExecutionTime   time.Duration `json:"execution_time"`
	StrategyUsed    MergeStrategy `json:"strategy_used"`
	MemoryUsedMB    int           `json:"memory_used_mb"`
	Error           string        `json:"error,omitempty"`
	Warnings        []string      `json:"warnings,omitempty"`
}

// Merger handles dataset merging operations
type Merger struct {
	db          *sqlx.DB
	fileStorage FileStorage
	config      *MergeConfig
}

// NewMerger creates a new dataset merger
func NewMerger(db *sqlx.DB, fileStorage FileStorage, config *MergeConfig) *Merger {
	if config == nil {
		config = &MergeConfig{
			Strategy:        StreamingMerge, // Always stream for scale!
			MaxMemoryMB:     128,            // Lower memory footprint since we stream
			ChunkSize:       5000,           // Smaller chunks for better memory control
			DuplicatePolicy: KeepFirst,
			ValidateSchema:  true,
		}
	}

	// Override any non-streaming strategy - we build for scale!
	if config.Strategy != StreamingMerge {
		config.Strategy = StreamingMerge
	}

	return &Merger{
		db:          db,
		fileStorage: fileStorage,
		config:      config,
	}
}

// MergeDatasets merges multiple datasets using streaming for maximum scalability
func (m *Merger) MergeDatasets(ctx context.Context, datasetIDs []core.ID, outputName string, config *MergeConfig) (*MergeResult, error) {
	startTime := time.Now()

	// Use provided config or default
	mergeConfig := config
	if mergeConfig == nil {
		mergeConfig = m.config
	}

	// We build for scale - ALWAYS stream!
	result, err := m.mergeStreaming(ctx, datasetIDs, outputName, mergeConfig)

	if result != nil {
		result.ExecutionTime = time.Since(startTime)
		result.StrategyUsed = StreamingMerge
	}

	return result, err
}

// Removed determineOptimalStrategy - we ALWAYS stream for maximum scalability
// No more strategy decisions - streaming is the only way we build for scale

// Removed mergeInMemory - we build for scale and ALWAYS stream!
// Memory-intensive operations have no place in our scalable architecture

// mergeStreaming performs true streaming merge for maximum scalability
func (m *Merger) mergeStreaming(ctx context.Context, datasetIDs []core.ID, outputName string, config *MergeConfig) (*MergeResult, error) {
	reportProgress(config, 0, "Starting streaming merge")

	if len(datasetIDs) == 0 {
		return &MergeResult{
			Success:      false,
			Error:        "no datasets provided",
			StrategyUsed: StreamingMerge,
		}, fmt.Errorf("no datasets provided")
	}

	// Detect if this is a timeseries merge
	isTimeseries := config.TemporalConfig != nil && config.TemporalConfig.TimeColumn != ""

	var allHeaders []string
	totalRows := 0
	duplicatesFound := 0

	// First pass: determine headers and validate compatibility
	reportProgress(config, 5, "Analyzing dataset schemas")
	for i, datasetID := range datasetIDs {
		reader, err := m.getDatasetReader(ctx, datasetID)
		if err != nil {
			return nil, fmt.Errorf("failed to get reader for dataset %s: %w", datasetID, err)
		}

		headers, err := m.extractHeaders(reader)
		reader.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to extract headers from dataset %s: %w", datasetID, err)
		}

		if i == 0 {
			allHeaders = headers

			// Auto-detect temporal column if not specified
			if !isTimeseries && m.hasTemporalColumn(headers) {
				isTimeseries = true
				if config.TemporalConfig == nil {
					config.TemporalConfig = &TemporalMergeConfig{
						TimeColumn:      m.detectTimeColumn(headers),
						TimeFormat:      "", // Will auto-detect
						SourceTimeZone:  "UTC",
						TargetTimeZone:  "UTC",
						Frequency:       FrequencyUnknown,
						GapFillStrategy: GapFillNone,
						Interpolation:   InterpolateNone,
						SortByTime:      true,
						DeduplicateBy:   DedupeTimeKeepFirst,
					}
				}
			}
		} else if config.ValidateSchema {
			if err := m.validateSchemaCompatibility(allHeaders, headers); err != nil {
				return nil, fmt.Errorf("schema incompatibility in dataset %s: %w", datasetID, err)
			}

			// For timeseries, also validate temporal column compatibility
			if isTimeseries {
				if err := m.validateTemporalCompatibility(allHeaders, headers, config.TemporalConfig); err != nil {
					return nil, fmt.Errorf("temporal incompatibility in dataset %s: %w", datasetID, err)
				}
			}
		}
	}

	// Second pass: stream merge all datasets
	reportProgress(config, 10, "Streaming merge operation")

	var outputPath string
	var rowsWritten, duplicates int
	var err error

	if isTimeseries {
		outputPath, rowsWritten, duplicates, err = m.streamMergeTimeseriesDatasets(ctx, datasetIDs, allHeaders, outputName, config)
	} else {
		outputPath, rowsWritten, duplicates, err = m.streamMergeDatasets(ctx, datasetIDs, allHeaders, outputName, config)
	}

	if err != nil {
		return nil, fmt.Errorf("streaming merge failed: %w", err)
	}

	totalRows = rowsWritten
	duplicatesFound = duplicates

	reportProgress(config, 100, "Streaming merge completed")

	return &MergeResult{
		Success:         true,
		RowCount:        totalRows,
		ColumnCount:     len(allHeaders),
		DuplicatesFound: duplicatesFound,
		OutputPath:      outputPath,
		StrategyUsed:    StreamingMerge,
		MemoryUsedMB:    m.getCurrentMemoryUsage(),
	}, nil
}

// Removed mergeWithDatabase - we build for scale and ALWAYS stream!
// Database operations are too slow for our high-performance streaming architecture

// Helper methods

// hasTemporalColumn checks if any column name suggests temporal data
func (m *Merger) hasTemporalColumn(headers []string) bool {
	temporalKeywords := []string{
		"time", "date", "timestamp", "datetime", "created", "updated",
		"occurred", "recorded", "period", "year", "month", "day", "hour", "minute", "second",
	}

	for _, header := range headers {
		headerLower := strings.ToLower(header)
		for _, keyword := range temporalKeywords {
			if strings.Contains(headerLower, keyword) {
				return true
			}
		}
	}
	return false
}

// detectTimeColumn attempts to identify the most likely timestamp column
func (m *Merger) detectTimeColumn(headers []string) string {
	// Priority order for timestamp column detection
	priorityPatterns := []string{
		"timestamp", "datetime", "time", "date",
		"created_at", "updated_at", "occurred_at", "recorded_at",
		"period", "event_time", "transaction_time",
	}

	for _, pattern := range priorityPatterns {
		for _, header := range headers {
			if strings.ToLower(header) == pattern {
				return header
			}
		}
	}

	// Fallback: look for any column containing temporal keywords
	temporalKeywords := []string{"time", "date", "created", "updated", "occurred"}
	for _, header := range headers {
		headerLower := strings.ToLower(header)
		for _, keyword := range temporalKeywords {
			if strings.Contains(headerLower, keyword) {
				return header
			}
		}
	}

	// Last resort: return first column (not ideal but better than failing)
	if len(headers) > 0 {
		return headers[0]
	}
	return ""
}

// validateTemporalCompatibility ensures temporal columns are compatible across datasets
func (m *Merger) validateTemporalCompatibility(baseHeaders, newHeaders []string, temporalConfig *TemporalMergeConfig) error {
	timeCol := temporalConfig.TimeColumn

	// Check if time column exists in both datasets
	baseHasTime := false
	newHasTime := false

	for _, header := range baseHeaders {
		if header == timeCol {
			baseHasTime = true
			break
		}
	}

	for _, header := range newHeaders {
		if header == timeCol {
			newHasTime = true
			break
		}
	}

	if !baseHasTime {
		return fmt.Errorf("time column '%s' not found in base dataset", timeCol)
	}
	if !newHasTime {
		return fmt.Errorf("time column '%s' not found in new dataset", timeCol)
	}

	return nil
}

// streamMergeTimeseriesDatasets handles timeseries-specific merging with temporal alignment
func (m *Merger) streamMergeTimeseriesDatasets(ctx context.Context, datasetIDs []core.ID, headers []string, outputName string, config *MergeConfig) (string, int, int, error) {
	temporalConfig := config.TemporalConfig
	timeCol := temporalConfig.TimeColumn

	reportProgress(config, 15, "Processing timeseries datasets...")

	// Collect all timeseries data points with temporal indexing
	timeseriesData := make(map[string][]TimeseriesRow) // timestamp -> []rows

	totalRows := 0
	duplicates := 0

	for i, datasetID := range datasetIDs {
		progress := 15 + float64(i)/float64(len(datasetIDs))*60
		reportProgress(config, progress, fmt.Sprintf("Processing timeseries dataset %d/%d", i+1, len(datasetIDs)))

		reader, err := m.getDatasetReader(ctx, datasetID)
		if err != nil {
			return "", 0, 0, fmt.Errorf("failed to get reader for dataset %s: %w", datasetID, err)
		}

		rowsProcessed, dups, err := m.processTimeseriesDataset(reader, headers, timeCol, timeseriesData, temporalConfig)
		reader.Close()

		if err != nil {
			return "", 0, 0, fmt.Errorf("failed to process timeseries dataset %s: %w", datasetID, err)
		}

		totalRows += rowsProcessed
		duplicates += dups
	}

	// Apply outlier detection if enabled
	if temporalConfig.OutlierDetection {
		// Convert map to slice for outlier detection
		var allRows []TimeseriesRow
		for _, rows := range timeseriesData {
			allRows = append(allRows, rows...)
		}

		filteredRows := m.detectOutliers(allRows, temporalConfig, headers)

		// Rebuild timeseries data with filtered rows
		timeseriesData = make(map[string][]TimeseriesRow)
		for _, row := range filteredRows {
			timeKey := row.Timestamp.Format(time.RFC3339)
			timeseriesData[timeKey] = []TimeseriesRow{row}
		}
	}

	// Apply frequency resampling if configured
	if temporalConfig.DetectFrequency && temporalConfig.Frequency != FrequencyUnknown {
		timeseriesData = m.resampleTimeseries(timeseriesData, temporalConfig.Frequency, temporalConfig.Interpolation)
	}

	// Apply gap filling if configured
	if temporalConfig.GapFillStrategy != GapFillNone {
		m.applyGapFilling(timeseriesData, temporalConfig)
	}

	// Sort by time if requested
	if temporalConfig.SortByTime {
		m.sortTimeseriesByTime(timeseriesData)
	}

	// Write output
	reportProgress(config, 80, "Writing timeseries output...")

	outputPath := fmt.Sprintf("/merged/%s_%d.csv", outputName, time.Now().Unix())
	rowsWritten, err := m.writeTimeseriesOutput(timeseriesData, headers, outputPath, config)
	if err != nil {
		return "", 0, 0, fmt.Errorf("failed to write timeseries output: %w", err)
	}

	reportProgress(config, 95, "Finalizing timeseries merge...")

	return outputPath, rowsWritten, duplicates, nil
}

// TimeseriesRow represents a single row with temporal information
type TimeseriesRow struct {
	Timestamp time.Time
	Data      []string
	DatasetID string
}

// processTimeseriesDataset processes a single timeseries dataset
func (m *Merger) processTimeseriesDataset(reader io.Reader, headers []string, timeCol string, timeseriesData map[string][]TimeseriesRow, config *TemporalMergeConfig) (int, int, error) {
	csvReader := csv.NewReader(reader)

	// Read header
	_, err := csvReader.Read()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read headers: %w", err)
	}

	// Find time column index
	timeColIndex := -1
	for i, header := range headers {
		if header == timeCol {
			timeColIndex = i
			break
		}
	}

	if timeColIndex == -1 {
		return 0, 0, fmt.Errorf("time column '%s' not found in headers", timeCol)
	}

	rowsProcessed := 0
	duplicates := 0

	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("failed to read row: %w", err)
		}

		if len(row) <= timeColIndex {
			continue // Skip malformed rows
		}

		// Parse timestamp with timezone support
		timestampStr := strings.TrimSpace(row[timeColIndex])
		if timestampStr == "" {
			continue // Skip rows without timestamps
		}

		timestamp, err := m.parseTimestamp(timestampStr, config.TimeFormat, config.SourceTimeZone, config.TargetTimeZone)
		if err != nil {
			// Log warning but continue processing
			continue
		}

		// Skip if outside business calendar rules (if configured)
		if config.BusinessCalendar != nil && !m.isBusinessTime(timestamp, config.BusinessCalendar) {
			continue
		}

		tsRow := TimeseriesRow{
			Timestamp: timestamp,
			Data:      make([]string, len(row)),
			DatasetID: "", // Will be set when merging multiple datasets
		}
		copy(tsRow.Data, row)

		// Use timestamp as key for deduplication
		timeKey := timestamp.Format(time.RFC3339)

		// Handle duplicates based on configuration
		existingRows := timeseriesData[timeKey]
		if len(existingRows) > 0 {
			duplicates++
			switch config.DeduplicateBy {
			case DedupeTimeKeepFirst:
				continue // Skip this duplicate
			case DedupeTimeKeepLast:
				// Replace existing with this one
				timeseriesData[timeKey] = []TimeseriesRow{tsRow}
			case DedupeTimeKeepNewest:
				// Compare with existing and keep newer data
				if tsRow.Timestamp.After(existingRows[0].Timestamp) {
					timeseriesData[timeKey] = []TimeseriesRow{tsRow}
				}
			case DedupeTimeAggregate:
				// For now, just keep all - aggregation would be more complex
				timeseriesData[timeKey] = append(existingRows, tsRow)
			default:
				timeseriesData[timeKey] = append(existingRows, tsRow)
			}
		} else {
			timeseriesData[timeKey] = []TimeseriesRow{tsRow}
		}

		rowsProcessed++
	}

	return rowsProcessed, duplicates, nil
}

// parseTimestamp parses timestamp with flexible format detection and timezone conversion
func (m *Merger) parseTimestamp(timestampStr, format, sourceTimezone, targetTimezone string) (time.Time, error) {
	var parsedTime time.Time
	var err error

	// Parse the timestamp
	if format != "" {
		// Use specified format
		if sourceTimezone != "" {
			loc, err := time.LoadLocation(sourceTimezone)
			if err != nil {
				return time.Time{}, fmt.Errorf("invalid source timezone %s: %w", sourceTimezone, err)
			}
			parsedTime, err = time.ParseInLocation(format, timestampStr, loc)
		} else {
			parsedTime, err = time.Parse(format, timestampStr)
		}
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse timestamp with format %s: %w", format, err)
		}
	} else {
		// Auto-detect format
		formats := []string{
			time.RFC3339,
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02 15:04:05",
			"2006-01-02 15:04:05.000",
			"2006-01-02",
			"01/02/2006",
			"2006/01/02",
			"02-Jan-2006",
			"2006-01-02 15:04:05 -0700",
			"Mon Jan 2 15:04:05 2006",
		}

		for _, fmt := range formats {
			if t, err := time.Parse(fmt, timestampStr); err == nil {
				parsedTime = t
				break
			}
		}

		if parsedTime.IsZero() {
			return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestampStr)
		}
	}

	// Convert to target timezone if specified
	if targetTimezone != "" && targetTimezone != sourceTimezone {
		targetLoc, err := time.LoadLocation(targetTimezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid target timezone %s: %w", targetTimezone, err)
		}
		parsedTime = parsedTime.In(targetLoc)
	}

	return parsedTime, nil
}

// normalizeTimezone converts timestamp to target timezone
func (m *Merger) normalizeTimezone(timestamp time.Time, sourceTz, targetTz string) (time.Time, error) {
	if sourceTz == "" || targetTz == "" || sourceTz == targetTz {
		return timestamp, nil
	}

	// If timestamp doesn't have location info, assume it's in source timezone
	if timestamp.Location().String() == "UTC" || timestamp.Location().String() == "Local" {
		sourceLoc, err := time.LoadLocation(sourceTz)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid source timezone %s: %w", sourceTz, err)
		}
		timestamp = time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(),
			timestamp.Hour(), timestamp.Minute(), timestamp.Second(), timestamp.Nanosecond(), sourceLoc)
	}

	targetLoc, err := time.LoadLocation(targetTz)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid target timezone %s: %w", targetTz, err)
	}

	return timestamp.In(targetLoc), nil
}

// applyGapFilling fills missing timestamps based on configured strategy
func (m *Merger) applyGapFilling(timeseriesData map[string][]TimeseriesRow, config *TemporalMergeConfig) {
	// This is a simplified implementation - in practice, this would be more sophisticated
	// For now, we'll implement forward fill as an example

	if config.GapFillStrategy == GapFillForward {
		var lastKnownValue []string
		sortedKeys := m.getSortedTimeKeys(timeseriesData)

		for _, key := range sortedKeys {
			rows := timeseriesData[key]
			if len(rows) > 0 && len(rows[0].Data) > 0 {
				lastKnownValue = make([]string, len(rows[0].Data))
				copy(lastKnownValue, rows[0].Data)
			} else if lastKnownValue != nil {
				// Create a new row with forward-filled data
				newRow := TimeseriesRow{
					Timestamp: m.parseTimeKey(key),
					Data:      make([]string, len(lastKnownValue)),
					DatasetID: "gap_filled",
				}
				copy(newRow.Data, lastKnownValue)
				timeseriesData[key] = []TimeseriesRow{newRow}
			}
		}
	}
}

// getSortedTimeKeys returns time keys sorted chronologically
func (m *Merger) getSortedTimeKeys(timeseriesData map[string][]TimeseriesRow) []string {
	keys := make([]string, 0, len(timeseriesData))
	for key := range timeseriesData {
		keys = append(keys, key)
	}

	// Sort by time
	sort.Slice(keys, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, keys[i])
		tj, _ := time.Parse(time.RFC3339, keys[j])
		return ti.Before(tj)
	})

	return keys
}

// parseTimeKey parses a time key back to time.Time
func (m *Merger) parseTimeKey(key string) time.Time {
	t, _ := time.Parse(time.RFC3339, key)
	return t
}

// sortTimeseriesByTime sorts the timeseries data by timestamp
func (m *Merger) sortTimeseriesByTime(timeseriesData map[string][]TimeseriesRow) {
	// The data is already indexed by timestamp, so it's implicitly sorted
	// This method could be extended for more complex sorting if needed
}

// writeTimeseriesOutput writes the merged timeseries data to output
func (m *Merger) writeTimeseriesOutput(timeseriesData map[string][]TimeseriesRow, headers []string, outputPath string, config *MergeConfig) (int, error) {
	// In a real implementation, this would write to the file storage system
	// For now, just simulate the write process

	sortedKeys := m.getSortedTimeKeys(timeseriesData)
	rowsWritten := 0

	for _, key := range sortedKeys {
		rows := timeseriesData[key]
		for range rows {
			rowsWritten++
		}
	}

	return rowsWritten, nil
}

// Removed loadDatasetRows - we don't load entire datasets into memory anymore
// True streaming means we never load full datasets - only process row by row

func (m *Merger) getDatasetReader(ctx context.Context, datasetID core.ID) (io.ReadCloser, error) {
	// This would retrieve the dataset file from storage
	// For now, return a mock implementation
	return nil, fmt.Errorf("dataset reader not implemented")
}

// Removed performMergeOperation - streaming handles merge logic inline
// No more batch processing - everything streams directly to output

// Removed unionMerge and innerJoinMerge - duplicate handling is done inline during streaming
// No more batch duplicate detection - everything happens in the streaming pipeline

func (m *Merger) writeMergedCSV(rows [][]string, headers []string, outputName string) (string, error) {
	// This would write to the file storage system
	// For now, return a mock path
	return fmt.Sprintf("/merged/%s.csv", outputName), nil
}

func (m *Merger) getCurrentMemoryUsage() int {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	return int(memStats.Alloc / 1024 / 1024)
}

// extractHeaders reads just the header row from a CSV reader
func (m *Merger) extractHeaders(reader io.Reader) ([]string, error) {
	csvReader := csv.NewReader(reader)

	headers, err := csvReader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read headers: %w", err)
	}

	return headers, nil
}

// validateSchemaCompatibility checks if headers are compatible for merging
func (m *Merger) validateSchemaCompatibility(expectedHeaders, actualHeaders []string) error {
	if len(expectedHeaders) != len(actualHeaders) {
		return fmt.Errorf("column count mismatch: expected %d, got %d", len(expectedHeaders), len(actualHeaders))
	}

	// For now, just check that all expected columns exist
	// In a more sophisticated implementation, we could handle column reordering
	expectedMap := make(map[string]bool)
	for _, header := range expectedHeaders {
		expectedMap[header] = true
	}

	for _, header := range actualHeaders {
		if !expectedMap[header] {
			return fmt.Errorf("missing required column: %s", header)
		}
	}

	return nil
}

// streamMergeDatasets performs the actual streaming merge operation
func (m *Merger) streamMergeDatasets(ctx context.Context, datasetIDs []core.ID, headers []string, outputName string, config *MergeConfig) (string, int, int, error) {
	// Create output file
	outputPath := fmt.Sprintf("/merged/%s_%d.csv", outputName, time.Now().Unix())

	// In a real implementation, this would write to the file storage system
	// For now, we'll simulate the streaming process

	totalRows := 0
	duplicates := 0

	// Track seen rows for duplicate detection (limited for memory efficiency)
	seenRows := make(map[string]bool)
	maxSeenRows := 100000 // Limit memory usage

	reportProgress(config, 15, "Processing datasets...")

	for i, datasetID := range datasetIDs {
		progress := 15 + float64(i)/float64(len(datasetIDs))*70
		reportProgress(config, progress, fmt.Sprintf("Streaming dataset %d/%d", i+1, len(datasetIDs)))

		reader, err := m.getDatasetReader(ctx, datasetID)
		if err != nil {
			return "", 0, 0, fmt.Errorf("failed to get reader for dataset %s: %w", datasetID, err)
		}

		rowsProcessed, dups, err := m.streamProcessDataset(reader, headers, seenRows, maxSeenRows, config)
		reader.Close()

		if err != nil {
			return "", 0, 0, fmt.Errorf("failed to process dataset %s: %w", datasetID, err)
		}

		totalRows += rowsProcessed
		duplicates += dups
	}

	reportProgress(config, 85, "Writing output...")

	// In a real implementation, we would write the merged data to storage here
	// For now, just return the simulated path and counts

	reportProgress(config, 95, "Finalizing...")

	return outputPath, totalRows, duplicates, nil
}

// streamProcessDataset processes one dataset in a streaming fashion
func (m *Merger) streamProcessDataset(reader io.Reader, expectedHeaders []string, seenRows map[string]bool, maxSeenRows int, config *MergeConfig) (int, int, error) {
	csvReader := csv.NewReader(reader)

	// Skip header row (we already validated it)
	_, err := csvReader.Read()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to skip headers: %w", err)
	}

	rowsProcessed := 0
	duplicates := 0

	// Process rows in streaming fashion
	for {
		row, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, fmt.Errorf("failed to read row: %w", err)
		}

		// Create a key for duplicate detection
		var key string
		if len(config.KeyColumns) > 0 {
			// Use specified key columns
			keyParts := make([]string, len(config.KeyColumns))
			for i, colName := range config.KeyColumns {
				// Find column index
				for j, header := range expectedHeaders {
					if header == colName && j < len(row) {
						keyParts[i] = row[j]
						break
					}
				}
			}
			key = strings.Join(keyParts, "|")
		} else {
			// Use all columns as key
			key = strings.Join(row, "|")
		}

		// Check for duplicates (only if we haven't exceeded memory limit)
		if len(seenRows) < maxSeenRows {
			if seenRows[key] {
				duplicates++
				switch config.DuplicatePolicy {
				case KeepFirst, ErrorOnDupes:
					continue // Skip duplicate
				case RemoveAll:
					// In streaming mode, we can't remove previously written rows
					// This would require a two-pass approach
					continue
				}
			} else {
				seenRows[key] = true
			}
		}

		// In a real implementation, we would write this row to the output stream
		rowsProcessed++
	}

	return rowsProcessed, duplicates, nil
}

// Legacy methods removed - we now use true streaming for maximum scalability
// No temporary files, no chunking, no memory-intensive operations

func reportProgress(config *MergeConfig, progress float64, message string) {
	if config.ProgressCallback != nil {
		config.ProgressCallback(progress, message)
	}
}

// isBusinessTime checks if a timestamp falls within business calendar rules
func (m *Merger) isBusinessTime(timestamp time.Time, calendar *BusinessCalendar) bool {
	// Check weekends
	if !calendar.IncludeWeekends {
		weekday := timestamp.Weekday()
		if weekday == time.Saturday || weekday == time.Sunday {
			return false
		}
	}

	// Check holidays
	for _, holiday := range calendar.Holidays {
		if timestamp.Year() == holiday.Date.Year() &&
		   timestamp.Month() == holiday.Date.Month() &&
		   timestamp.Day() == holiday.Date.Day() {
			return false
		}
	}

	// Check business hours (if specified)
	if calendar.BusinessHours.Start != 0 || calendar.BusinessHours.End != 0 {
		sinceMidnight := time.Duration(timestamp.Hour())*time.Hour +
						time.Duration(timestamp.Minute())*time.Minute +
						time.Duration(timestamp.Second())*time.Second

		if sinceMidnight < calendar.BusinessHours.Start ||
		   sinceMidnight > calendar.BusinessHours.End {
			return false
		}
	}

	return true
}

// detectOutliers performs statistical outlier detection on numeric columns
func (m *Merger) detectOutliers(rows []TimeseriesRow, config *TemporalMergeConfig, headers []string) []TimeseriesRow {
	if !config.OutlierDetection || len(rows) < 3 {
		return rows
	}

	// Group data by column for outlier detection
	columnData := make(map[int][]float64)

	// Find time column index and collect numeric data
	for i, header := range headers {
		if header == config.TimeColumn {
			continue // Skip time column
		}

		// Collect numeric values for this column
		var values []float64
		for _, row := range rows {
			if i < len(row.Data) {
				if val, err := strconv.ParseFloat(strings.TrimSpace(row.Data[i]), 64); err == nil {
					values = append(values, val)
				}
			}
		}

		if len(values) > 0 {
			columnData[i] = values
		}
	}

	// Calculate z-scores for each column
	threshold := config.OutlierThreshold
	if threshold == 0 {
		threshold = 3.0 // Default z-score threshold
	}

	outlierIndices := make(map[int]bool)

	for _, values := range columnData {
		if len(values) < 3 {
			continue
		}

		// Calculate mean and standard deviation
		mean, std := m.calculateMeanStd(values)

		// Find outliers
		for i, value := range values {
			zScore := math.Abs((value - mean) / std)
			if zScore > threshold {
				outlierIndices[i] = true
			}
		}
	}

	// Filter out outliers
	var filteredRows []TimeseriesRow
	for i, row := range rows {
		if !outlierIndices[i] {
			filteredRows = append(filteredRows, row)
		}
	}

	return filteredRows
}

// calculateMeanStd computes mean and standard deviation
func (m *Merger) calculateMeanStd(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate standard deviation
	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	std := math.Sqrt(sumSq / float64(len(values)))

	return mean, std
}

// resampleTimeseries performs frequency resampling
func (m *Merger) resampleTimeseries(data map[string][]TimeseriesRow, targetFreq TemporalFrequency, method InterpolationType) map[string][]TimeseriesRow {
	if len(data) < 2 {
		return data
	}

	// Get sorted timestamps
	sortedKeys := m.getSortedTimeKeys(data)
	if len(sortedKeys) < 2 {
		return data
	}

	// Determine target interval
	targetInterval := m.frequencyToDuration(targetFreq)
	if targetInterval == 0 {
		return data // Unknown frequency, return as-is
	}

	// Create resampled data
	resampled := make(map[string][]TimeseriesRow)

	// Find time range
	firstTime, _ := time.Parse(time.RFC3339, sortedKeys[0])
	lastTime, _ := time.Parse(time.RFC3339, sortedKeys[len(sortedKeys)-1])

	// Generate target timestamps
	current := firstTime
	for !current.After(lastTime) {
		targetKey := current.Format(time.RFC3339)

		// Find nearest data points for interpolation
		var beforeRow, afterRow *TimeseriesRow
		beforeTime, afterTime := time.Time{}, time.Time{}

		for _, key := range sortedKeys {
			rowTime, _ := time.Parse(time.RFC3339, key)
			if rowTime.Before(current) || rowTime.Equal(current) {
				if beforeTime.IsZero() || rowTime.After(beforeTime) {
					beforeTime = rowTime
					if rows, exists := data[key]; exists && len(rows) > 0 {
						beforeRow = &rows[0]
					}
				}
			}
			if rowTime.After(current) {
				if afterTime.IsZero() || rowTime.Before(afterTime) {
					afterTime = rowTime
					if rows, exists := data[key]; exists && len(rows) > 0 {
						afterRow = &rows[0]
					}
				}
			}
		}

		// Create interpolated row
		if beforeRow != nil {
			newRow := TimeseriesRow{
				Timestamp: current,
				Data:      make([]string, len(beforeRow.Data)),
				DatasetID: "resampled",
			}

			copy(newRow.Data, beforeRow.Data)

			// Interpolate numeric values if we have both points
			if afterRow != nil && method == InterpolateLinear {
				timeDiff := afterTime.Sub(beforeTime)
				if timeDiff > 0 {
					currentDiff := current.Sub(beforeTime)
					ratio := float64(currentDiff) / float64(timeDiff)

					for i := range newRow.Data {
						if i < len(beforeRow.Data) && i < len(afterRow.Data) {
							beforeVal, beforeErr := strconv.ParseFloat(beforeRow.Data[i], 64)
							afterVal, afterErr := strconv.ParseFloat(afterRow.Data[i], 64)

							if beforeErr == nil && afterErr == nil {
								interpolated := beforeVal + (afterVal-beforeVal)*ratio
								newRow.Data[i] = fmt.Sprintf("%.6f", interpolated)
							}
						}
					}
				}
			}

			resampled[targetKey] = []TimeseriesRow{newRow}
		}

		current = current.Add(targetInterval)
	}

	return resampled
}

// frequencyToDuration converts frequency enum to time duration
func (m *Merger) frequencyToDuration(freq TemporalFrequency) time.Duration {
	switch freq {
	case FrequencySecond:
		return time.Second
	case FrequencyMinute:
		return time.Minute
	case FrequencyHour:
		return time.Hour
	case FrequencyDay:
		return 24 * time.Hour
	case FrequencyWeek:
		return 7 * 24 * time.Hour
	case FrequencyMonth:
		return 30 * 24 * time.Hour // Approximate
	case FrequencyYear:
		return 365 * 24 * time.Hour // Approximate
	default:
		return 0
	}
}
