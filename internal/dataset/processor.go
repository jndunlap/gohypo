// Package dataset provides cloud-ready dataset processing with AI-powered analysis.
//
// Cloud Transition Features:
// - Abstracted FileStorage interface for easy S3/Cloud Storage migration
// - Configurable storage paths and limits via StorageConfig
// - Memory-efficient processing with chunked file operations
// - Automatic temporary file cleanup
// - Streaming support for large files
// - Environment-based configuration for different deployments
//
// To migrate to cloud storage:
//  1. Implement S3FileStorage or similar cloud storage adapter
//  2. Update dependency injection in server.go:
//     fileStorage := s3.NewS3FileStorage(config)
//  3. Set environment variables:
//     DATASET_STORAGE_TYPE=s3
//     DATASET_STORAGE_PATH=bucket-name/datasets
//     AWS_REGION=us-east-1
//     AWS_ACCESS_KEY_ID=...
//     AWS_SECRET_ACCESS_KEY=...
//  4. No code changes needed in the processing logic
package dataset

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gohypo/adapters/excel"
	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/internal/api"
	"gohypo/ports"

	"github.com/jmoiron/sqlx"
)

// Processor handles dataset file processing with AI analysis
type Processor struct {
	forensicScout      *ai.ForensicScout
	repository         ports.DatasetRepository
	workspaceRepo      ports.WorkspaceRepository
	fileStorage        FileStorage
	sseHub             *api.SSEHub
	config             *StorageConfig
	Merger             *Merger
	RelationshipEngine *RelationshipDiscoveryEngine
}

// FileStorage defines the interface for file storage operations
type FileStorage interface {
	// Core operations
	Store(ctx context.Context, file multipart.File, filename string) (string, error)
	GetReader(ctx context.Context, filePath string) (io.ReadCloser, error)
	Delete(ctx context.Context, filePath string) error
	GetFileSize(filePath string) (int64, error)
	Exists(ctx context.Context, filePath string) (bool, error)

	// Utility operations
	GetSignedURL(ctx context.Context, filePath string, expiry time.Duration) (string, error)
	GetPublicURL(filePath string) string
}

// StorageConfig holds configuration for file storage
type StorageConfig struct {
	BasePath      string        // Base directory for local storage (empty for cloud)
	MaxFileSize   int64         // Maximum file size in bytes
	MaxMemoryMB   int           // Maximum memory usage for processing in MB
	TempDir       string        // Temporary directory for processing
	AllowedTypes  []string      // Allowed MIME types
	ChunkSize     int           // Chunk size for streaming (default 1MB)
	EnableCleanup bool          // Auto-cleanup temporary files
	CleanupAfter  time.Duration // How long to keep temp files
}

// DefaultStorageConfig returns sensible defaults
func DefaultStorageConfig() *StorageConfig {
	return &StorageConfig{
		BasePath:      "uploads/datasets",
		MaxFileSize:   50 * 1024 * 1024, // 50MB
		MaxMemoryMB:   512,              // 512MB
		TempDir:       os.TempDir(),
		AllowedTypes:  []string{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "application/vnd.ms-excel", "text/csv"},
		ChunkSize:     1024 * 1024, // 1MB
		EnableCleanup: true,
		CleanupAfter:  time.Hour,
	}
}

// NewProcessor creates a new dataset processor
func NewProcessor(forensicScout *ai.ForensicScout, repository ports.DatasetRepository, workspaceRepo ports.WorkspaceRepository, fileStorage FileStorage, sseHub *api.SSEHub, db *sqlx.DB) *Processor {
	return NewProcessorWithConfig(forensicScout, repository, workspaceRepo, fileStorage, sseHub, db, DefaultStorageConfig())
}

