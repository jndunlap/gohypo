package api

import (
	"encoding/json"
	"fmt"
	"time"

	"gohypo/domain/core"
)

// SSEEventType defines the types of SSE events for API ingestion
type SSEEventType string

const (
	EventTypeAPISyncStarted      SSEEventType = "api_sync_started"
	EventTypeAPISyncProgress     SSEEventType = "api_sync_progress"
	EventTypeAPISyncCompleted    SSEEventType = "api_sync_completed"
	EventTypeAPISyncFailed       SSEEventType = "api_sync_failed"
	EventTypeSchemaDriftDetected SSEEventType = "schema_drift_detected"
	EventTypeDriftResolved       SSEEventType = "drift_resolved"
	EventTypeDataQualityIssue    SSEEventType = "data_quality_issue"
	EventTypeRateLimitExceeded   SSEEventType = "rate_limit_exceeded"
)

// SSEEvent represents a server-sent event for API ingestion
type SSEEvent struct {
	EventType SSEEventType `json:"event_type"`
	SessionID string       `json:"session_id,omitempty"`
	Timestamp time.Time    `json:"timestamp"`
	Data      interface{}  `json:"data"`
}

// ToSSEFormat converts the event to SSE format
func (e *SSEEvent) ToSSEFormat() string {
	eventData := map[string]interface{}{
		"event_type": e.EventType,
		"timestamp":  e.Timestamp,
		"data":       e.Data,
	}

	if e.SessionID != "" {
		eventData["session_id"] = e.SessionID
	}

	jsonData, err := json.Marshal(eventData)
	if err != nil {
		// Fallback to basic format
		return fmt.Sprintf("event: %s\ndata: %s\n\n", e.EventType, "error marshalling event")
	}

	return fmt.Sprintf("event: %s\ndata: %s\n\n", e.EventType, string(jsonData))
}

// APISyncStartedEvent data for sync start
type APISyncStartedEvent struct {
	DataSourceID   core.ID `json:"data_source_id"`
	DataSourceName string  `json:"data_source_name"`
	ExpectedRecords int    `json:"expected_records,omitempty"`
}

// APISyncProgressEvent data for sync progress updates
type APISyncProgressEvent struct {
	DataSourceID    core.ID  `json:"data_source_id"`
	DataSourceName  string   `json:"data_source_name"`
	ProgressPercent float64  `json:"progress_percent"`
	CurrentStep     string   `json:"current_step"`
	RecordsFetched  int      `json:"records_fetched,omitempty"`
	RateLimitRemaining int   `json:"rate_limit_remaining,omitempty"`
}

// APISyncCompletedEvent data for successful sync completion
type APISyncCompletedEvent struct {
	DataSourceID     core.ID        `json:"data_source_id"`
	DataSourceName   string         `json:"data_source_name"`
	RecordsIngested  int            `json:"records_ingested"`
	Duration         time.Duration  `json:"duration"`
	DatasetID        core.ID        `json:"dataset_id,omitempty"`
	NewDataset       bool           `json:"new_dataset"`
}

