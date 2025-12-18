package coercer

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gohypo/domain/datareadiness/ingestion"
)

// TypeCoercer handles deterministic type coercion with versioned rules
type TypeCoercer struct {
	config CoercionConfig
}

// CoercionConfig defines the coercion thresholds and rules
type CoercionConfig struct {
	NumericThreshold   float64 `json:"numeric_threshold"`   // % of values that must parse as numbers
	BooleanThreshold   float64 `json:"boolean_threshold"`   // % of values that must parse as booleans
	TimestampThreshold float64 `json:"timestamp_threshold"` // % of values that must parse as timestamps
	MaxCategories      int     `json:"max_categories"`      // Max categories before truncation
	NormalizeStrings   bool    `json:"normalize_strings"`   // Whether to trim/lower strings
}

// DefaultCoercionConfig returns sensible defaults
func DefaultCoercionConfig() CoercionConfig {
	return CoercionConfig{
		NumericThreshold:   0.8, // 80% must parse as numbers
		BooleanThreshold:   0.9, // 90% must parse as booleans
		TimestampThreshold: 0.8, // 80% must parse as timestamps
		MaxCategories:      100,
		NormalizeStrings:   true,
	}
}

// NewTypeCoercer creates a coercer with the given config
func NewTypeCoercer(config CoercionConfig) *TypeCoercer {
	return &TypeCoercer{config: config}
}

// CoerceValue deterministically converts an unknown value to a typed Value
func (c *TypeCoercer) CoerceValue(rawValue interface{}) ingestion.Value {
	if rawValue == nil {
		return ingestion.NewMissingValue()
	}

	// Convert to string for analysis
	strVal := c.toString(rawValue)

	// Try numeric first (most restrictive)
	if numericVal, ok := c.tryParseNumeric(strVal); ok {
		return numericVal
	}

	// Try boolean
	if boolVal, ok := c.tryParseBoolean(strVal); ok {
		return boolVal
	}

	// Try timestamp
	if tsVal, ok := c.tryParseTimestamp(strVal); ok {
		return tsVal
	}

	// Default to string/categorical
	return c.coerceToString(strVal)
}

// AnalyzeTypeDistribution analyzes a sample to determine the best type coercion strategy
func (c *TypeCoercer) AnalyzeTypeDistribution(values []interface{}) TypeAnalysis {
	analysis := TypeAnalysis{
		TotalCount: len(values),
	}

	validCount := 0
	for _, val := range values {
		if val != nil {
			validCount++
			strVal := c.toString(val)

			// Count how many values can be coerced to each type
			if _, ok := c.tryParseNumeric(strVal); ok {
				analysis.NumericCount++
			}
			if _, ok := c.tryParseBoolean(strVal); ok {
				analysis.BooleanCount++
			}
			if _, ok := c.tryParseTimestamp(strVal); ok {
				analysis.TimestampCount++
			}
		}
	}

	analysis.ValidCount = validCount
	analysis.NumericRatio = float64(analysis.NumericCount) / float64(validCount)
	analysis.BooleanRatio = float64(analysis.BooleanCount) / float64(validCount)
	analysis.TimestampRatio = float64(analysis.TimestampCount) / float64(validCount)

	// Determine recommended type based on thresholds
	analysis.RecommendedType = c.determineRecommendedType(analysis)

	return analysis
}

// coerceToString converts to normalized string value
func (c *TypeCoercer) coerceToString(strVal string) ingestion.Value {
	if strVal == "" {
		return ingestion.NewMissingValue()
	}

	if c.config.NormalizeStrings {
		strVal = c.normalizeString(strVal)
	}

	// Check if it's actually empty after normalization
	if strVal == "" {
		return ingestion.NewMissingValue()
	}

	return ingestion.NewStringValue(strVal)
}