// NewProcessorWithConfig creates a new dataset processor with custom configuration
func NewProcessorWithConfig(forensicScout *ai.ForensicScout, repository ports.DatasetRepository, workspaceRepo ports.WorkspaceRepository, fileStorage FileStorage, sseHub *api.SSEHub, db *sqlx.DB, config *StorageConfig) *Processor {
	if config == nil {
		config = DefaultStorageConfig()
	}

	mergeConfig := &MergeConfig{
		Strategy:       HybridMerge,
		MaxMemoryMB:    config.MaxMemoryMB,
		ChunkSize:      10000,
		TempDir:        config.TempDir,
		ValidateSchema: true,
	}

	return &Processor{
		forensicScout:      forensicScout,
		repository:         repository,
		workspaceRepo:      workspaceRepo,
		fileStorage:        fileStorage,
		sseHub:             sseHub,
		config:             config,
		Merger:             NewMerger(db, fileStorage, mergeConfig),
		RelationshipEngine: NewRelationshipDiscoveryEngine(forensicScout, repository, workspaceRepo, NewMerger(db, fileStorage, mergeConfig), db),
	}
}

// ProcessUpload processes an uploaded dataset file
func (p *Processor) ProcessUpload(ctx context.Context, upload *dataset.DatasetUpload) (core.ID, error) {
	log.Printf("[DatasetProcessor] Starting processing for file: %s", upload.Filename)

	// Comprehensive validation
	if err := p.validateUpload(upload); err != nil {
		return "", fmt.Errorf("upload validation failed: %w", err)
	}

	// Get file size - ensure we always have a valid size
	fileSize, err := p.getFileSize(upload.File)
	if err != nil {
		log.Printf("[DatasetProcessor] Warning: could not determine file size: %v", err)
		fileSize = 1 // Default minimum size if we can't determine
	}
	if fileSize <= 0 {
		fileSize = 1 // Ensure positive file size
	}

	// Quick parse to count rows and get basic metadata
	p.broadcastProgress("", "upload_progress", 20, "Analyzing file structure...")
	recordCount, fieldCount, err := p.quickCountRowsAndFields(upload.File, upload.MimeType)
	if err != nil {
		log.Printf("[DatasetProcessor] Warning: could not count rows: %v", err)
		recordCount = 0
		fieldCount = 0
	}

	// Create initial dataset record with row count
	ds := dataset.NewDataset(upload.UserID, upload.Filename)
	ds.WorkspaceID = upload.WorkspaceID
	ds.MimeType = upload.MimeType
	if ds.MimeType == "" {
		// Fallback mime type based on file extension
		if strings.HasSuffix(strings.ToLower(upload.Filename), ".csv") {
			ds.MimeType = "text/csv"
		} else if strings.HasSuffix(strings.ToLower(upload.Filename), ".xlsx") {
			ds.MimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
		} else if strings.HasSuffix(strings.ToLower(upload.Filename), ".xls") {
			ds.MimeType = "application/vnd.ms-excel"
		} else {
			ds.MimeType = "application/octet-stream"
		}
	}
	ds.FileSize = fileSize
	ds.RecordCount = recordCount
	ds.FieldCount = fieldCount
	ds.Status = dataset.StatusProcessing

	// Store in database initially
	if err := p.repository.Create(ctx, ds); err != nil {
		p.broadcastProgress(ds.ID, "upload_failed", 0, fmt.Sprintf("Failed to initialize dataset: %v", err))
		return "", fmt.Errorf("failed to create initial dataset record: %w", err)
	}

	// Process asynchronously to avoid blocking the API
	go func() {
		backgroundCtx := context.Background()
		if err := p.processInBackground(backgroundCtx, ds.ID, upload); err != nil {
			log.Printf("[DatasetProcessor] ❌ Background processing FAILED for dataset %s: %v", ds.ID, err)
			// Update status to failed
			p.repository.UpdateStatus(backgroundCtx, ds.ID, dataset.StatusFailed, err.Error())
		} else {
			log.Printf("[DatasetProcessor] ✅ Background processing completed successfully for dataset %s", ds.ID)
		}
	}()

	return ds.ID, nil
}

