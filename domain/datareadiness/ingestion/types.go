package ingestion

import (
	"fmt"
	"time"

	"gohypo/domain/core"
)

// CanonicalEvent represents the normalized event shape that all sources must conform to
type CanonicalEvent struct {
	EntityID   core.ID                `json:"entity_id"`             // Who (entity identifier)
	ObservedAt core.Timestamp         `json:"observed_at"`           // When (event timestamp)
	Source     string                 `json:"source"`                // Where (source system name)
	FieldKey   string                 `json:"field_key"`             // What (variable name)
	Value      Value                  `json:"value"`                 // Value (typed storage)
	RawPayload map[string]interface{} `json:"raw_payload,omitempty"` // Original data
}

// Value represents a typed value with deterministic coercion
type Value struct {
	Type         ValueType  `json:"type"`
	StringVal    *string    `json:"string_val,omitempty"`
	NumericVal   *float64   `json:"numeric_val,omitempty"`
	BooleanVal   *bool      `json:"boolean_val,omitempty"`
	TimestampVal *time.Time `json:"timestamp_val,omitempty"`
	IsMissing    bool       `json:"is_missing"`
}

// ValueType defines the storage type for values
type ValueType string

const (
	ValueTypeString    ValueType = "string"
	ValueTypeNumeric   ValueType = "numeric"
	ValueTypeBoolean   ValueType = "boolean"
	ValueTypeTimestamp ValueType = "timestamp"
	ValueTypeMissing   ValueType = "missing"
)

// IngestionResult contains the outcome of normalizing source data
type IngestionResult struct {
	SourceName     string           `json:"source_name"`
	EventsIngested int              `json:"events_ingested"`
	Errors         []IngestionError `json:"errors"`
	DurationMs     int64            `json:"duration_ms"`
}

// IngestionError represents a normalization failure
type IngestionError struct {
	RowIndex  int    `json:"row_index"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	ErrorType string `json:"error_type"`
	Message   string `json:"message"`
}

// SourceNormalizer defines how to normalize a specific source format
type SourceNormalizer interface {
	// Normalize converts source-specific data to canonical events
	Normalize(sourceData interface{}) ([]CanonicalEvent, []IngestionError, error)

	// SourceName returns the name of this source
	SourceName() string

	// RequiredFields returns fields that must be present in source data
	RequiredFields() []string
}

// NewStringValue creates a string value
func NewStringValue(s string) Value {
	if s == "" {
		return Value{Type: ValueTypeMissing, IsMissing: true}
	}
	return Value{Type: ValueTypeString, StringVal: &s}
}

// NewNumericValue creates a numeric value
func NewNumericValue(n float64) Value {
	return Value{Type: ValueTypeNumeric, NumericVal: &n}
}

// NewBooleanValue creates a boolean value
func NewBooleanValue(b bool) Value {
	return Value{Type: ValueTypeBoolean, BooleanVal: &b}
}

// NewTimestampValue creates a timestamp value
func NewTimestampValue(t time.Time) Value {
	return Value{Type: ValueTypeTimestamp, TimestampVal: &t}
}

// NewMissingValue creates a missing value
func NewMissingValue() Value {
	return Value{Type: ValueTypeMissing, IsMissing: true}
}

// String returns the string representation of the value
func (v Value) String() string {
	switch v.Type {
	case ValueTypeString:
		if v.StringVal != nil {
			return *v.StringVal
		}
	case ValueTypeNumeric:
		if v.NumericVal != nil {
			return fmt.Sprintf("%.6f", *v.NumericVal)
		}
	case ValueTypeBoolean:
		if v.BooleanVal != nil {
			return fmt.Sprintf("%t", *v.BooleanVal)
		}
	case ValueTypeTimestamp:
		if v.TimestampVal != nil {
			return v.TimestampVal.Format(time.RFC3339)
		}
	case ValueTypeMissing:
		return "<missing>"
	}
	return "<invalid>"
}

// IsNumeric returns true if the value represents a valid number
func (v Value) IsNumeric() bool {
	return v.Type == ValueTypeNumeric && v.NumericVal != nil
}

// IsString returns true if the value represents a valid string
func (v Value) IsString() bool {
	return v.Type == ValueTypeString && v.StringVal != nil
}

// IsBoolean returns true if the value represents a valid boolean
func (v Value) IsBoolean() bool {
	return v.Type == ValueTypeBoolean && v.BooleanVal != nil
}

// IsTimestamp returns true if the value represents a valid timestamp
func (v Value) IsTimestamp() bool {
	return v.Type == ValueTypeTimestamp && v.TimestampVal != nil
}

// AsFloat64 returns the numeric value as float64, or 0 if not numeric
func (v Value) AsFloat64() float64 {
	if v.NumericVal != nil {
		return *v.NumericVal
	}
	return 0.0
}

// AsString returns the string value, or empty string if not a string
func (v Value) AsString() string {
	if v.StringVal != nil {
		return *v.StringVal
	}
	return ""
}

// AsBoolean returns the boolean value, or false if not a boolean
func (v Value) AsBoolean() bool {
	if v.BooleanVal != nil {
		return *v.BooleanVal
	}
	return false
}
