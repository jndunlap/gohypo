package excel

import (
	"encoding/csv"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gohypo/adapters/datareadiness/coercer"
	"gohypo/domain/datareadiness/ingestion"

	"github.com/xuri/excelize/v2"
)

// DataReader handles reading Excel and CSV files
type DataReader struct {
	filePath string
	fileType string // "xlsx" or "csv"
}

// ExcelReader is an alias for DataReader for backward compatibility
type ExcelReader = DataReader

// NewDataReader creates a new data reader that handles both Excel and CSV files
func NewDataReader(filePath string) *DataReader {
	ext := strings.ToLower(filepath.Ext(filePath))
	fileType := "xlsx"
	if ext == ".csv" {
		fileType = "csv"
	}
	return &DataReader{filePath: filePath, fileType: fileType}
}

// NewExcelReader creates a new Excel reader (deprecated, use NewDataReader)
func NewExcelReader(filePath string) *DataReader {
	return NewDataReader(filePath)
}

// ReadData reads data from Excel or CSV files into structured format
func (r *DataReader) ReadData() (*ExcelData, error) {
	log.Printf("[DataReader] Starting to read %s file: %s", r.fileType, r.filePath)

	// Check if file exists
	if _, err := os.Stat(r.filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s file not found: %s", strings.ToUpper(r.fileType), r.filePath)
	}

	switch r.fileType {
	case "csv":
		return r.readCSVData()
	case "xlsx":
		return r.readExcelData()
	default:
		return nil, fmt.Errorf("unsupported file type: %s", r.fileType)
	}
}

// readExcelData reads Excel data from Sheet1 into structured format
func (r *DataReader) readExcelData() (*ExcelData, error) {
	startTime := time.Now()
	f, err := excelize.OpenFile(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()
	fileOpenTime := time.Since(startTime)
	log.Printf("[DataReader] Excel file opened in %.2fms", float64(fileOpenTime.Nanoseconds())/1e6)

	// Always use Sheet1
	readStart := time.Now()
	rows, err := f.GetRows("Sheet1")
	if err != nil {
		return nil, fmt.Errorf("failed to read Sheet1: %w", err)
	}
	readTime := time.Since(readStart)
	log.Printf("[DataReader] Sheet1 read in %.2fms (%d rows)", float64(readTime.Nanoseconds())/1e6, len(rows))

	if len(rows) < 2 {
		return nil, fmt.Errorf("Excel file must have at least a header row and one data row")
	}

	return r.processRows(rows)
}

// readCSVData reads CSV data into structured format
func (r *DataReader) readCSVData() (*ExcelData, error) {
	file, err := os.Open(r.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	readStart := time.Now()
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}
	readTime := time.Since(readStart)
	log.Printf("[DataReader] CSV file read in %.2fms (%d rows)", float64(readTime.Nanoseconds())/1e6, len(rows))

	if len(rows) < 2 {
		return nil, fmt.Errorf("CSV file must have at least a header row and one data row")
	}

	return r.processRows(rows)
}

// processRows converts raw string rows into ExcelData format
func (r *DataReader) processRows(rows [][]string) (*ExcelData, error) {
	// Extract headers from first row
	headerRow := rows[0]
	headers := make([]string, len(headerRow))
	for i, header := range headerRow {
		headers[i] = strings.TrimSpace(header)
	}

	// Extract data rows
	var dataRows []RawRowData
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowData := make(RawRowData)

		for j, cell := range row {
			if j < len(headers) {
				rowData[headers[j]] = strings.TrimSpace(cell)
			}
		}

		dataRows = append(dataRows, rowData)
	}

	log.Printf("[DataReader] %s file processed (%d columns, %d rows)",
		strings.ToUpper(r.fileType), len(headers), len(dataRows))

	return &ExcelData{
		Headers: headers,
		Rows:    dataRows,
	}, nil
}