// processInBackground handles the actual file processing
func (p *Processor) processInBackground(ctx context.Context, datasetID core.ID, upload *dataset.DatasetUpload) error {
	log.Printf("[DatasetProcessor] 🔄 Background processing started for dataset: %s", datasetID)

	// Send initial progress update
	p.broadcastProgress(datasetID, "upload_started", 0, "Upload processing started")

	// Get initial file size estimate
	fileSize := int64(0)
	if multipartFile, ok := upload.File.(multipart.File); ok {
		if seeker, ok := multipartFile.(io.Seeker); ok {
			size, err := seeker.Seek(0, io.SeekEnd)
			if err == nil {
				fileSize = size
				// Reset to beginning
				seeker.Seek(0, io.SeekStart)
			}
		}
	}

	// Step 1: Store the file
	p.broadcastProgress(datasetID, "upload_progress", 10, "Storing file...")
	filePath, err := p.fileStorage.Store(ctx, upload.File.(multipart.File), upload.Filename)
	if err != nil {
		p.broadcastProgress(datasetID, "upload_failed", 0, fmt.Sprintf("Failed to store file: %v", err))
		return fmt.Errorf("failed to store file: %w", err)
	}

	// Get actual file size from stored file
	actualFileSize, err := p.fileStorage.GetFileSize(filePath)
	if err == nil && actualFileSize > 0 {
		fileSize = actualFileSize
	} else {
		// If we can't get file size from storage, try to get it from the uploaded file
		// This ensures we always have some file size estimate
		if multipartFile, ok := upload.File.(multipart.File); ok {
			if seeker, ok := multipartFile.(io.Seeker); ok {
				currentPos, _ := seeker.Seek(0, io.SeekCurrent)
				size, err := seeker.Seek(0, io.SeekEnd)
				if err == nil && size > 0 {
					fileSize = size
				}
				seeker.Seek(currentPos, io.SeekStart) // Restore position
			}
		}
		// Ensure fileSize is never 0 or negative
		if fileSize <= 0 {
			fileSize = 1 // Minimum valid file size
		}
	}

	// Step 2: Parse the file to extract metadata
	p.broadcastProgress(datasetID, "upload_progress", 30, "Parsing file and extracting metadata...")
	parsedData, err := p.parseFile(upload.File.(multipart.File), upload.MimeType)
	if err != nil {
		p.broadcastProgress(datasetID, "upload_failed", 0, fmt.Sprintf("Failed to parse file: %v", err))
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// Reset file pointer for re-reading if needed
	if seeker, ok := upload.File.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Step 3: Run Forensic Scout analysis
	p.broadcastProgress(datasetID, "upload_progress", 60, "Analyzing data structure with AI...")
	scoutResult, err := p.runForensicScout(ctx, parsedData.Fields)
	if err != nil {
		log.Printf("[DatasetProcessor] Forensic Scout failed, using fallback: %v", err)
		// Extract field names for fallback
		fieldNames := make([]string, len(parsedData.Fields))
		for i, field := range parsedData.Fields {
			fieldNames[i] = field.Name
		}
		scoutResult = p.forensicScout.GetFallbackResponse(fieldNames)
	}
	p.broadcastProgress(datasetID, "upload_progress", 80, "Generating dataset name and description...")

	// Step 4: Calculate statistics
	stats := p.calculateStatistics(parsedData)

	// Step 5: Generate description
	description := p.generateDescription(scoutResult, stats, parsedData)

	// Step 6: Update dataset record
	updateDataset := &dataset.Dataset{
		ID:               datasetID,
		OriginalFilename: upload.Filename,
		FilePath:         filePath,
		FileSize:         fileSize,
		MimeType:         upload.MimeType,
		DisplayName:      scoutResult.DatasetName,
		Domain:           scoutResult.Domain,
		Description:      description,
		RecordCount:      len(parsedData.Rows),
		FieldCount:       len(parsedData.Fields),
		MissingRate:      stats.OverallMissingRate,
		Status:           dataset.StatusReady,
		Metadata: dataset.DatasetMetadata{
			Fields:     parsedData.Fields,
			SampleRows: parsedData.SampleRows,
			AIAnalysis: dataset.ForensicScoutResult{
				Domain:      scoutResult.Domain,
				DatasetName: scoutResult.DatasetName,
				AnalyzedAt:  time.Now(),
			},
		},
		UpdatedAt: time.Now(),
	}

	if err := p.repository.Update(ctx, updateDataset); err != nil {
		p.broadcastProgress(datasetID, "upload_failed", 0, fmt.Sprintf("Failed to save dataset: %v", err))
		return fmt.Errorf("failed to update dataset: %w", err)
	}

	// Relationship discovery is now triggered manually via UI buttons
	// Removed automatic relationship discovery after upload

	p.broadcastProgress(datasetID, "upload_completed", 100, fmt.Sprintf("Dataset '%s' ready for analysis!", scoutResult.DatasetName))

	log.Printf("[DatasetProcessor] ✅ Successfully processed dataset: %s (%s) with %d fields and %d records",
		scoutResult.DatasetName, datasetID, len(parsedData.Fields), len(parsedData.Rows))
	return nil
}

// ParsedFileData represents the extracted data from a file
type ParsedFileData struct {
	Fields     []dataset.FieldInfo
	Rows       []map[string]interface{}
	SampleRows []map[string]interface{}
}

// parseFile extracts data from various file formats
func (p *Processor) parseFile(file multipart.File, mimeType string) (*ParsedFileData, error) {
	// Determine file type and parse accordingly
	switch {
	case strings.Contains(mimeType, "spreadsheet") || strings.HasSuffix(strings.ToLower(mimeType), "xlsx") || strings.HasSuffix(strings.ToLower(mimeType), "xls"):
		return p.parseExcelFile(file)
	case strings.Contains(mimeType, "csv") || strings.HasSuffix(strings.ToLower(mimeType), "csv"):
		return p.parseCSVFile(file)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", mimeType)
	}
}

// parseExcelFile parses Excel files with cloud-friendly temporary storage
func (p *Processor) parseExcelFile(file multipart.File) (*ParsedFileData, error) {
	// Create temporary file with proper cleanup
	tempFile, err := p.createTempFile(file, "dataset_excel_*.xlsx")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Get the actual temp file path for the excel reader
	tempPath := tempFile.Name()

	// Use existing excel adapter
	reader := excel.NewDataReader(tempPath)
	data, err := reader.ReadData()
	if err != nil {
		return nil, fmt.Errorf("failed to read Excel data: %w", err)
	}

	// Convert to our format
	fields := make([]dataset.FieldInfo, len(data.Headers))
	rows := make([]map[string]interface{}, len(data.Rows))

	// Process headers
	for i, header := range data.Headers {
		sampleStrings := p.getSampleValues(data.Rows, header, 5)
		sampleInterfaces := make([]interface{}, len(sampleStrings))
		for j, s := range sampleStrings {
			sampleInterfaces[j] = s
		}

		fields[i] = dataset.FieldInfo{
			Name:         header,
			DataType:     p.inferDataType(data.Rows, header),
			SampleValues: sampleInterfaces,
		}
	}

	// Process rows - convert RawRowData (map[string]string) to map[string]interface{}
	for i, row := range data.Rows {
		rowMap := make(map[string]interface{})
		for _, header := range data.Headers {
			if val, exists := row[header]; exists {
				rowMap[header] = val
			} else {
				rowMap[header] = nil
			}
		}
		rows[i] = rowMap
	}

	// Extract sample rows efficiently (first 100, or stratified sample for large datasets)
	const maxSampleRows = 100
	sampleRows := p.extractSampleRows(rows, maxSampleRows)

	// Calculate field statistics
	for i := range fields {
		fields[i].MissingCount = p.countMissing(rows, fields[i].Name)
		fields[i].UniqueCount = p.countUnique(rows, fields[i].Name)
		fields[i].Nullable = fields[i].MissingCount > 0
	}

	return &ParsedFileData{
		Fields:     fields,
		Rows:       rows,
		SampleRows: sampleRows,
	}, nil
}

// parseCSVFile parses CSV files with proper field analysis
func (p *Processor) parseCSVFile(file multipart.File) (*ParsedFileData, error) {
	// Reset file position to beginning
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read all records at once (for now - could be optimized for large files)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV data: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	// First row is headers
	headers := records[0]
	dataRows := records[1:]

	// Convert data rows to map format for consistency with Excel parsing
	rows := make([]map[string]interface{}, len(dataRows))
	for i, record := range dataRows {
		rowMap := make(map[string]interface{})
		for j, value := range record {
			if j < len(headers) {
				// Trim whitespace and handle empty strings
				cleanValue := strings.TrimSpace(value)
				if cleanValue == "" {
					rowMap[headers[j]] = nil
				} else {
					rowMap[headers[j]] = cleanValue
				}
			}
		}
		rows[i] = rowMap
	}

	// Create field information
	fields := make([]dataset.FieldInfo, len(headers))
	for i, header := range headers {
		sampleStrings := p.getSampleValuesFromCSV(dataRows, i, 5)
		sampleInterfaces := make([]interface{}, len(sampleStrings))
		for j, s := range sampleStrings {
			sampleInterfaces[j] = s
		}

		fields[i] = dataset.FieldInfo{
			Name:         header,
			DataType:     p.inferDataTypeFromCSV(dataRows, i),
			SampleValues: sampleInterfaces,
		}
	}

	// Extract sample rows efficiently
	const maxSampleRows = 100
	sampleRows := p.extractSampleRows(rows, maxSampleRows)

	// Calculate field statistics
	for i := range fields {
		fields[i].MissingCount = p.countMissing(rows, fields[i].Name)
		fields[i].UniqueCount = p.countUnique(rows, fields[i].Name)
		fields[i].Nullable = fields[i].MissingCount > 0
	}

	return &ParsedFileData{
		Fields:     fields,
		Rows:       rows,
		SampleRows: sampleRows,
	}, nil
}

// runForensicScout analyzes field names using the Forensic Scout
func (p *Processor) runForensicScout(ctx context.Context, fields []dataset.FieldInfo) (*ai.ScoutResponse, error) {
	fieldNames := make([]string, len(fields))
	for i, field := range fields {
		fieldNames[i] = field.Name
	}

	return p.forensicScout.AnalyzeFields(ctx, fieldNames)
}

// getSampleValuesFromCSV extracts sample values from CSV records for a specific column
func (p *Processor) getSampleValuesFromCSV(records [][]string, colIndex int, maxSamples int) []string {
	var samples []string
	for _, record := range records {
		if len(record) > colIndex {
			value := strings.TrimSpace(record[colIndex])
			if value != "" {
				samples = append(samples, value)
				if len(samples) >= maxSamples {
					break
				}
			}
		}
	}
	return samples
}

// inferDataTypeFromCSV infers data type from CSV column values
func (p *Processor) inferDataTypeFromCSV(records [][]string, colIndex int) string {
	if colIndex >= len(records[0]) {
		return "text"
	}

	hasNumbers := false
	hasDates := false
	hasBooleans := false
	totalValues := 0
	emptyValues := 0

	// Sample first 100 rows for type inference
	sampleSize := 100
	if len(records) < sampleSize {
		sampleSize = len(records)
	}

	for i := 0; i < sampleSize; i++ {
		if colIndex >= len(records[i]) {
			emptyValues++
			continue
		}

		value := strings.TrimSpace(records[i][colIndex])
		if value == "" {
			emptyValues++
			continue
		}

		totalValues++

		// Check for boolean values
		lowerValue := strings.ToLower(value)
		if lowerValue == "true" || lowerValue == "false" ||
			lowerValue == "1" || lowerValue == "0" ||
			lowerValue == "yes" || lowerValue == "no" ||
			lowerValue == "y" || lowerValue == "n" {
			hasBooleans = true
		}

		// Check for numbers (integers and floats)
		if _, err := strconv.ParseFloat(value, 64); err == nil {
			hasNumbers = true
		}

		// Check for dates (basic patterns)
		if p.isLikelyDate(value) {
			hasDates = true
		}
	}

	// Determine type based on patterns
	if hasBooleans && !hasNumbers && !hasDates {
		return "boolean"
	}
	if hasDates && !hasNumbers {
		return "date"
	}
	if hasNumbers {
		return "numeric"
	}

	return "text"
}

// quickCountRowsAndFields does a fast count of rows and fields without full parsing
func (p *Processor) quickCountRowsAndFields(file multipart.File, mimeType string) (recordCount, fieldCount int, err error) {
	// Reset file position to beginning
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	switch {
	case strings.Contains(mimeType, "spreadsheet") || strings.HasSuffix(strings.ToLower(mimeType), "xlsx") || strings.HasSuffix(strings.ToLower(mimeType), "xls"):
		return p.quickCountExcel(file)
	case strings.Contains(mimeType, "csv") || strings.HasSuffix(strings.ToLower(mimeType), "csv"):
		return p.quickCountCSV(file)
	default:
		// For unknown types, return 0,0
		return 0, 0, nil
	}
}

// quickCountExcel does a fast row/field count for Excel files
func (p *Processor) quickCountExcel(file multipart.File) (recordCount, fieldCount int, err error) {
	// Create temporary file for Excel reading
	tempFile, err := p.createTempFile(file, "quick_excel_*.xlsx")
	if err != nil {
		return 0, 0, err
	}
	defer tempFile.Close()

	// Use existing excel adapter for quick count
	reader := excel.NewDataReader(tempFile.Name())
	data, err := reader.ReadData()
	if err != nil {
		return 0, 0, err
	}

	// Count data rows (excluding header if present)
	recordCount = len(data.Rows)
	if recordCount > 0 {
		fieldCount = len(data.Headers)
	}

	return recordCount, fieldCount, nil
}

// quickCountCSV does a fast row/field count for CSV files
func (p *Processor) quickCountCSV(file multipart.File) (recordCount, fieldCount int, err error) {
	// Reset file position
	if seeker, ok := file.(io.Seeker); ok {
		seeker.Seek(0, io.SeekStart)
	}

	reader := csv.NewReader(file)

	// Read first row to get field count
	firstRow, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return 0, 0, nil // Empty file
		}
		return 0, 0, err
	}

	fieldCount = len(firstRow)
	recordCount = 1 // Count the header row

	// Count remaining rows efficiently
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
		recordCount++
	}

	// Subtract 1 for header row if we assume first row is headers
	if recordCount > 0 {
		recordCount--
	}

	return recordCount, fieldCount, nil
}

