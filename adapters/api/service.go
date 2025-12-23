package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/internal/dataset"
	"gohypo/ports"
)

// APIIngestionService orchestrates API data ingestion and dataset management
type APIIngestionService struct {
	apiReader         *APIReader
	driftDetector     *SchemaDriftDetector
	datasetProcessor  *dataset.Processor
	datasetRepo       ports.DatasetRepository
	apiDataSourceRepo APIDataSourceRepository
	eventBroadcaster  *SSEEventBroadcaster
}

// APIDataSourceRepository defines interface for API data source persistence
type APIDataSourceRepository interface {
	Create(ctx context.Context, source *APIDataSource) error
	GetByID(ctx context.Context, id core.ID) (*APIDataSource, error)
	Update(ctx context.Context, source *APIDataSource) error
	ListByWorkspace(ctx context.Context, workspaceID core.ID) ([]*APIDataSource, error)
	Delete(ctx context.Context, id core.ID) error
}

// NewAPIIngestionService creates a new API ingestion service
func NewAPIIngestionService(
	datasetProcessor *dataset.Processor,
	datasetRepo ports.DatasetRepository,
	apiDataSourceRepo APIDataSourceRepository,
	eventBroadcaster *SSEEventBroadcaster,
) *APIIngestionService {

	return &APIIngestionService{
		driftDetector:     NewSchemaDriftDetector(DefaultDriftThresholds()),
		datasetProcessor:  datasetProcessor,
		datasetRepo:       datasetRepo,
		apiDataSourceRepo: apiDataSourceRepo,
		eventBroadcaster:  eventBroadcaster,
	}
}

// CreateAPIDataSource creates a new API data source configuration
func (s *APIIngestionService) CreateAPIDataSource(ctx context.Context, source *APIDataSource) error {
	// Validate configuration
	if err := s.validateDataSource(source); err != nil {
		return fmt.Errorf("invalid data source configuration: %w", err)
	}

	// Test the API connection
	testReader := NewAPIReader(source)
	if _, err := testReader.FetchData(ctx); err != nil {
		return fmt.Errorf("API connection test failed: %w", err)
	}

	// Create the data source
	if err := s.apiDataSourceRepo.Create(ctx, source); err != nil {
		return fmt.Errorf("failed to create API data source: %w", err)
	}

	log.Printf("[APIIngestion] Created API data source: %s (%s)", source.Name, source.ID)
	return nil
}