// APISyncFailedEvent data for sync failures
type APISyncFailedEvent struct {
	DataSourceID   core.ID `json:"data_source_id"`
	DataSourceName string  `json:"data_source_name"`
	Error          string  `json:"error"`
	ErrorType      string  `json:"error_type"` // "network", "auth", "schema", "rate_limit", "timeout"
	Retryable      bool    `json:"retryable"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
}

// SchemaDriftDetectedEvent data for schema drift detection
type SchemaDriftDetectedEvent struct {
	DataSourceID    core.ID            `json:"data_source_id"`
	DataSourceName  string             `json:"data_source_name"`
	Severity        string             `json:"severity"` // "low", "medium", "high", "critical"
	ImpactScore     float64            `json:"impact_score"`
	Changes         []DriftChangeEvent `json:"changes"`
	Recommendations []string           `json:"recommendations"`
	AutoResolvable  bool               `json:"auto_resolvable"`
	RequiresReview  bool               `json:"requires_review"`
}

// DriftChangeEvent represents a single schema change
type DriftChangeEvent struct {
	FieldName  string `json:"field_name"`
	ChangeType string `json:"change_type"`
	OldValue   interface{} `json:"old_value,omitempty"`
	NewValue   interface{} `json:"new_value,omitempty"`
	Severity   string `json:"severity"`
	Impact     string `json:"impact"`
}

// DriftResolvedEvent data for drift resolution
type DriftResolvedEvent struct {
	DataSourceID   core.ID `json:"data_source_id"`
	DataSourceName string  `json:"data_source_name"`
	Resolution     string  `json:"resolution"` // "auto_accepted", "manual_accepted", "rejected", "quarantined"
	ActionTaken    string  `json:"action_taken"`
	AffectedDataset *core.ID `json:"affected_dataset,omitempty"`
}

// DataQualityIssueEvent data for data quality problems
type DataQualityIssueEvent struct {
	DataSourceID   core.ID   `json:"data_source_id"`
	DataSourceName string    `json:"data_source_name"`
	Issues         []string  `json:"issues"`
	Severity       string    `json:"severity"` // "warning", "error", "critical"
	RecordsAffected int      `json:"records_affected,omitempty"`
}

// RateLimitExceededEvent data for rate limiting
type RateLimitExceededEvent struct {
	DataSourceID       core.ID     `json:"data_source_id"`
	DataSourceName     string      `json:"data_source_name"`
	APIProvider        string      `json:"api_provider"`
	LimitType          string      `json:"limit_type"` // "requests_per_minute", "requests_per_hour", "concurrent"
	CurrentUsage       int         `json:"current_usage"`
	LimitValue         int         `json:"limit_value"`
	ResetTime          *time.Time  `json:"reset_time,omitempty"`
	BackoffDuration    time.Duration `json:"backoff_duration"`
}

// SSEEventBuilder provides methods to create SSE events
type SSEEventBuilder struct{}

// NewSSEEventBuilder creates a new event builder
func NewSSEEventBuilder() *SSEEventBuilder {
	return &SSEEventBuilder{}
}

// BuildAPISyncStarted creates a sync started event
func (b *SSEEventBuilder) BuildAPISyncStarted(sessionID string, event APISyncStartedEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeAPISyncStarted,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// BuildAPISyncProgress creates a sync progress event
func (b *SSEEventBuilder) BuildAPISyncProgress(sessionID string, event APISyncProgressEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeAPISyncProgress,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// BuildAPISyncCompleted creates a sync completed event
func (b *SSEEventBuilder) BuildAPISyncCompleted(sessionID string, event APISyncCompletedEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeAPISyncCompleted,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// BuildAPISyncFailed creates a sync failed event
func (b *SSEEventBuilder) BuildAPISyncFailed(sessionID string, event APISyncFailedEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeAPISyncFailed,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// BuildSchemaDriftDetected creates a schema drift detected event
func (b *SSEEventBuilder) BuildSchemaDriftDetected(sessionID string, event SchemaDriftDetectedEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeSchemaDriftDetected,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// BuildDriftResolved creates a drift resolved event
func (b *SSEEventBuilder) BuildDriftResolved(sessionID string, event DriftResolvedEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeDriftResolved,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// BuildDataQualityIssue creates a data quality issue event
func (b *SSEEventBuilder) BuildDataQualityIssue(sessionID string, event DataQualityIssueEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeDataQualityIssue,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// BuildRateLimitExceeded creates a rate limit exceeded event
func (b *SSEEventBuilder) BuildRateLimitExceeded(sessionID string, event RateLimitExceededEvent) *SSEEvent {
	return &SSEEvent{
		EventType: EventTypeRateLimitExceeded,
		SessionID: sessionID,
		Timestamp: time.Now(),
		Data:      event,
	}
}

// Helper functions to convert internal types to event types

// ConvertDriftSeverity converts DriftSeverity to string
func ConvertDriftSeverity(severity DriftSeverity) string {
	switch severity {
	case DriftSeverityLow:
		return "low"
	case DriftSeverityMedium:
		return "medium"
	case DriftSeverityHigh:
		return "high"
	case DriftSeverityCritical:
		return "critical"
	default:
		return "none"
	}
}

// ConvertDriftChanges converts []FieldChange to []DriftChangeEvent
func ConvertDriftChanges(changes []FieldChange) []DriftChangeEvent {
	result := make([]DriftChangeEvent, len(changes))
	for i, change := range changes {
		result[i] = DriftChangeEvent{
			FieldName:  change.FieldName,
			ChangeType: string(change.ChangeType),
			OldValue:   change.OldValue,
			NewValue:   change.NewValue,
			Severity:   ConvertDriftSeverity(change.Severity),
			Impact:     change.Impact,
		}
	}
	return result
}

// SSEEventBroadcaster handles broadcasting SSE events for API ingestion
type SSEEventBroadcaster struct {
	eventBuilder *SSEEventBuilder
	sseHub       interface{} // This would be your SSE hub interface
}

// NewSSEEventBroadcaster creates a new event broadcaster
func NewSSEEventBroadcaster(sseHub interface{}) *SSEEventBroadcaster {
	return &SSEEventBroadcaster{
		eventBuilder: NewSSEEventBuilder(),
		sseHub:       sseHub,
	}
}

// BroadcastAPISyncStarted broadcasts a sync started event
func (b *SSEEventBroadcaster) BroadcastAPISyncStarted(sessionID string, dataSource *APIDataSource, expectedRecords int) {
	event := b.eventBuilder.BuildAPISyncStarted(sessionID, APISyncStartedEvent{
		DataSourceID:     dataSource.ID,
		DataSourceName:   dataSource.Name,
		ExpectedRecords:  expectedRecords,
	})
	b.broadcastEvent(event)
}

// BroadcastAPISyncProgress broadcasts a sync progress event
func (b *SSEEventBroadcaster) BroadcastAPISyncProgress(sessionID string, dataSource *APIDataSource, progressPercent float64, currentStep string, recordsFetched int, rateLimitRemaining int) {
	event := b.eventBuilder.BuildAPISyncProgress(sessionID, APISyncProgressEvent{
		DataSourceID:       dataSource.ID,
		DataSourceName:     dataSource.Name,
		ProgressPercent:    progressPercent,
		CurrentStep:        currentStep,
		RecordsFetched:     recordsFetched,
		RateLimitRemaining: rateLimitRemaining,
	})
	b.broadcastEvent(event)
}

// BroadcastAPISyncCompleted broadcasts a sync completed event
func (b *SSEEventBroadcaster) BroadcastAPISyncCompleted(sessionID string, dataSource *APIDataSource, result *APIIngestResult, datasetID core.ID, newDataset bool) {
	event := b.eventBuilder.BuildAPISyncCompleted(sessionID, APISyncCompletedEvent{
		DataSourceID:     dataSource.ID,
		DataSourceName:   dataSource.Name,
		RecordsIngested:  result.RecordsIngested,
		Duration:         result.Duration,
		DatasetID:        datasetID,
		NewDataset:       newDataset,
	})
	b.broadcastEvent(event)
}

// BroadcastAPISyncFailed broadcasts a sync failed event
func (b *SSEEventBroadcaster) BroadcastAPISyncFailed(sessionID string, dataSource *APIDataSource, err error, errorType string, retryable bool, nextRetryAt *time.Time) {
	event := b.eventBuilder.BuildAPISyncFailed(sessionID, APISyncFailedEvent{
		DataSourceID:   dataSource.ID,
		DataSourceName: dataSource.Name,
		Error:          err.Error(),
		ErrorType:      errorType,
		Retryable:      retryable,
		NextRetryAt:    nextRetryAt,
	})
	b.broadcastEvent(event)
}

// BroadcastSchemaDriftDetected broadcasts a schema drift detected event
func (b *SSEEventBroadcaster) BroadcastSchemaDriftDetected(sessionID string, dataSource *APIDataSource, driftReport *SchemaDriftReport) {
	event := b.eventBuilder.BuildSchemaDriftDetected(sessionID, SchemaDriftDetectedEvent{
		DataSourceID:    dataSource.ID,
		DataSourceName:  dataSource.Name,
		Severity:        ConvertDriftSeverity(driftReport.Severity),
		ImpactScore:     driftReport.ImpactScore,
		Changes:         ConvertDriftChanges(driftReport.Changes),
		Recommendations: driftReport.Recommendations,
		AutoResolvable:  driftReport.Severity <= DriftSeverityMedium,
		RequiresReview:  driftReport.Severity >= DriftSeverityHigh,
	})
	b.broadcastEvent(event)
}

// BroadcastDriftResolved broadcasts a drift resolved event
func (b *SSEEventBroadcaster) BroadcastDriftResolved(sessionID string, dataSource *APIDataSource, resolution string, actionTaken string, affectedDataset *core.ID) {
	event := b.eventBuilder.BuildDriftResolved(sessionID, DriftResolvedEvent{
		DataSourceID:    dataSource.ID,
		DataSourceName:  dataSource.Name,
		Resolution:      resolution,
		ActionTaken:     actionTaken,
		AffectedDataset: affectedDataset,
	})
	b.broadcastEvent(event)
}

// BroadcastDataQualityIssue broadcasts a data quality issue event
func (b *SSEEventBroadcaster) BroadcastDataQualityIssue(sessionID string, dataSource *APIDataSource, issues []string, severity string, recordsAffected int) {
	event := b.eventBuilder.BuildDataQualityIssue(sessionID, DataQualityIssueEvent{
		DataSourceID:     dataSource.ID,
		DataSourceName:   dataSource.Name,
		Issues:           issues,
		Severity:         severity,
		RecordsAffected:  recordsAffected,
	})
	b.broadcastEvent(event)
}

// BroadcastRateLimitExceeded broadcasts a rate limit exceeded event
func (b *SSEEventBroadcaster) BroadcastRateLimitExceeded(sessionID string, dataSource *APIDataSource, apiProvider string, limitType string, currentUsage int, limitValue int, resetTime *time.Time, backoffDuration time.Duration) {
	event := b.eventBuilder.BuildRateLimitExceeded(sessionID, RateLimitExceededEvent{
		DataSourceID:     dataSource.ID,
		DataSourceName:   dataSource.Name,
		APIProvider:      apiProvider,
		LimitType:        limitType,
		CurrentUsage:     currentUsage,
		LimitValue:       limitValue,
		ResetTime:        resetTime,
		BackoffDuration:  backoffDuration,
	})
	b.broadcastEvent(event)
}

// broadcastEvent sends the event to the SSE hub
func (b *SSEEventBroadcaster) broadcastEvent(event *SSEEvent) {
	// This would integrate with your existing SSE hub
	// For example:
	// if hub, ok := b.sseHub.(*api.SSEHub); ok {
	//     hub.BroadcastToSession(event.SessionID, event.ToSSEFormat())
	// }
}