// isLikelyDate checks if a string value looks like a date
func (p *Processor) isLikelyDate(value string) bool {
	// Common date patterns
	datePatterns := []string{
		// YYYY-MM-DD
		`^\d{4}-\d{2}-\d{2}$`,
		// MM/DD/YYYY, DD/MM/YYYY
		`^\d{1,2}/\d{1,2}/\d{4}$`,
		// DD-MM-YYYY, MM-DD-YYYY
		`^\d{1,2}-\d{1,2}-\d{4}$`,
		// YYYY/MM/DD
		`^\d{4}/\d{2}/\d{2}$`,
		// Month DD, YYYY
		`^[A-Za-z]{3,9} \d{1,2}, \d{4}$`,
		// DD Month YYYY
		`^\d{1,2} [A-Za-z]{3,9} \d{4}$`,
	}

	for _, pattern := range datePatterns {
		if matched, _ := regexp.MatchString(pattern, value); matched {
			return true
		}
	}

	return false
}

// calculateStatistics computes overall dataset statistics
func (p *Processor) calculateStatistics(data *ParsedFileData) *DatasetStatistics {
	totalCells := len(data.Rows) * len(data.Fields)
	missingCells := 0

	for _, field := range data.Fields {
		missingCells += field.MissingCount
	}

	overallMissingRate := 0.0
	if totalCells > 0 {
		overallMissingRate = float64(missingCells) / float64(totalCells)
	}

	return &DatasetStatistics{
		OverallMissingRate: overallMissingRate,
		TotalRows:          len(data.Rows),
		TotalFields:        len(data.Fields),
	}
}