// DetectEntityColumn automatically detects the entity column
func (r *DataReader) DetectEntityColumn(data *ExcelData) (string, error) {
	if len(data.Rows) == 0 {
		return "", fmt.Errorf("no data rows found")
	}

	// Common entity column names to check
	commonEntityColumns := []string{
		"id",
		"entity_id",
		"customer_id",
		"user_id",
		"account_id",
		"record_id",
		"key",
		"primary_key",
	}

	// Check for common entity column names
	for _, colName := range commonEntityColumns {
		for _, header := range data.Headers {
			if strings.ToLower(header) == colName {
				// Verify this column has unique, non-empty values
				if r.isValidEntityColumn(data, header) {
					return header, nil
				}
			}
		}
	}

	// Fall back to first column if no common names found
	if len(data.Headers) > 0 {
		firstCol := data.Headers[0]
		if r.isValidEntityColumn(data, firstCol) {
			return firstCol, nil
		}
	}

	return "", fmt.Errorf("could not detect a valid entity column")
}

// isValidEntityColumn checks if a column is suitable as an entity column
func (r *DataReader) isValidEntityColumn(data *ExcelData, columnName string) bool {
	values := make(map[string]bool)
	emptyCount := 0

	for _, row := range data.Rows {
		if value, exists := row[columnName]; exists {
			if value == "" {
				emptyCount++
			} else {
				values[value] = true
			}
		} else {
			emptyCount++
		}
	}

	// Entity column should have mostly non-empty, somewhat unique values
	totalRows := len(data.Rows)
	emptyRatio := float64(emptyCount) / float64(totalRows)
	uniqueRatio := float64(len(values)) / float64(totalRows)

	return emptyRatio < 0.5 && uniqueRatio > 0.5 // Less than 50% empty, more than 50% unique
}

// InferColumnTypes analyzes data to infer data types for each column
// Uses Excel's native cell types when available and improves type detection
func (r *DataReader) InferColumnTypes(data *ExcelData) (map[string]string, error) {
	if len(data.Rows) == 0 {
		return nil, fmt.Errorf("no data rows to analyze")
	}

	// For CSV files, use string-based inference only
	if r.fileType == "csv" {
		return r.inferColumnTypesFromStrings(data)
	}

	// For Excel files, try to use native cell types
	f, err := excelize.OpenFile(r.filePath)
	if err != nil {
		// Fallback to string-based inference if file can't be reopened
		return r.inferColumnTypesFromStrings(data)
	}
	defer f.Close()

	coercer := coercer.NewTypeCoercer(coercer.DefaultCoercionConfig())
	columnTypes := make(map[string]string)
	sheetName := "Sheet1"

	// Determine optimal sample size (up to 500 rows for better accuracy, but cap for performance)
	maxSampleSize := 500
	sampleSize := len(data.Rows)
	if sampleSize > maxSampleSize {
		sampleSize = maxSampleSize
	}

	// Use stratified sampling: take evenly distributed rows across the dataset
	sampleIndices := r.getStratifiedSample(len(data.Rows), sampleSize)

	for colIdx, header := range data.Headers {
		colLetter := r.columnIndexToLetter(colIdx)

		// Collect values with Excel native types
		values := make([]interface{}, 0, len(sampleIndices))
		excelTypes := make([]excelize.CellType, 0, len(sampleIndices))
		uniqueValues := make(map[string]bool)
		integerCount := 0
		floatCount := 0

		for _, rowIdx := range sampleIndices {
			cellRef := fmt.Sprintf("%s%d", colLetter, rowIdx+2) // +2 because row 1 is header, Excel is 1-indexed

			// Get native Excel cell type
			cellType, err := f.GetCellType(sheetName, cellRef)
			if err != nil {
				// Fallback to string value from data
				if rowIdx < len(data.Rows) {
					if value, exists := data.Rows[rowIdx][header]; exists {
						values = append(values, value)
						excelTypes = append(excelTypes, excelize.CellTypeUnset)
					}
				}
				continue
			}

			// Get cell value
			cellValue, err := f.GetCellValue(sheetName, cellRef)
			if err != nil {
				continue
			}

			// Track unique values for categorical detection
			if cellValue != "" {
				uniqueValues[cellValue] = true
			}

			// Check if numeric value is integer or float
			if cellType == excelize.CellTypeNumber {
				if val, err := strconv.ParseFloat(cellValue, 64); err == nil {
					if val == math.Trunc(val) {
						integerCount++
					} else {
						floatCount++
					}
				}
			}

			values = append(values, cellValue)
			excelTypes = append(excelTypes, cellType)
		}

		// Analyze type distribution using coercer
		analysis := coercer.AnalyzeTypeDistribution(values)

		// Use Excel native types as strong hints
		dataType := r.inferTypeWithExcelHints(header, analysis, excelTypes, uniqueValues, integerCount, floatCount, len(values))

		columnTypes[header] = dataType
	}

	return columnTypes, nil
}