// IngestFromDataSource performs a full ingestion cycle from an API data source
func (s *APIIngestionService) IngestFromDataSource(ctx context.Context, dataSourceID core.ID, sessionID string) (*APIIngestResult, error) {
	startTime := time.Now()

	result := &APIIngestResult{
		DataSourceID: dataSourceID,
		Success:      false,
	}

	// Get the data source configuration
	dataSource, err := s.apiDataSourceRepo.GetByID(ctx, dataSourceID)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to get data source: %v", err)
		return result, fmt.Errorf("failed to get data source: %w", err)
	}

	if !dataSource.Enabled {
		result.Error = "Data source is disabled"
		return result, fmt.Errorf("data source is disabled")
	}

	// Broadcast sync started
	if s.eventBroadcaster != nil {
		s.eventBroadcaster.BroadcastAPISyncStarted(sessionID, dataSource, 0)
	}

	// Create API reader for this data source
	s.apiReader = NewAPIReader(dataSource)

	// Fetch data from API
	log.Printf("[APIIngestion] Fetching data from API: %s", dataSource.Name)

	// Broadcast progress
	if s.eventBroadcaster != nil {
		s.eventBroadcaster.BroadcastAPISyncProgress(sessionID, dataSource, 20, "Connecting to API...", 0, 0)
	}

	apiData, err := s.apiReader.FetchData(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("API fetch failed: %v", err)
		if s.eventBroadcaster != nil {
			s.eventBroadcaster.BroadcastAPISyncFailed(sessionID, dataSource, err, "network", true, nil)
		}
		return result, s.handleIngestionError(ctx, dataSource, err)
	}

	// Broadcast progress
	if s.eventBroadcaster != nil {
		s.eventBroadcaster.BroadcastAPISyncProgress(sessionID, dataSource, 50, "Processing data...", apiData.Metadata.RecordsCount, apiData.Metadata.RateLimitRemaining)
	}

	// Detect schema drift if we have a baseline
	var driftReport *SchemaDriftReport
	if dataSource.SchemaFingerprint != nil {
		log.Printf("[APIIngestion] Detecting schema drift for: %s", dataSource.Name)
		driftReport, err = s.driftDetector.DetectDrift(dataSourceID, apiData.ParsedData, dataSource.SchemaFingerprint)
		if err != nil {
			log.Printf("[APIIngestion] Drift detection failed: %v", err)
			// Continue with ingestion but log the error
		}

		// Handle significant drift
		if driftReport != nil && driftReport.Severity >= DriftSeverityMedium {
			if s.eventBroadcaster != nil {
				s.eventBroadcaster.BroadcastSchemaDriftDetected(sessionID, dataSource, driftReport)
			}
			return s.handleSchemaDrift(ctx, dataSource, apiData, driftReport)
		}
	}

	// Convert API data to dataset format
	datasetUpload, err := s.convertAPIDataToDatasetUpload(dataSource, apiData)
	if err != nil {
		result.Error = fmt.Sprintf("Data conversion failed: %v", err)
		return result, fmt.Errorf("failed to convert API data: %w", err)
	}

	// Create or update dataset
	datasetID, err := s.datasetProcessor.ProcessUpload(ctx, datasetUpload)
	if err != nil {
		result.Error = fmt.Sprintf("Dataset processing failed: %v", err)
		return result, fmt.Errorf("failed to process dataset: %w", err)
	}

	// Update schema fingerprint for future drift detection
	newFingerprint, err := s.apiReader.ComputeSchemaFingerprint(apiData.ParsedData)
	if err == nil {
		dataSource.SchemaFingerprint = newFingerprint
		dataSource.LastSync = time.Now()
		dataSource.Status = "active"
		dataSource.ErrorMessage = ""

		if err := s.apiDataSourceRepo.Update(ctx, dataSource); err != nil {
			log.Printf("[APIIngestion] Failed to update schema fingerprint: %v", err)
		}
	}

	// Success
	result.Success = true
	result.RecordsIngested = len(apiData.ParsedData)
	result.Duration = time.Since(startTime)

	if driftReport != nil {
		result.DriftDetected = driftReport
	}

	// Broadcast completion
	if s.eventBroadcaster != nil {
		s.eventBroadcaster.BroadcastAPISyncCompleted(sessionID, dataSource, result, datasetID, true)
	}

	log.Printf("[APIIngestion] Successfully ingested %d records from API: %s",
		result.RecordsIngested, dataSource.Name)

	return result, nil
}

// handleSchemaDrift manages significant schema drift detection
func (s *APIIngestionService) handleSchemaDrift(
	ctx context.Context,
	dataSource *APIDataSource,
	apiData *APIData,
	driftReport *SchemaDriftReport,
) (*APIIngestResult, error) {

	result := &APIIngestResult{
		DataSourceID:   dataSource.ID,
		Success:        false,
		DriftDetected:  driftReport,
	}

	// Update data source status to indicate drift
	dataSource.Status = "drift_detected"
	dataSource.ErrorMessage = fmt.Sprintf("Schema drift detected: %s", driftReport.Severity.String())

	if err := s.apiDataSourceRepo.Update(ctx, dataSource); err != nil {
		log.Printf("[APIIngestion] Failed to update data source status: %v", err)
	}

	// For high-severity drift, quarantine the data but don't create dataset
	if driftReport.Severity >= DriftSeverityHigh {
		result.Error = fmt.Sprintf("High-severity schema drift detected: %s", driftReport.Recommendations[0])
		log.Printf("[APIIngestion] High-severity drift quarantined for data source: %s", dataSource.Name)
		return result, nil
	}

	// For medium-severity drift, log warning but proceed with ingestion
	log.Printf("[APIIngestion] Medium-severity drift detected for %s: %v",
		dataSource.Name, driftReport.Recommendations)

	// Continue with normal processing despite drift
	return s.proceedWithDrift(ctx, dataSource, apiData, driftReport)
}

