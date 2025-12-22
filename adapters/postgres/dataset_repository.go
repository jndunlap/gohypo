package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/ports"

	"github.com/jmoiron/sqlx"
)

// datasetRepository implements the DatasetRepository interface
type datasetRepository struct {
	db *sqlx.DB
}

// NewDatasetRepository creates a new dataset repository
func NewDatasetRepository(db *sqlx.DB) ports.DatasetRepository {
	return &datasetRepository{db: db}
}

// Create inserts a new dataset into the database
func (r *datasetRepository) Create(ctx context.Context, ds *dataset.Dataset) error {
	metadataJSON, err := json.Marshal(ds.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `INSERT INTO datasets (
		id, user_id, workspace_id, original_filename, file_path, file_size, mime_type,
		display_name, domain, description, record_count, field_count, missing_rate,
		source, status, error_message, metadata, created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
	)`

	_, err = r.db.ExecContext(ctx, query,
		ds.ID, ds.UserID, ds.WorkspaceID, ds.OriginalFilename, ds.FilePath, ds.FileSize, ds.MimeType,
		ds.DisplayName, ds.Domain, ds.Description, ds.RecordCount, ds.FieldCount, ds.MissingRate,
		ds.Source, ds.Status, ds.ErrorMessage, metadataJSON, ds.CreatedAt, ds.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create dataset: %w", err)
	}

	return nil
}

// GetByID retrieves a dataset by its ID
func (r *datasetRepository) GetByID(ctx context.Context, id core.ID) (*dataset.Dataset, error) {
	query := `SELECT
		id, user_id, workspace_id, original_filename, COALESCE(file_path, '') as file_path, COALESCE(file_size, 0) as file_size, COALESCE(mime_type, '') as mime_type,
		display_name, domain, description, COALESCE(record_count, 0) as record_count, COALESCE(field_count, 0) as field_count, COALESCE(missing_rate, 0.0) as missing_rate,
		source, status, COALESCE(error_message, '') as error_message, metadata, created_at, updated_at
	FROM datasets WHERE id = $1`

	var ds dataset.Dataset
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&ds.ID, &ds.UserID, &ds.WorkspaceID, &ds.OriginalFilename, &ds.FilePath, &ds.FileSize, &ds.MimeType,
		&ds.DisplayName, &ds.Domain, &ds.Description, &ds.RecordCount, &ds.FieldCount, &ds.MissingRate,
		&ds.Source, &ds.Status, &ds.ErrorMessage, &metadataJSON, &ds.CreatedAt, &ds.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("dataset not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get dataset: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		err = json.Unmarshal(metadataJSON, &ds.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &ds, nil
}

// GetByUserID retrieves datasets for a specific user with pagination
func (r *datasetRepository) GetByUserID(ctx context.Context, userID core.ID, limit, offset int) ([]*dataset.Dataset, error) {
	query := `SELECT
		id, user_id, workspace_id, original_filename, COALESCE(file_path, '') as file_path, COALESCE(file_size, 0) as file_size, COALESCE(mime_type, '') as mime_type,
		display_name, domain, description, COALESCE(record_count, 0) as record_count, COALESCE(field_count, 0) as field_count, COALESCE(missing_rate, 0.0) as missing_rate,
		source, status, COALESCE(error_message, '') as error_message, metadata, created_at, updated_at
	FROM datasets
	WHERE user_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query datasets: %w", err)
	}
	defer rows.Close()

	var datasets []*dataset.Dataset
	for rows.Next() {
		var ds dataset.Dataset
		var metadataJSON []byte

		err := rows.Scan(
			&ds.ID, &ds.UserID, &ds.OriginalFilename, &ds.FilePath, &ds.FileSize, &ds.MimeType,
			&ds.DisplayName, &ds.Domain, &ds.Description, &ds.RecordCount, &ds.FieldCount, &ds.MissingRate,
			&ds.Source, &ds.Status, &ds.ErrorMessage, &metadataJSON, &ds.CreatedAt, &ds.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dataset: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			err = json.Unmarshal(metadataJSON, &ds.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		datasets = append(datasets, &ds)
	}

	return datasets, nil
}

// Update modifies an existing dataset
func (r *datasetRepository) Update(ctx context.Context, ds *dataset.Dataset) error {
	metadataJSON, err := json.Marshal(ds.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `UPDATE datasets SET
		original_filename = $2, file_path = $3, file_size = $4, mime_type = $5,
		display_name = $6, domain = $7, description = $8, record_count = $9,
		field_count = $10, missing_rate = $11, source = $12, status = $13,
		error_message = $14, metadata = $15, updated_at = $16
	WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		ds.ID, ds.OriginalFilename, ds.FilePath, ds.FileSize, ds.MimeType,
		ds.DisplayName, ds.Domain, ds.Description, ds.RecordCount, ds.FieldCount,
		ds.MissingRate, ds.Source, ds.Status, ds.ErrorMessage, metadataJSON, ds.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update dataset: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("dataset not found: %s", ds.ID)
	}

	return nil
}

// Delete removes a dataset from the database
func (r *datasetRepository) Delete(ctx context.Context, id core.ID) error {
	query := `DELETE FROM datasets WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete dataset: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("dataset not found: %s", id)
	}

	return nil
}

// GetCurrent retrieves the special "current" Excel dataset
func (r *datasetRepository) GetCurrent(ctx context.Context) (*dataset.Dataset, error) {
	query := `SELECT
		id, user_id, original_filename, COALESCE(file_path, '') as file_path, COALESCE(file_size, 0) as file_size, COALESCE(mime_type, '') as mime_type,
		display_name, domain, description, COALESCE(record_count, 0) as record_count, COALESCE(field_count, 0) as field_count, COALESCE(missing_rate, 0.0) as missing_rate,
		source, status, COALESCE(error_message, '') as error_message, metadata, created_at, updated_at
	FROM datasets WHERE id = $1`

	var ds dataset.Dataset
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, "550e8400-e29b-41d4-a716-446655440000").Scan(
		&ds.ID, &ds.UserID, &ds.OriginalFilename, &ds.FilePath, &ds.FileSize, &ds.MimeType,
		&ds.DisplayName, &ds.Domain, &ds.Description, &ds.RecordCount, &ds.FieldCount, &ds.MissingRate,
		&ds.Source, &ds.Status, &ds.ErrorMessage, &metadataJSON, &ds.CreatedAt, &ds.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("current dataset not found")
		}
		return nil, fmt.Errorf("failed to get current dataset: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		err = json.Unmarshal(metadataJSON, &ds.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &ds, nil
}

// ListByStatus retrieves datasets by processing status
func (r *datasetRepository) ListByStatus(ctx context.Context, status dataset.DatasetStatus) ([]*dataset.Dataset, error) {
	query := `SELECT
		id, user_id, workspace_id, original_filename, COALESCE(file_path, '') as file_path, COALESCE(file_size, 0) as file_size, COALESCE(mime_type, '') as mime_type,
		display_name, domain, description, COALESCE(record_count, 0) as record_count, COALESCE(field_count, 0) as field_count, COALESCE(missing_rate, 0.0) as missing_rate,
		source, status, COALESCE(error_message, '') as error_message, metadata, created_at, updated_at
	FROM datasets WHERE status = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query datasets by status: %w", err)
	}
	defer rows.Close()

	return r.scanDatasets(rows)
}