// inferTypeWithExcelHints determines the best type using both coercer analysis and Excel native types
func (r *DataReader) inferTypeWithExcelHints(
	header string,
	analysis coercer.TypeAnalysis,
	excelTypes []excelize.CellType,
	uniqueValues map[string]bool,
	integerCount, floatCount, totalCount int,
) string {
	config := coercer.DefaultCoercionConfig()

	// Count Excel native types
	excelNumericCount := 0
	excelBoolCount := 0
	excelDateCount := 0
	excelStringCount := 0

	for _, ct := range excelTypes {
		switch ct {
		case excelize.CellTypeNumber:
			excelNumericCount++
		case excelize.CellTypeBool:
			excelBoolCount++
		case excelize.CellTypeDate:
			excelDateCount++
		case excelize.CellTypeInlineString, excelize.CellTypeSharedString:
			excelStringCount++
		}
	}

	// Use Excel native types as strong signal (weighted 70% vs 30% for coercer)
	if len(excelTypes) > 0 {
		excelNumericRatio := float64(excelNumericCount) / float64(len(excelTypes))
		excelBoolRatio := float64(excelBoolCount) / float64(len(excelTypes))
		excelDateRatio := float64(excelDateCount) / float64(len(excelTypes))

		// Combined ratio (weighted average)
		combinedNumericRatio := 0.7*excelNumericRatio + 0.3*analysis.NumericRatio
		combinedBoolRatio := 0.7*excelBoolRatio + 0.3*analysis.BooleanRatio
		combinedDateRatio := 0.7*excelDateRatio + 0.3*analysis.TimestampRatio

		// Prefer Excel's native type detection when strong signal exists
		if combinedNumericRatio >= config.NumericThreshold {
			// Check if all numeric values are integers
			if integerCount > 0 && floatCount == 0 {
				// Check for categorical codes (low cardinality integers)
				uniqueRatio := float64(len(uniqueValues)) / float64(totalCount)
				if uniqueRatio < 0.1 && len(uniqueValues) <= 20 {
					return "categorical"
				}
				return "numeric" // Could return "integer" if we add that type
			}
			return "numeric"
		}

		if combinedBoolRatio >= config.BooleanThreshold {
			return "boolean"
		}

		if combinedDateRatio >= config.TimestampThreshold {
			return "timestamp"
		}
	}

	// Fallback to coercer's recommended type
	if analysis.ValidCount > 0 {
		// Use the coercer's recommended type (it already does threshold checking)
		switch analysis.RecommendedType {
		case ingestion.ValueTypeNumeric:
			// Check for categorical codes (low cardinality numeric)
			uniqueRatio := float64(len(uniqueValues)) / float64(analysis.ValidCount)
			if uniqueRatio < 0.1 && len(uniqueValues) <= 20 && analysis.NumericRatio >= 0.5 {
				return "categorical"
			}
			return "numeric"
		case ingestion.ValueTypeBoolean:
			return "boolean"
		case ingestion.ValueTypeTimestamp:
			return "timestamp"
		case ingestion.ValueTypeString:
			// Check for categorical (few unique values relative to total)
			uniqueRatio := float64(len(uniqueValues)) / float64(analysis.ValidCount)
			if uniqueRatio < 0.1 && len(uniqueValues) <= 20 {
				return "categorical"
			}
			return "string"
		}
	}

	return "string" // default
}