// generateDescription creates a human-readable description
func (p *Processor) generateDescription(scout *ai.ScoutResponse, stats *DatasetStatistics, data *ParsedFileData) string {
	return fmt.Sprintf("A %s dataset containing %d records with %d fields. This appears to be %s data.",
		strings.ToLower(scout.Domain),
		stats.TotalRows,
		stats.TotalFields,
		strings.ToLower(scout.DatasetName))
}

// Helper methods for data analysis
func (p *Processor) inferDataType(rows []excel.RawRowData, fieldName string) string {
	// Simple type inference - check first few non-null values
	sampleValues := p.getSampleValues(rows, fieldName, 10)

	hasNumeric := false
	hasText := false

	for _, val := range sampleValues {
		if val == "" {
			continue
		}

		// Check if it's numeric
		if _, err := strconv.ParseFloat(val, 64); err == nil {
			hasNumeric = true
		} else {
			hasText = true
		}
	}

	if hasNumeric && !hasText {
		return "numeric"
	} else if hasText {
		return "text"
	}

	return "unknown"
}

func (p *Processor) getSampleValues(rows []excel.RawRowData, fieldName string, limit int) []string {
	var values []string
	for _, row := range rows {
		if val, exists := row[fieldName]; exists && val != "" {
			values = append(values, val)
			if len(values) >= limit {
				break
			}
		}
	}
	return values
}