// ListByDomain retrieves datasets by business domain
func (r *datasetRepository) ListByDomain(ctx context.Context, domain string) ([]*dataset.Dataset, error) {
	query := `SELECT
		id, user_id, workspace_id, original_filename, COALESCE(file_path, '') as file_path, COALESCE(file_size, 0) as file_size, COALESCE(mime_type, '') as mime_type,
		display_name, domain, description, COALESCE(record_count, 0) as record_count, COALESCE(field_count, 0) as field_count, COALESCE(missing_rate, 0.0) as missing_rate,
		source, status, COALESCE(error_message, '') as error_message, metadata, created_at, updated_at
	FROM datasets WHERE domain = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to query datasets by domain: %w", err)
	}
	defer rows.Close()

	return r.scanDatasets(rows)
}

// UpdateStatus updates only the status and error message of a dataset
func (r *datasetRepository) UpdateStatus(ctx context.Context, id core.ID, status dataset.DatasetStatus, errorMsg string) error {
	query := `UPDATE datasets SET status = $2, error_message = $3, updated_at = NOW() WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id, status, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to update dataset status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("dataset not found: %s", id)
	}

	return nil
}

// GetByWorkspace retrieves datasets for a specific workspace
func (r *datasetRepository) GetByWorkspace(ctx context.Context, workspaceID core.ID, limit, offset int) ([]*dataset.Dataset, error) {
	query := `SELECT
		id, user_id, workspace_id, original_filename, COALESCE(file_path, '') as file_path, COALESCE(file_size, 0) as file_size, COALESCE(mime_type, '') as mime_type,
		display_name, domain, description, COALESCE(record_count, 0) as record_count, COALESCE(field_count, 0) as field_count, COALESCE(missing_rate, 0.0) as missing_rate,
		source, status, COALESCE(error_message, '') as error_message, metadata, created_at, updated_at
	FROM datasets
	WHERE workspace_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryContext(ctx, query, workspaceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query datasets by workspace: %w", err)
	}
	defer rows.Close()

	return r.scanDatasets(rows)
}

// scanDatasets is a helper function to scan multiple dataset rows
func (r *datasetRepository) scanDatasets(rows *sql.Rows) ([]*dataset.Dataset, error) {
	var datasets []*dataset.Dataset
	for rows.Next() {
		var ds dataset.Dataset
		var metadataJSON []byte

		err := rows.Scan(
			&ds.ID, &ds.UserID, &ds.WorkspaceID, &ds.OriginalFilename, &ds.FilePath, &ds.FileSize, &ds.MimeType,
			&ds.DisplayName, &ds.Domain, &ds.Description, &ds.RecordCount, &ds.FieldCount, &ds.MissingRate,
			&ds.Source, &ds.Status, &ds.ErrorMessage, &metadataJSON, &ds.CreatedAt, &ds.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan dataset: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			err = json.Unmarshal(metadataJSON, &ds.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		datasets = append(datasets, &ds)
	}

	return datasets, nil
}
