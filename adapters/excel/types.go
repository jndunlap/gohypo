package excel

// RawRowData represents a row of raw Excel data as string key-value pairs
type RawRowData map[string]string

// ExcelData represents the complete Excel dataset
type ExcelData struct {
	Headers []string     // Column headers
	Rows    []RawRowData // Data rows
}