func (p *Processor) countMissing(rows []map[string]interface{}, fieldName string) int {
	count := 0
	for _, row := range rows {
		if val, exists := row[fieldName]; !exists || val == nil || val == "" {
			count++
		}
	}
	return count
}

func (p *Processor) countUnique(rows []map[string]interface{}, fieldName string) int {
	seen := make(map[interface{}]bool)
	for _, row := range rows {
		if val, exists := row[fieldName]; exists && val != nil && val != "" {
			seen[val] = true
		}
	}
	return len(seen)
}

// DatasetStatistics holds computed statistics
type DatasetStatistics struct {
	OverallMissingRate float64
	TotalRows          int
	TotalFields        int
}

// broadcastProgress sends progress updates via SSE
func (p *Processor) broadcastProgress(datasetID core.ID, eventType string, progress float64, message string) {
	if p.sseHub == nil {
		return // SSE not available, skip broadcasting
	}

	event := api.UploadProgressEvent{
		SessionID: "upload-session", // Use a generic session for uploads
		EventType: eventType,
		DatasetID: string(datasetID),
		Progress:  progress,
		Message:   message,
		Data: map[string]interface{}{
			"dataset_id": string(datasetID),
		},
		Timestamp: time.Now(),
	}

	p.sseHub.BroadcastUploadProgress(event)
}