// inferColumnTypesFromStrings fallback method using only string values
func (r *DataReader) inferColumnTypesFromStrings(data *ExcelData) (map[string]string, error) {
	coercer := coercer.NewTypeCoercer(coercer.DefaultCoercionConfig())
	columnTypes := make(map[string]string)

	maxSampleSize := 500
	sampleSize := len(data.Rows)
	if sampleSize > maxSampleSize {
		sampleSize = maxSampleSize
	}

	sampleIndices := r.getStratifiedSample(len(data.Rows), sampleSize)

	for _, header := range data.Headers {
		values := make([]interface{}, 0, len(sampleIndices))
		uniqueValues := make(map[string]bool)

		for _, idx := range sampleIndices {
			if value, exists := data.Rows[idx][header]; exists {
				values = append(values, value)
				if value != "" {
					uniqueValues[value] = true
				}
			}
		}

		analysis := coercer.AnalyzeTypeDistribution(values)

		dataType := "string" // default
		if analysis.ValidCount > 0 {
			switch analysis.RecommendedType {
			case ingestion.ValueTypeNumeric:
				uniqueRatio := float64(len(uniqueValues)) / float64(analysis.ValidCount)
				if uniqueRatio < 0.1 && len(uniqueValues) <= 20 {
					dataType = "categorical"
				} else {
					dataType = "numeric"
				}
			case ingestion.ValueTypeBoolean:
				dataType = "boolean"
			case ingestion.ValueTypeTimestamp:
				dataType = "timestamp"
			case ingestion.ValueTypeString:
				uniqueRatio := float64(len(uniqueValues)) / float64(analysis.ValidCount)
				if uniqueRatio < 0.1 && len(uniqueValues) <= 20 {
					dataType = "categorical"
				} else {
					dataType = "string"
				}
			}
		}

		columnTypes[header] = dataType
	}

	return columnTypes, nil
}

// getStratifiedSample returns evenly distributed row indices across the dataset
func (r *DataReader) getStratifiedSample(totalRows, sampleSize int) []int {
	if sampleSize >= totalRows {
		indices := make([]int, totalRows)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}

	indices := make([]int, 0, sampleSize)
	step := float64(totalRows) / float64(sampleSize)

	for i := 0; i < sampleSize; i++ {
		idx := int(math.Round(float64(i) * step))
		if idx < totalRows {
			indices = append(indices, idx)
		}
	}

	// Ensure we have exactly sampleSize indices, filling gaps if needed
	if len(indices) < sampleSize {
		// Add random samples from remaining rows
		used := make(map[int]bool)
		for _, idx := range indices {
			used[idx] = true
		}

		// Safety: prevent infinite loop if all rows are already used
		remaining := totalRows - len(used)
		if remaining > 0 {
			attempts := 0
			maxAttempts := remaining * 10 // Reasonable limit
			for len(indices) < sampleSize && attempts < maxAttempts {
				idx := rand.Intn(totalRows)
				if !used[idx] {
					indices = append(indices, idx)
					used[idx] = true
				}
				attempts++
			}
		}
		// If we still don't have enough, return what we have (shouldn't happen in practice)
	}

	return indices
}

// columnIndexToLetter converts 0-based column index to Excel column letter (A, B, ..., Z, AA, AB, ...)
func (r *DataReader) columnIndexToLetter(colIdx int) string {
	result := ""
	colIdx++ // Excel is 1-indexed internally
	for colIdx > 0 {
		colIdx--
		result = string(rune('A'+(colIdx%26))) + result
		colIdx /= 26
	}
	return result
}