// tryParseNumeric attempts to parse as numeric with strict rules
// Handles international formats: parentheses for negatives, European decimals, currency symbols
func (c *TypeCoercer) tryParseNumeric(strVal string) (ingestion.Value, bool) {
	if strVal == "" {
		return ingestion.Value{}, false
	}

	// Trim whitespace
	cleanVal := strings.TrimSpace(strVal)

	// Handle parentheses for negative numbers: (123) -> -123
	isNegative := false
	if strings.HasPrefix(cleanVal, "(") && strings.HasSuffix(cleanVal, ")") {
		cleanVal = strings.TrimPrefix(cleanVal, "(")
		cleanVal = strings.TrimSuffix(cleanVal, ")")
		isNegative = true
	}

	// Remove currency symbols: $, €, £, ¥
	currencySymbols := []string{"$", "€", "£", "¥", "USD", "EUR", "GBP", "JPY"}
	for _, symbol := range currencySymbols {
		cleanVal = strings.ReplaceAll(cleanVal, symbol, "")
	}
	cleanVal = strings.TrimSpace(cleanVal)

	// Remove percentage sign
	cleanVal = strings.ReplaceAll(cleanVal, "%", "")

	// Detect European/French number format (1.234,56 or 1 234,56)
	// Check if there's a comma that might be a decimal separator
	hasComma := strings.Contains(cleanVal, ",")
	hasPeriod := strings.Contains(cleanVal, ".")
	hasSpace := strings.Contains(cleanVal, " ")

	// European format: period as thousands separator, comma as decimal
	// French format: space as thousands separator, comma as decimal
	if hasComma && (hasPeriod || hasSpace) {
		// Count digits after comma - if <= 2, likely European decimal
		commaIdx := strings.LastIndex(cleanVal, ",")
		afterComma := cleanVal[commaIdx+1:]
		if len(afterComma) <= 3 && strings.Count(afterComma, "0123456789") == len(afterComma) {
			// Replace comma with period for decimal, remove periods/spaces as thousands separators
			cleanVal = strings.ReplaceAll(cleanVal, ".", "")
			cleanVal = strings.ReplaceAll(cleanVal, " ", "")
			cleanVal = strings.ReplaceAll(cleanVal, ",", ".")
		} else {
			// Comma is likely thousands separator, remove it
			cleanVal = strings.ReplaceAll(cleanVal, ",", "")
		}
	} else if hasComma && !hasPeriod {
		// Only comma present - could be decimal separator (European) or thousands separator
		// Try as decimal first (more common in European format)
		cleanVal = strings.ReplaceAll(cleanVal, ",", ".")
	} else {
		// Standard format: remove commas and spaces (thousands separators)
		cleanVal = strings.ReplaceAll(cleanVal, ",", "")
		cleanVal = strings.ReplaceAll(cleanVal, " ", "")
	}

	// Apply negative sign if parentheses were detected
	if isNegative {
		cleanVal = "-" + cleanVal
	}

	// Try parsing as float (handles scientific notation automatically)
	if val, err := strconv.ParseFloat(cleanVal, 64); err == nil {
		// Additional validation: not infinity, not NaN
		if !math.IsInf(val, 0) && !math.IsNaN(val) {
			return ingestion.NewNumericValue(val), true
		}
	}

	return ingestion.Value{}, false
}

// tryParseBoolean attempts to parse as boolean with strict rules
func (c *TypeCoercer) tryParseBoolean(strVal string) (ingestion.Value, bool) {
	if strVal == "" {
		return ingestion.Value{}, false
	}

	lowerVal := strings.ToLower(strings.TrimSpace(strVal))

	// Accept various boolean representations
	switch lowerVal {
	case "true", "1", "yes", "y", "on":
		return ingestion.NewBooleanValue(true), true
	case "false", "0", "no", "n", "off":
		return ingestion.NewBooleanValue(false), true
	}

	return ingestion.Value{}, false
}

// tryParseTimestamp attempts to parse as timestamp with multiple formats
func (c *TypeCoercer) tryParseTimestamp(strVal string) (ingestion.Value, bool) {
	if strVal == "" {
		return ingestion.Value{}, false
	}

	// Common timestamp formats to try
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"01/02/2006",
		"2006/01/02",
		"02-Jan-2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, strVal); err == nil {
			return ingestion.NewTimestampValue(t), true
		}
	}

	// Try Unix timestamp
	if unixVal, err := strconv.ParseInt(strVal, 10, 64); err == nil {
		if unixVal > 0 && unixVal < 2147483647 { // Reasonable Unix timestamp range
			t := time.Unix(unixVal, 0)
			return ingestion.NewTimestampValue(t), true
		}
	}

	return ingestion.Value{}, false
}

// normalizeString applies deterministic string normalization
func (c *TypeCoercer) normalizeString(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)

	// Convert to lowercase for consistency
	s = strings.ToLower(s)

	// Remove excessive whitespace
	re := regexp.MustCompile(`\s+`)
	s = re.ReplaceAllString(s, " ")

	// Remove control characters
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)

	return s
}

// toString converts interface{} to string safely
func (c *TypeCoercer) toString(val interface{}) string {
	if val == nil {
		return ""
	}

	switch v := val.(type) {
	case string:
		return v
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%.6f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// determineRecommendedType chooses the best type based on analysis
func (c *TypeCoercer) determineRecommendedType(analysis TypeAnalysis) ingestion.ValueType {
	// Check thresholds in order of preference (most restrictive first)
	if analysis.NumericRatio >= c.config.NumericThreshold {
		return ingestion.ValueTypeNumeric
	}

	if analysis.BooleanRatio >= c.config.BooleanThreshold {
		return ingestion.ValueTypeBoolean
	}

	if analysis.TimestampRatio >= c.config.TimestampThreshold {
		return ingestion.ValueTypeTimestamp
	}

	// Default to string/categorical
	return ingestion.ValueTypeString
}

// TypeAnalysis contains the results of type distribution analysis
type TypeAnalysis struct {
	TotalCount      int                 `json:"total_count"`
	ValidCount      int                 `json:"valid_count"`
	NumericCount    int                 `json:"numeric_count"`
	BooleanCount    int                 `json:"boolean_count"`
	TimestampCount  int                 `json:"timestamp_count"`
	NumericRatio    float64             `json:"numeric_ratio"`
	BooleanRatio    float64             `json:"boolean_ratio"`
	TimestampRatio  float64             `json:"timestamp_ratio"`
	RecommendedType ingestion.ValueType `json:"recommended_type"`
}