// validateUpload performs comprehensive validation of the uploaded file
func (p *Processor) validateUpload(upload *dataset.DatasetUpload) error {
	if upload.File == nil {
		return fmt.Errorf("no file provided")
	}

	if upload.Filename == "" {
		return fmt.Errorf("no filename provided")
	}

	// Check file size
	if multipartFile, ok := upload.File.(multipart.File); ok {
		fileSize, err := p.getFileSize(multipartFile)
		if err == nil && fileSize > p.config.MaxFileSize {
			return fmt.Errorf("file size %d bytes exceeds maximum allowed size %d bytes", fileSize, p.config.MaxFileSize)
		}
	}

	// Validate MIME type
	if !p.isAllowedMimeType(upload.MimeType) {
		return fmt.Errorf("MIME type %s is not allowed", upload.MimeType)
	}

	// Validate file extension matches MIME type
	if err := p.validateFileExtension(upload.Filename, upload.MimeType); err != nil {
		return err
	}

	return nil
}

// isAllowedMimeType checks if the MIME type is in the allowed list
func (p *Processor) isAllowedMimeType(mimeType string) bool {
	for _, allowed := range p.config.AllowedTypes {
		if mimeType == allowed {
			return true
		}
	}
	return false
}

// validateFileExtension ensures file extension matches the MIME type
func (p *Processor) validateFileExtension(filename, mimeType string) error {
	ext := strings.ToLower(filepath.Ext(filename))

	switch mimeType {
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		if ext != ".xlsx" {
			return fmt.Errorf("file extension %s does not match MIME type %s", ext, mimeType)
		}
	case "application/vnd.ms-excel":
		if ext != ".xls" {
			return fmt.Errorf("file extension %s does not match MIME type %s", ext, mimeType)
		}
	case "text/csv":
		if ext != ".csv" {
			return fmt.Errorf("file extension %s does not match MIME type %s", ext, mimeType)
		}
	default:
		// For other types, just check basic extension
		validExts := []string{".xlsx", ".xls", ".csv"}
		for _, validExt := range validExts {
			if ext == validExt {
				return nil
			}
		}
		return fmt.Errorf("unsupported file extension: %s", ext)
	}

	return nil
}

