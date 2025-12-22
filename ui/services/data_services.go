package services

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gohypo/adapters/excel"
	"gohypo/domain/core"
	"gohypo/domain/greenfield"
	"gohypo/ports"

	"github.com/google/uuid"
)

type DataService struct {
	reader           ports.LedgerReaderPort
	datasetRepo      ports.DatasetRepository

	// Excel cache fields
	excelDataCache      *excel.ExcelData
	excelColumnTypes    map[string]string
	excelCacheMutex     sync.RWMutex
	excelCacheLoaded    bool
	excelCacheTimestamp time.Time
}

func NewDataService(reader ports.LedgerReaderPort, datasetRepo ports.DatasetRepository) *DataService {
	return &DataService{
		reader:           reader,
		datasetRepo:      datasetRepo,
		excelDataCache:   nil,
		excelColumnTypes: make(map[string]string),
	}
}

// GetFieldMetadataByWorkspace returns field metadata for datasets in a specific workspace
func (s *DataService) GetFieldMetadataByWorkspace(workspaceID uuid.UUID) ([]greenfield.FieldMetadata, error) {
	// Query datasets directly for this workspace
	datasets, err := s.datasetRepo.GetByWorkspace(context.Background(), core.ID(workspaceID.String()), 1000, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to get datasets for workspace: %w", err)
	}

	fieldMap := make(map[string]*greenfield.FieldMetadata)

	for _, dataset := range datasets {
		// Extract field metadata from dataset metadata structure
		for _, field := range dataset.Metadata.Fields {
			if field.Name != "" {
				if _, exists := fieldMap[field.Name]; !exists {
					fieldMap[field.Name] = &greenfield.FieldMetadata{
						Name:     field.Name,
						DataType: field.DataType,
					}
				}
			}
		}
	}

	var metadata []greenfield.FieldMetadata
	for _, field := range fieldMap {
		metadata = append(metadata, *field)
	}

	log.Printf("[API] 📊 Workspace %s field collection complete: %d fields from %d datasets",
		workspaceID, len(metadata), len(datasets))

	return metadata, nil
}

func (s *DataService) GetFieldMetadata() ([]greenfield.FieldMetadata, error) {
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

	if excelData, columnTypes, err := s.getExcelFieldMetadata(); err == nil && excelData != nil {
		for _, fieldName := range excelData.Headers {
			// Skip empty or whitespace-only field names (phantom columns)
			fieldName = strings.TrimSpace(fieldName)
			if fieldName == "" {
				log.Printf("[API] ⚠️ Skipping empty field name from Excel headers")
				continue
			}

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

// GetStatisticalArtifactsByWorkspace returns statistical artifacts for datasets in a specific workspace
func (s *DataService) GetStatisticalArtifactsByWorkspace(workspaceID uuid.UUID) ([]map[string]interface{}, error) {
	// For now, delegate to the general method - in future this should filter by workspace
	// TODO: Implement workspace-specific artifact filtering
	return s.GetStatisticalArtifacts()
}

func (s *DataService) GetStatisticalArtifacts() ([]map[string]interface{}, error) {
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

func (s *DataService) getExcelFieldMetadata() (*excel.ExcelData, map[string]string, error) {
	// Check cache first
	s.excelCacheMutex.RLock()
	if s.excelCacheLoaded && s.excelDataCache != nil && s.excelColumnTypes != nil {
		// Check if cache is still fresh (5 minutes)
		if time.Since(s.excelCacheTimestamp) < 5*time.Minute {
			data := s.excelDataCache
			types := s.excelColumnTypes
			s.excelCacheMutex.RUnlock()
			return data, types, nil
		}
	}
	s.excelCacheMutex.RUnlock()

	// Cache miss or expired - read from disk
	excelFile := os.Getenv("EXCEL_FILE")
	if excelFile == "" {
		// No Excel file configured - return nil to indicate no Excel data available
		return nil, nil, nil
	}

	// Read Excel data
	reader := excel.NewDataReader(excelFile)
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

	// Update cache
	s.excelCacheMutex.Lock()
	s.excelDataCache = data
	s.excelColumnTypes = columnTypes
	s.excelCacheLoaded = true
	s.excelCacheTimestamp = time.Now()
	s.excelCacheMutex.Unlock()

	return data, columnTypes, nil
}
