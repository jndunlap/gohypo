package ui

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gohypo/adapters/excel"
	domainBrief "gohypo/domain/stats/brief"

	"github.com/gin-gonic/gin"
)

// extractFieldData extracts data for a specific field from Excel data
func extractFieldData(excelData *excel.ExcelData, fieldName string) []float64 {
	data := make([]float64, 0, len(excelData.Rows))
	for _, row := range excelData.Rows {
		if strVal, exists := row[fieldName]; exists && strVal != "" {
			if val, err := strconv.ParseFloat(strVal, 64); err == nil {
				data = append(data, val)
			}
		}
	}
	return data
}

func (s *Server) loadDatasetInfo(ctx context.Context) error {
	startTime := time.Now()
	log.Printf("[loadDatasetInfo] Starting dataset loading process")

	excelData, err := s.getCachedExcelData()
	if err != nil {
		log.Printf("[loadDatasetInfo] FAILED - Excel data loading failed after %v: %v", time.Since(startTime), err)
		return fmt.Errorf("failed to get Excel data: %w", err)
	}

	log.Printf("[loadDatasetInfo] Excel data loaded successfully - Fields: %d, Rows: %d", len(excelData.Headers), len(excelData.Rows))

	fieldStats := make([]*FieldStats, 0, len(excelData.Headers))
	log.Printf("[loadDatasetInfo] Starting field statistics computation for %d fields", len(excelData.Headers))

	for i, fieldName := range excelData.Headers {
		stat := &FieldStats{
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

		if s.analysisEngine != nil {
			fieldData := extractFieldData(excelData, fieldName)
			if len(fieldData) > 0 {
				req := domainBrief.ComputationRequest{
					ForValidation: true,
					ForHypothesis: true,
				}
				if statBrief, err := s.analysisEngine.Computer().ComputeBrief(fieldData, fieldName, "ui", req); err == nil {
					stat.StatisticalBrief = statBrief
					stat.SampleSize = statBrief.SampleSize
					stat.MissingRate = statBrief.Quality.MissingRatio
					stat.MissingRatePct = fmt.Sprintf("%.1f", statBrief.Quality.MissingRatio*100)

					// Extract summary statistics with validation
					stat.Mean = statBrief.Summary.Mean
					stat.StdDev = statBrief.Summary.StdDev
					// Ensure StdDev is non-negative
					if stat.StdDev < 0 {
						stat.StdDev = 0
					}
					stat.Min = statBrief.Summary.Min
					stat.Max = statBrief.Summary.Max
					// Ensure Min <= Max
					if stat.Min > stat.Max {
						stat.Min, stat.Max = stat.Max, stat.Min
					}
					stat.Median = statBrief.Summary.Median

					// Calculate coefficient of variation (avoid division by zero and extreme values)
					if stat.Mean != 0 && stat.Mean >= 0.001 { // Avoid very small means
						stat.CV = stat.StdDev / stat.Mean
						// Cap extreme CV values for display purposes
						if stat.CV > 10.0 {
							stat.CV = 10.0
						}
					}

					// Set variance to standard deviation squared for compatibility
					if statBrief.Summary.StdDev > 0 {
						stat.Variance = statBrief.Summary.StdDev * statBrief.Summary.StdDev
					}

					// Extract distribution statistics
					stat.Skewness = statBrief.Distribution.Skewness
					stat.Kurtosis = statBrief.Distribution.Kurtosis
					stat.IsNormal = statBrief.Distribution.IsNormal

					// Extract quality statistics with bounds checking
					stat.SparsityRatio = statBrief.Quality.SparsityRatio
					// Ensure sparsity is between 0 and 1
					if stat.SparsityRatio < 0 {
						stat.SparsityRatio = 0
					} else if stat.SparsityRatio > 1 {
						stat.SparsityRatio = 1
					}
					stat.NoiseCoefficient = statBrief.Quality.NoiseCoefficient
					// Ensure noise coefficient is non-negative
					if stat.NoiseCoefficient < 0 {
						stat.NoiseCoefficient = 0
					}
					stat.OutlierCount = statBrief.Quality.OutlierCount
					// Ensure outlier count is non-negative
					if stat.OutlierCount < 0 {
						stat.OutlierCount = 0
					}

					// Set cardinality and unique count from categorical stats if available
					if statBrief.Categorical != nil {
						stat.UniqueCount = statBrief.Categorical.Cardinality
						stat.Cardinality = statBrief.Categorical.Cardinality
						stat.Entropy = statBrief.Categorical.Entropy
						// Cap extreme entropy values for display
						if stat.Entropy > 5.0 {
							stat.Entropy = 5.0
						}
						stat.Mode = statBrief.Categorical.Mode
						stat.ModeFrequency = statBrief.Categorical.ModeFrequency
						if statBrief.Categorical.IsCategorical {
							stat.Type = "categorical"
						}
					} else {
						// For numeric fields, estimate unique count from sample size and quality
						// This is a rough approximation - in practice, we'd compute this properly
						stat.UniqueCount = int(float64(stat.SampleSize) * (1.0 - statBrief.Quality.SparsityRatio))
						if stat.UniqueCount < 0 {
							stat.UniqueCount = 0
						}
						stat.Cardinality = stat.UniqueCount
					}
				}
			}
		}

		fieldStats = append(fieldStats, stat)

		// Log progress every 10 fields
		if (i+1)%10 == 0 || i+1 == len(excelData.Headers) {
			log.Printf("[loadDatasetInfo] Processed %d/%d fields (%d%%)", i+1, len(excelData.Headers), (i+1)*100/len(excelData.Headers))
		}
	}

	// Calculate overall missingness rate
	var totalMissingRate float64
	var totalSampleSize int
	for _, stat := range fieldStats {
		if stat.SampleSize > 0 {
			totalMissingRate += stat.MissingRate * float64(stat.SampleSize)
			totalSampleSize += stat.SampleSize
		}
	}
	var overallMissingness float64
	if totalSampleSize > 0 {
		overallMissingness = totalMissingRate / float64(totalSampleSize)
	}

	// Extract forensic scout context if available
	var scoutData map[string]interface{}
	if s.forensicScout != nil {
		log.Printf("[loadDatasetInfo] Running forensic scout for industry context...")
		if scoutResponse, err := s.forensicScout.ExtractIndustryContext(ctx); err == nil && scoutResponse != nil {
			scoutData = map[string]interface{}{
				"domain":     scoutResponse.Domain,
				"context":    scoutResponse.Context,
				"bottleneck": scoutResponse.Bottleneck,
				"physics":    scoutResponse.Physics,
				"map":        scoutResponse.Map,
			}
			log.Printf("[loadDatasetInfo] Forensic scout completed - Domain: %s", scoutResponse.Domain)
		} else {
			log.Printf("[loadDatasetInfo] Forensic scout failed or not available: %v", err)
		}
	}

	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	s.datasetCache = map[string]interface{}{
		"VariablesTotal":    len(fieldStats),
		"VariablesEligible": len(fieldStats),
		"VariablesRejected": 0,
		"FieldStats":        fieldStats,
		"FieldCount":        len(fieldStats),
		"DatasetInfo": map[string]interface{}{
			"name":               "Dataset",
			"missingnessOverall": overallMissingness,
		},
		"RunStatus":         "READY",
		"RelationshipCount": 0,
		"SignificantCount":  0,
		"StageStatuses": map[string]interface{}{
			"Profile": map[string]interface{}{
				"Status":        "COMPLETE",
				"ArtifactCount": len(fieldStats),
			},
		},
		"ForensicScout": scoutData,
	}

	s.cacheLoaded = true
	s.cacheLastUpdated = time.Now()

	totalDuration := time.Since(startTime)
	log.Printf("[loadDatasetInfo] Dataset loading completed successfully in %v - Total fields: %d", totalDuration, len(fieldStats))

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

	// Validate file type (only allow Excel/CSV files)
	filename := header.Filename
	if !strings.HasSuffix(strings.ToLower(filename), ".xlsx") &&
		!strings.HasSuffix(strings.ToLower(filename), ".xls") &&
		!strings.HasSuffix(strings.ToLower(filename), ".csv") {
		log.Printf("[handleFileUpload] FAILED - Invalid file type: %s", filename)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only Excel (.xlsx, .xls) and CSV (.csv) files are allowed"})
		return
	}

	// Create uploads directory if it doesn't exist
	uploadDir := "uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Printf("[handleFileUpload] FAILED - Could not create uploads directory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	// Generate unique filename to avoid conflicts
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)
	timestamp := time.Now().Format("20060102_150405")
	newFilename := fmt.Sprintf("%s_%s%s", baseName, timestamp, ext)
	filepath := filepath.Join(uploadDir, newFilename)

	// Save the uploaded file
	out, err := os.Create(filepath)
	if err != nil {
		log.Printf("[handleFileUpload] FAILED - Could not create file: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save uploaded file"})
		return
	}
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		log.Printf("[handleFileUpload] FAILED - Could not save file content: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file content"})
		return
	}

	log.Printf("[handleFileUpload] File uploaded successfully: %s", filepath)

	// Update the dataset file and trigger a reload
	go func() {
		log.Printf("[handleFileUpload] Switching to uploaded file: %s", filepath)

		// Update the current dataset file
		s.currentDatasetFile = filepath

		// Clear existing caches
		s.excelCacheMutex.Lock()
		s.excelCacheLoaded = false
		s.excelDataCache = nil
		s.excelCacheMutex.Unlock()

		s.cacheMutex.Lock()
		s.cacheLoaded = false
		s.datasetCache = make(map[string]interface{})
		s.cacheMutex.Unlock()

		// Reload with new file
		ctx := context.Background()
		if err := s.loadDatasetInfo(ctx); err != nil {
			log.Printf("[handleFileUpload] Dataset reload failed: %v", err)
		} else {
			log.Printf("[handleFileUpload] Dataset reloaded successfully with new file")
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"message":  "File uploaded successfully. Dataset is reloading...",
		"filename": newFilename,
		"filepath": filepath,
	})
}
