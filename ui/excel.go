package ui

import (
	"fmt"
	"log"
	"os"
	"time"

	"gohypo/adapters/excel"
)

// getExcelData extracts data from Excel/CSV file if available
func (s *Server) getExcelData() (*excel.ExcelData, error) {
	return s.getExcelDataFromFile(s.currentDatasetFile)
}

// getExcelDataFromFile extracts data from a specific Excel/CSV file
func (s *Server) getExcelDataFromFile(excelFile string) (*excel.ExcelData, error) {
	if excelFile == "" {
		return nil, fmt.Errorf("no Excel file specified")
	}

	if _, err := os.Stat(excelFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("Excel file not found: %s", excelFile)
	}

	log.Printf("[getExcelDataFromFile] Reading Excel file: %s", excelFile)
	reader := excel.NewDataReader(excelFile)
	data, err := reader.ReadData()
	if err != nil {
		log.Printf("[getExcelDataFromFile] FAILED - Error reading Excel file %s: %v", excelFile, err)
		return nil, fmt.Errorf("failed to read Excel file: %w", err)
	}

	log.Printf("[getExcelDataFromFile] Successfully loaded Excel data - Fields: %d, Rows: %d", len(data.Headers), len(data.Rows))
	return data, nil
}

// loadExcelData loads and caches Excel data
func (s *Server) loadExcelData() error {
	log.Printf("[loadExcelData] Starting Excel data loading and caching")
	s.excelCacheMutex.Lock()
	defer s.excelCacheMutex.Unlock()

	data, err := s.getExcelData()
	if err != nil {
		log.Printf("[loadExcelData] FAILED - Excel data loading failed: %v", err)
		return err
	}

	s.excelDataCache = data
	s.excelColumnTypes = make(map[string]string)
	for _, header := range data.Headers {
		s.excelColumnTypes[header] = "numeric"
	}

	s.excelCacheLoaded = true
	s.excelCacheTimestamp = time.Now()

	log.Printf("[loadExcelData] Excel data cached successfully")
	return nil
}

// getCachedExcelData returns cached Excel data or loads it if not available
func (s *Server) getCachedExcelData() (*excel.ExcelData, error) {
	s.excelCacheMutex.RLock()
	if s.excelCacheLoaded && time.Since(s.excelCacheTimestamp) < 5*time.Minute {
		data := s.excelDataCache
		s.excelCacheMutex.RUnlock()
		return data, nil
	}
	s.excelCacheMutex.RUnlock()

	if err := s.loadExcelData(); err != nil {
		return nil, err
	}

	s.excelCacheMutex.RLock()
	defer s.excelCacheMutex.RUnlock()
	return s.excelDataCache, nil
}