// getFileSize safely gets the file size
func (p *Processor) getFileSize(file multipart.File) (int64, error) {
	if seeker, ok := file.(io.Seeker); ok {
		currentPos, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return 0, err
		}
		defer seeker.Seek(currentPos, io.SeekStart) // Restore position

		size, err := seeker.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
		seeker.Seek(0, io.SeekStart) // Reset to beginning
		return size, nil
	}
	return 0, fmt.Errorf("file does not support seeking")
}

// createTempFile creates a temporary file with proper cleanup
func (p *Processor) createTempFile(src multipart.File, prefix string) (*os.File, error) {
	tempFile, err := os.CreateTemp(p.config.TempDir, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy with chunking to handle large files
	buf := make([]byte, p.config.ChunkSize)
	_, err = io.CopyBuffer(tempFile, src, buf)
	if err != nil {
		tempFile.Close()
		os.Remove(tempFile.Name())
		return nil, fmt.Errorf("failed to copy to temp file: %w", err)
	}

	// Reset temp file to beginning
	tempFile.Seek(0, io.SeekStart)

	// Schedule cleanup if enabled
	if p.config.EnableCleanup {
		go func(filePath string) {
			time.Sleep(p.config.CleanupAfter)
			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				log.Printf("[DatasetProcessor] Failed to cleanup temp file %s: %v", filePath, err)
			}
		}(tempFile.Name())
	}

	return tempFile, nil
}

// extractSampleRows efficiently extracts sample rows for preview
// For small datasets (< 1000 rows): takes first N rows
// For large datasets: uses stratified sampling across the dataset
func (p *Processor) extractSampleRows(rows []map[string]interface{}, maxSamples int) []map[string]interface{} {
	totalRows := len(rows)
	if totalRows == 0 {
		return []map[string]interface{}{}
	}

	if totalRows <= maxSamples {
		// Small dataset - return all rows
		return rows
	}

	if totalRows <= 1000 {
		// Medium dataset - return first maxSamples
		return rows[:maxSamples]
	}

	// Large dataset - use stratified sampling
	sampleRows := make([]map[string]interface{}, 0, maxSamples)

	// Always include first row (headers are often most important)
	sampleRows = append(sampleRows, rows[0])

	// Stratified sampling: take samples evenly distributed across the dataset
	step := float64(totalRows-1) / float64(maxSamples-1) // -1 to account for first row
	for i := 1; i < maxSamples; i++ {
		idx := int(float64(i) * step)
		if idx < totalRows && idx != 0 { // Avoid duplicating first row
			sampleRows = append(sampleRows, rows[idx])
		}
	}

	return sampleRows
}

// GetRelationshipEngine returns the relationship discovery engine
func (p *Processor) GetRelationshipEngine() *RelationshipDiscoveryEngine {
	return p.RelationshipEngine
}
