package api

import (
	"time"
)

// APIAdapterConfig holds configuration for the API ingestion adapter
type APIAdapterConfig struct {
	// Data source settings
	DefaultTimeout         time.Duration `json:"default_timeout"`
	DefaultRateLimit       int           `json:"default_rate_limit"`       // requests per minute
	DefaultMaxPages        int           `json:"default_max_pages"`
	DefaultPageSize        int           `json:"default_page_size"`

	// Drift detection settings
	DriftThresholds        DriftThresholds `json:"drift_thresholds"`

	// Sync scheduling
	DefaultSyncInterval    time.Duration `json:"default_sync_interval"`
	MaxConcurrentSyncs     int           `json:"max_concurrent_syncs"`

	// Circuit breaker settings
	CircuitBreakerEnabled  bool          `json:"circuit_breaker_enabled"`
	CircuitBreakerTimeout  time.Duration `json:"circuit_breaker_timeout"`
	CircuitBreakerFailures int           `json:"circuit_breaker_failures"`

	// Storage settings
	SchemaFingerprintTTL   time.Duration `json:"schema_fingerprint_ttl"`
	MaxStoredFingerprints  int           `json:"max_stored_fingerprints"`

	// Quality assurance
	DataQualityChecks      bool          `json:"data_quality_checks"`
	MinRecordsThreshold    int           `json:"min_records_threshold"`
	MaxEmptyRecordsPercent float64       `json:"max_empty_records_percent"`

	// Notification settings
	NotifyOnDrift          bool     `json:"notify_on_drift"`
	DriftNotificationLevels []string `json:"drift_notification_levels"` // ["medium", "high", "critical"]
}

// DefaultAPIAdapterConfig returns sensible defaults for API ingestion
func DefaultAPIAdapterConfig() *APIAdapterConfig {
	return &APIAdapterConfig{
		DefaultTimeout:         30 * time.Second,
		DefaultRateLimit:       60,  // 60 requests per minute
		DefaultMaxPages:        10,
		DefaultPageSize:        100,

		DriftThresholds: DefaultDriftThresholds(),

		DefaultSyncInterval:  1 * time.Hour,
		MaxConcurrentSyncs:   5,

		CircuitBreakerEnabled:  true,
		CircuitBreakerTimeout:  5 * time.Minute,
		CircuitBreakerFailures: 3,

		SchemaFingerprintTTL:   30 * 24 * time.Hour, // 30 days
		MaxStoredFingerprints:  10,

		DataQualityChecks:      true,
		MinRecordsThreshold:    1,
		MaxEmptyRecordsPercent: 50.0,

		NotifyOnDrift: true,
		DriftNotificationLevels: []string{"high", "critical"},
	}
}

// Validate checks if the configuration is valid
func (c *APIAdapterConfig) Validate() error {
	if c.DefaultTimeout <= 0 {
		return &ValidationError{Field: "DefaultTimeout", Message: "must be positive"}
	}

	if c.DefaultRateLimit <= 0 {
		return &ValidationError{Field: "DefaultRateLimit", Message: "must be positive"}
	}

	if c.MaxConcurrentSyncs <= 0 {
		return &ValidationError{Field: "MaxConcurrentSyncs", Message: "must be positive"}
	}

	if c.MinRecordsThreshold < 0 {
		return &ValidationError{Field: "MinRecordsThreshold", Message: "cannot be negative"}
	}

	if c.MaxEmptyRecordsPercent < 0 || c.MaxEmptyRecordsPercent > 100 {
		return &ValidationError{Field: "MaxEmptyRecordsPercent", Message: "must be between 0 and 100"}
	}

	return nil
}

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Message)
}

// CircuitBreakerConfig holds circuit breaker specific settings
type CircuitBreakerConfig struct {
	Enabled  bool          `json:"enabled"`
	Timeout  time.Duration `json:"timeout"`
	Failures int           `json:"failures"`
	Cooldown time.Duration `json:"cooldown"`
}

// DefaultCircuitBreakerConfig returns default circuit breaker settings
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		Enabled:  true,
		Timeout:  5 * time.Minute,
		Failures: 3,
		Cooldown: 10 * time.Minute,
	}
}

// QualityCheckConfig holds data quality validation settings
type QualityCheckConfig struct {
	Enabled                bool    `json:"enabled"`
	MinRecords             int     `json:"min_records"`
	MaxEmptyRecordsPercent float64 `json:"max_empty_records_percent"`
	CheckIdenticalValues   bool    `json:"check_identical_values"`
	CheckDataTypes         bool    `json:"check_data_types"`
}

// DefaultQualityCheckConfig returns default quality check settings
func DefaultQualityCheckConfig() QualityCheckConfig {
	return QualityCheckConfig{
		Enabled:                true,
		MinRecords:             1,
		MaxEmptyRecordsPercent: 50.0,
		CheckIdenticalValues:   true,
		CheckDataTypes:         true,
	}
}

// SyncSchedulerConfig holds sync scheduling settings
type SyncSchedulerConfig struct {
	Enabled              bool          `json:"enabled"`
	DefaultInterval      time.Duration `json:"default_interval"`
	MaxConcurrentSyncs   int           `json:"max_concurrent_syncs"`
	RetryAttempts        int           `json:"retry_attempts"`
	RetryBackoff         time.Duration `json:"retry_backoff"`
	MaintenanceWindow    string        `json:"maintenance_window"` // "02:00-04:00"
	SkipWeekends         bool          `json:"skip_weekends"`
}

// DefaultSyncSchedulerConfig returns default sync scheduler settings
func DefaultSyncSchedulerConfig() SyncSchedulerConfig {
	return SyncSchedulerConfig{
		Enabled:            true,
		DefaultInterval:    1 * time.Hour,
		MaxConcurrentSyncs: 5,
		RetryAttempts:      3,
		RetryBackoff:       5 * time.Minute,
		MaintenanceWindow:  "",
		SkipWeekends:       false,
	}
}
