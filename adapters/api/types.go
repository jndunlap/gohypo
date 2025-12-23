package api

import (
	"time"
	"gohypo/domain/core"
)

// APIDataSource represents an API endpoint configuration
type APIDataSource struct {
	ID          core.ID `json:"id"`
	UserID      core.ID `json:"user_id"`
	WorkspaceID core.ID `json:"workspace_id"`

	// Connection settings
	Name        string            `json:"name"`
	BaseURL     string            `json:"base_url"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"query_params,omitempty"`

	// Authentication
	AuthMethod string `json:"auth_method"` // "none", "bearer", "api_key", "basic", "oauth"
	AuthToken  string `json:"auth_token,omitempty"`  // Encrypted
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`    // Encrypted

	// Data extraction
	DataPath    string `json:"data_path"`    // JSONPath for data array (e.g., "data.items")
	EntityPath  string `json:"entity_path"`  // JSONPath for entity ID field

	// Pagination
	PaginationType string `json:"pagination_type"` // "none", "offset", "cursor", "page"
	PageSize       int    `json:"page_size"`
	MaxPages       int    `json:"max_pages"`

	// Sync settings
	SyncInterval    time.Duration `json:"sync_interval"`
	RateLimit       int           `json:"rate_limit"`       // Requests per minute
	Timeout         time.Duration `json:"timeout"`
	RetryAttempts   int           `json:"retry_attempts"`

	// Status
	Enabled       bool      `json:"enabled"`
	LastSync      time.Time `json:"last_sync,omitempty"`
	NextSync      time.Time `json:"next_sync,omitempty"`
	Status        string    `json:"status"` // "active", "error", "disabled"
	ErrorMessage  string    `json:"error_message,omitempty"`

	// Schema tracking for drift detection
	SchemaFingerprint SchemaFingerprint `json:"schema_fingerprint,omitempty"`
}

// SchemaFingerprint tracks the statistical signature of API data for drift detection
type SchemaFingerprint struct {
	Version     int                        `json:"version"`
	Fields      map[string]FieldFingerprint `json:"fields"`
	CreatedAt   time.Time                  `json:"created_at"`
	DataHash    string                     `json:"data_hash"` // Hash of sample data
}

// FieldFingerprint captures statistical properties of a field for drift detection
type FieldFingerprint struct {
	DataType    string  `json:"data_type"`
	Nullable    bool    `json:"nullable"`
	SampleSize  int     `json:"sample_size"`

	// Statistical properties
	Mean        float64 `json:"mean,omitempty"`
	StdDev      float64 `json:"std_dev,omitempty"`
	Min         float64 `json:"min,omitempty"`
	Max         float64 `json:"max,omitempty"`
	Entropy     float64 `json:"entropy"`      // Information entropy
	Kurtosis    float64 `json:"kurtosis"`     // Fat-tail detection
	Cardinality int     `json:"cardinality"`  // Unique value count

	// Sample values for validation
	SampleValues []interface{} `json:"sample_values,omitempty"`
}

// APIData represents fetched API data
type APIData struct {
	Source      *APIDataSource `json:"source"`
	RawResponse []byte         `json:"raw_response"`
	ParsedData  []map[string]interface{} `json:"parsed_data"`
	Metadata    APIMetadata    `json:"metadata"`
}

// APIMetadata contains information about the API fetch
type APIMetadata struct {
	URL            string        `json:"url"`
	StatusCode     int           `json:"status_code"`
	ResponseTime   time.Duration `json:"response_time"`
	FetchedAt      time.Time     `json:"fetched_at"`
	RecordsCount   int           `json:"records_count"`
	ContentType    string        `json:"content_type"`
	RateLimitRemaining int       `json:"rate_limit_remaining,omitempty"`
	RateLimitReset     time.Time `json:"rate_limit_reset,omitempty"`
}

// SchemaDriftReport captures detected schema changes
type SchemaDriftReport struct {
	DataSourceID    core.ID         `json:"data_source_id"`
	Severity        DriftSeverity   `json:"severity"`
	Changes         []FieldChange   `json:"changes"`
	ImpactScore     float64         `json:"impact_score"` // 0-1, higher = more disruptive
	DetectionTime   time.Time       `json:"detection_time"`
	Recommendations []string        `json:"recommendations"`
}

// DriftSeverity indicates the impact level of schema changes
type DriftSeverity int

const (
	DriftSeverityNone DriftSeverity = iota
	DriftSeverityLow     // New fields, safe type changes
	DriftSeverityMedium  // Type changes requiring attention
	DriftSeverityHigh    // Breaking changes, field removals
	DriftSeverityCritical // Entity field changed or removed
)

// FieldChange describes a specific schema change
type FieldChange struct {
	FieldName  string        `json:"field_name"`
	ChangeType ChangeType    `json:"change_type"`
	OldValue   interface{}   `json:"old_value,omitempty"`
	NewValue   interface{}   `json:"new_value,omitempty"`
	Severity   DriftSeverity `json:"severity"`
	Impact     string        `json:"impact"` // Description of downstream effects
}

// ChangeType categorizes the type of schema change
type ChangeType string

const (
	ChangeTypeFieldAdded     ChangeType = "field_added"
	ChangeTypeFieldRemoved   ChangeType = "field_removed"
	ChangeTypeTypeChanged    ChangeType = "type_changed"
	ChangeTypeEntropySpike   ChangeType = "entropy_spike"
	ChangeTypeKurtosisShift  ChangeType = "kurtosis_shift"
	ChangeTypeCardinalityChange ChangeType = "cardinality_change"
)

// APIIngestResult represents the outcome of an API ingestion attempt
type APIIngestResult struct {
	DataSourceID   core.ID         `json:"data_source_id"`
	Success        bool            `json:"success"`
	RecordsIngested int            `json:"records_ingested"`
	Duration       time.Duration   `json:"duration"`
	Error          string          `json:"error,omitempty"`
	DriftDetected  *SchemaDriftReport `json:"drift_detected,omitempty"`
}
