package dataset

import (
	"gohypo/domain/core"
	"mime/multipart"
	"time"
)

// DatasetStatus represents the processing state of a dataset
type DatasetStatus string

const (
	StatusProcessing DatasetStatus = "processing"
	StatusReady      DatasetStatus = "ready"
	StatusFailed     DatasetStatus = "failed"
)

// Workspace represents a user's workspace for organizing datasets
type Workspace struct {
	ID          core.ID                `json:"id"`
	UserID      core.ID                `json:"user_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Color       string                 `json:"color"`      // Hex color for UI theming
	IsDefault   bool                   `json:"is_default"` // One default workspace per user
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// DatasetRelation represents relationships between datasets within a workspace
type DatasetRelation struct {
	ID              core.ID                `json:"id"`
	WorkspaceID     core.ID                `json:"workspace_id"`
	SourceDatasetID core.ID                `json:"source_dataset_id"`
	TargetDatasetID core.ID                `json:"target_dataset_id"`
	RelationType    string                 `json:"relation_type"` // 'entity_link', 'field_mapping', 'data_flow', 'reference'
	Confidence      float64                `json:"confidence,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	DiscoveredAt    time.Time              `json:"discovered_at"`
}

// Dataset represents a stored dataset with AI-generated metadata
type Dataset struct {
	ID     core.ID `json:"id"`
	UserID core.ID `json:"user_id"`

	// Workspace assignment
	WorkspaceID core.ID `json:"workspace_id"`

	// File information
	OriginalFilename string `json:"original_filename"`
	FilePath         string `json:"file_path,omitempty"`
	FileSize         int64  `json:"file_size"`
	MimeType         string `json:"mime_type"`

	// AI-generated naming and context (from Forensic Scout)
	DisplayName string `json:"display_name"` // snake_case name like "customer_purchase_history"
	Domain      string `json:"domain"`       // "Retail Analytics", "Healthcare", etc.
	Description string `json:"description"`  // AI-generated summary

	// Dataset statistics
	RecordCount int     `json:"record_count"`
	FieldCount  int     `json:"field_count"`
	MissingRate float64 `json:"missing_rate"`
	Source      string  `json:"source"` // "upload", "excel", "api"

	// Processing state
	Status       DatasetStatus `json:"status"`
	ErrorMessage string        `json:"error_message,omitempty"`

	// Rich metadata stored as structured data
	Metadata DatasetMetadata `json:"metadata"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Populated relationships (when loaded)
	Workspace *Workspace        `json:"workspace,omitempty"`
	Relations []DatasetRelation `json:"relations,omitempty"`
}

// DatasetMetadata contains detailed information about the dataset
type DatasetMetadata struct {
	Fields     []FieldInfo              `json:"fields"`
	SampleRows []map[string]interface{} `json:"sample_rows"`
	AIAnalysis ForensicScoutResult      `json:"ai_analysis"`
	FileInfo   FileInfo                 `json:"file_info,omitempty"`
}

// FieldInfo describes a single field/column in the dataset
type FieldInfo struct {
	Name         string                 `json:"name"`
	DataType     string                 `json:"data_type"` // "numeric", "categorical", "text", etc.
	Nullable     bool                   `json:"nullable"`
	UniqueCount  int                    `json:"unique_count"`
	MissingCount int                    `json:"missing_count"`
	SampleValues []interface{}          `json:"sample_values,omitempty"`
	Statistics   map[string]interface{} `json:"statistics,omitempty"` // min, max, mean, etc.
}

// ForensicScoutResult contains the AI analysis results
type ForensicScoutResult struct {
	Domain      string    `json:"domain"`
	DatasetName string    `json:"dataset_name"` // snake_case name
	Confidence  float64   `json:"confidence,omitempty"`
	AnalyzedAt  time.Time `json:"analyzed_at"`
}

// FileInfo contains file-specific metadata
type FileInfo struct {
	Encoding   string `json:"encoding,omitempty"`
	Delimiter  string `json:"delimiter,omitempty"`
	HasHeaders bool   `json:"has_headers"`
	SheetName  string `json:"sheet_name,omitempty"` // for Excel files
}

// DatasetUpload represents an uploaded file before processing
type DatasetUpload struct {
	UserID      core.ID
	WorkspaceID core.ID
	Filename    string
	File        multipart.File // Multipart file from HTTP upload
	MimeType    string
}

// NewDataset creates a new dataset with default values
func NewDataset(userID core.ID, originalFilename string) *Dataset {
	return &Dataset{
		ID:               core.NewID(),
		UserID:           userID,
		OriginalFilename: originalFilename,
		Status:           StatusProcessing,
		Source:           "upload",
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

// IsReady returns true if the dataset is ready for use
func (d *Dataset) IsReady() bool {
	return d.Status == StatusReady
}

// GetDisplayName returns the AI-generated display name or falls back to original filename
func (d *Dataset) GetDisplayName() string {
	if d.DisplayName != "" {
		return d.DisplayName
	}
	return d.OriginalFilename
}

// GetDomain returns the detected domain or a default value
func (d *Dataset) GetDomain() string {
	if d.Domain != "" {
		return d.Domain
	}
	return "Unknown Domain"
}

// IsInWorkspace checks if the dataset belongs to a specific workspace
func (d *Dataset) IsInWorkspace(workspaceID core.ID) bool {
	return d.WorkspaceID == workspaceID
}

// GetRelatedDatasets returns datasets that have relations with this one
func (d *Dataset) GetRelatedDatasets() []core.ID {
	var related []core.ID
	for _, relation := range d.Relations {
		if relation.SourceDatasetID == d.ID {
			related = append(related, relation.TargetDatasetID)
		} else if relation.TargetDatasetID == d.ID {
			related = append(related, relation.SourceDatasetID)
		}
	}
	return related
}

// NewWorkspace creates a new workspace with default values
func NewWorkspace(userID core.ID, name string) *Workspace {
	return &Workspace{
		ID:        core.NewID(),
		UserID:    userID,
		Name:      name,
		Color:     "#3B82F6", // Default blue color
		IsDefault: false,
		Metadata:  make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// NewDefaultWorkspace creates a default workspace for a user
func NewDefaultWorkspace(userID core.ID) *Workspace {
	workspace := NewWorkspace(userID, "Default Workspace")
	workspace.IsDefault = true
	workspace.Description = "Your primary workspace for data analysis and research"
	workspace.Metadata = map[string]interface{}{
		"auto_discover_relations": true,
		"max_datasets":            50,
	}
	return workspace
}

// CanAddDataset checks if a workspace can accommodate more datasets
func (w *Workspace) CanAddDataset() bool {
	if maxDatasets, ok := w.Metadata["max_datasets"].(float64); ok {
		// In a real implementation, you'd check the actual dataset count
		// For now, return true (implement counting logic later)
		_ = maxDatasets
		return true
	}
	return true // No limit set
}
