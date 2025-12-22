package ui

import (
	"fmt"
	"log"

	"gohypo/adapters/excel"
)

// getExcelData extracts data from Excel/CSV file if available - STUBBED
func (s *Server) getExcelData() (*excel.ExcelData, error) {
	log.Printf("[getExcelData] Excel data extraction is now handled by the dataset processor")
	return nil, fmt.Errorf("Excel data extraction moved to dataset processor")
}

// getExcelDataFromFile extracts data from a specific Excel/CSV file - STUBBED
func (s *Server) getExcelDataFromFile(excelFile string) (*excel.ExcelData, error) {
	log.Printf("[getExcelDataFromFile] Excel file processing is now handled by the dataset processor")
	return nil, fmt.Errorf("Excel file processing moved to dataset processor")
}

// loadExcelData loads and caches Excel data - STUBBED
func (s *Server) loadExcelData() error {
	log.Printf("[loadExcelData] Excel data loading is now handled by the dataset processor")
	return nil
}

// getCachedExcelData returns cached Excel data - STUBBED
func (s *Server) getCachedExcelData() (*excel.ExcelData, error) {
	log.Printf("[getCachedExcelData] Excel data caching is no longer used")
	return nil, fmt.Errorf("Excel data caching is no longer supported")
}