// proceedWithDrift continues ingestion despite detected drift
func (s *APIIngestionService) proceedWithDrift(
	ctx context.Context,
	dataSource *APIDataSource,
	apiData *APIData,
	driftReport *SchemaDriftReport,
) (*APIIngestResult, error) {

	// Convert with drift awareness
	datasetUpload, err := s.convertAPIDataToDatasetUploadWithDrift(dataSource, apiData, driftReport)
	if err != nil {
		return &APIIngestResult{
			DataSourceID: dataSource.ID,
			Success:      false,
			Error:        fmt.Sprintf("Drift-aware conversion failed: %v", err),
			DriftDetected: driftReport,
		}, err
	}

	// Process dataset
	datasetID, err := s.datasetProcessor.ProcessUpload(ctx, datasetUpload)
	if err != nil {
		return &APIIngestResult{
			DataSourceID:  dataSource.ID,
			Success:       false,
			Error:         fmt.Sprintf("Dataset processing failed: %v", err),
			DriftDetected: driftReport,
		}, err
	}

	// Update data source with drift information
	dataSource.LastSync = time.Now()
	dataSource.Status = "active_with_drift"
	dataSource.ErrorMessage = fmt.Sprintf("Processed with drift: %s", driftReport.Severity.String())

	if err := s.apiDataSourceRepo.Update(ctx, dataSource); err != nil {
		log.Printf("[APIIngestion] Failed to update data source after drift processing: %v", err)
	}

	// Broadcast drift resolution
	if s.eventBroadcaster != nil {
		s.eventBroadcaster.BroadcastDriftResolved(sessionID, dataSource, "auto_accepted", "Processed with drift-aware transformations", &datasetID)
	}

	return &APIIngestResult{
		DataSourceID:     dataSource.ID,
		Success:          true,
		RecordsIngested:  len(apiData.ParsedData),
		DriftDetected:    driftReport,
	}, nil
}

// convertAPIDataToDatasetUpload converts API data to dataset upload format
func (s *APIIngestionService) convertAPIDataToDatasetUpload(
	dataSource *APIDataSource,
	apiData *APIData,
) (*dataset.DatasetUpload, error) {

	// Convert parsed data to CSV format for compatibility with existing pipeline
	csvData, err := s.convertToCSV(apiData.ParsedData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to CSV: %w", err)
	}

	// Create a virtual file from the CSV data
	virtualFile := &VirtualFile{
		data: csvData,
		name: fmt.Sprintf("%s_%d.csv", dataSource.Name, time.Now().Unix()),
	}

	return &dataset.DatasetUpload{
		UserID:      dataSource.UserID,
		WorkspaceID: dataSource.WorkspaceID,
		Filename:    virtualFile.name,
		File:        virtualFile,
		MimeType:    "text/csv",
	}, nil
}

// convertAPIDataToDatasetUploadWithDrift handles conversion when drift is present
func (s *APIIngestionService) convertAPIDataToDatasetUploadWithDrift(
	dataSource *APIDataSource,
	apiData *APIData,
	driftReport *SchemaDriftReport,
) (*dataset.DatasetUpload, error) {

	// Apply drift-aware transformations
	transformedData := s.applyDriftTransformations(apiData.ParsedData, driftReport)

	// Convert to CSV
	csvData, err := s.convertToCSV(transformedData)
	if err != nil {
		return nil, fmt.Errorf("failed to convert drifted data to CSV: %w", err)
	}

	virtualFile := &VirtualFile{
		data: csvData,
		name: fmt.Sprintf("%s_drift_%d.csv", dataSource.Name, time.Now().Unix()),
	}

	return &dataset.DatasetUpload{
		UserID:      dataSource.UserID,
		WorkspaceID: dataSource.WorkspaceID,
		Filename:    virtualFile.name,
		File:        virtualFile,
		MimeType:    "text/csv",
	}, nil
}

// applyDriftTransformations modifies data to handle detected drift
func (s *APIIngestionService) applyDriftTransformations(
	data []map[string]interface{},
	driftReport *SchemaDriftReport,
) []map[string]interface{} {

	// For now, apply basic transformations for safe type conversions
	transformed := make([]map[string]interface{}, len(data))

	for i, record := range data {
		transformed[i] = make(map[string]interface{})

		for key, value := range record {
			transformed[i][key] = s.transformValueForDrift(value, key, driftReport)
		}
	}

	return transformed
}

// transformValueForDrift applies drift-aware value transformations
func (s *APIIngestionService) transformValueForDrift(
	value interface{},
	fieldName string,
	driftReport *SchemaDriftReport,
) interface{} {

	// Find any type change for this field
	for _, change := range driftReport.Changes {
		if change.FieldName == fieldName && change.ChangeType == ChangeTypeTypeChanged {
			// Apply safe type conversion if possible
			return s.attemptSafeTypeConversion(value, change.OldValue.(string), change.NewValue.(string))
		}
	}

	// No transformation needed
	return value
}

// attemptSafeTypeConversion tries safe type conversions for drift handling
func (s *APIIngestionService) attemptSafeTypeConversion(value interface{}, fromType, toType string) interface{} {
	// Implement safe conversion logic
	// For example: int -> float, string -> numeric (if parseable), etc.
	switch {
	case fromType == "int" && toType == "float":
		if intVal, ok := value.(int); ok {
			return float64(intVal)
		}
	case fromType == "string" && toType == "numeric":
		// Try to parse as number, keep as string if fails
		if strVal, ok := value.(string); ok {
			// Could use strconv.ParseFloat here
			return strVal // For now, keep as string
		}
	}

	return value
}

// convertToCSV converts parsed data to CSV format
func (s *APIIngestionService) convertToCSV(data []map[string]interface{}) ([]byte, error) {
	if len(data) == 0 {
		return []byte{}, nil
	}

	// Get all field names from first record
	var headers []string
	for fieldName := range data[0] {
		headers = append(headers, fieldName)
	}

	// Create CSV content
	var csvLines []string

	// Add header row
	csvLines = append(csvLines, fmt.Sprintf("%s", headers[0]))
	for _, header := range headers[1:] {
		csvLines[len(csvLines)-1] += "," + header
	}

	// Add data rows
	for _, record := range data {
		var values []string
		for _, header := range headers {
			if value, exists := record[header]; exists {
				values = append(values, fmt.Sprintf("%v", value))
			} else {
				values = append(values, "")
			}
		}

		line := fmt.Sprintf("%s", values[0])
		for _, value := range values[1:] {
			line += "," + value
		}
		csvLines = append(csvLines, line)
	}

	result := ""
	for _, line := range csvLines {
		result += line + "\n"
	}

	return []byte(result), nil
}

// handleIngestionError handles various types of ingestion errors
func (s *APIIngestionService) handleIngestionError(ctx context.Context, dataSource *APIDataSource, err error) error {
	// Update data source with error status
	dataSource.Status = "error"
	dataSource.ErrorMessage = err.Error()
	dataSource.LastSync = time.Now()

	if updateErr := s.apiDataSourceRepo.Update(ctx, dataSource); updateErr != nil {
		log.Printf("[APIIngestion] Failed to update data source error status: %v", updateErr)
	}

	return err
}

// validateDataSource validates API data source configuration
func (s *APIIngestionService) validateDataSource(source *APIDataSource) error {
	if source.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	if source.Name == "" {
		return fmt.Errorf("name is required")
	}

	if source.AuthMethod != "none" && source.AuthToken == "" && source.Username == "" {
		return fmt.Errorf("authentication credentials required for method: %s", source.AuthMethod)
	}

	return nil
}

// VirtualFile implements multipart.File for in-memory data
type VirtualFile struct {
	data   []byte
	name   string
	offset int64
}

func (vf *VirtualFile) Read(p []byte) (n int, err error) {
	if vf.offset >= int64(len(vf.data)) {
		return 0, fmt.Errorf("EOF")
	}

	n = copy(p, vf.data[vf.offset:])
	vf.offset += int64(n)
	return n, nil
}

func (vf *VirtualFile) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(vf.data)) {
		return 0, fmt.Errorf("EOF")
	}

	n = copy(p, vf.data[off:])
	return n, nil
}

func (vf *VirtualFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0: // SeekStart
		vf.offset = offset
	case 1: // SeekCurrent
		vf.offset += offset
	case 2: // SeekEnd
		vf.offset = int64(len(vf.data)) + offset
	}

	if vf.offset < 0 {
		vf.offset = 0
	}
	if vf.offset > int64(len(vf.data)) {
		vf.offset = int64(len(vf.data))
	}

	return vf.offset, nil
}

func (vf *VirtualFile) Close() error {
	return nil
}
