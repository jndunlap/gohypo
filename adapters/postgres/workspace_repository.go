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

// workspaceRepository implements the WorkspaceRepository interface
type workspaceRepository struct {
	db *sqlx.DB
}

// NewWorkspaceRepository creates a new workspace repository
func NewWorkspaceRepository(db *sqlx.DB) ports.WorkspaceRepository {
	return &workspaceRepository{db: db}
}

// Create inserts a new workspace into the database
func (r *workspaceRepository) Create(ctx context.Context, workspace *dataset.Workspace) error {
	metadataJSON, err := json.Marshal(workspace.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `INSERT INTO workspaces (
		id, user_id, name, description, color, is_default, metadata, created_at, updated_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9
	)`

	_, err = r.db.ExecContext(ctx, query,
		workspace.ID, workspace.UserID, workspace.Name, workspace.Description,
		workspace.Color, workspace.IsDefault, metadataJSON,
		workspace.CreatedAt, workspace.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	return nil
}

// GetByID retrieves a workspace by its ID
func (r *workspaceRepository) GetByID(ctx context.Context, id core.ID) (*dataset.Workspace, error) {
	query := `SELECT
		id, user_id, name, description, color, is_default, metadata, created_at, updated_at
	FROM workspaces WHERE id = $1`

	var workspace dataset.Workspace
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&workspace.ID, &workspace.UserID, &workspace.Name, &workspace.Description,
		&workspace.Color, &workspace.IsDefault, &metadataJSON,
		&workspace.CreatedAt, &workspace.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workspace not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get workspace: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		err = json.Unmarshal(metadataJSON, &workspace.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &workspace, nil
}

// GetByUserID retrieves all workspaces for a user
func (r *workspaceRepository) GetByUserID(ctx context.Context, userID core.ID) ([]*dataset.Workspace, error) {
	query := `SELECT
		id, user_id, name, description, color, is_default, metadata, created_at, updated_at
	FROM workspaces
	WHERE user_id = $1
	ORDER BY is_default DESC, created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query workspaces: %w", err)
	}
	defer rows.Close()

	workspaces := make([]*dataset.Workspace, 0) // Initialize as empty slice, not nil
	for rows.Next() {
		var workspace dataset.Workspace
		var metadataJSON []byte

		err := rows.Scan(
			&workspace.ID, &workspace.UserID, &workspace.Name, &workspace.Description,
			&workspace.Color, &workspace.IsDefault, &metadataJSON,
			&workspace.CreatedAt, &workspace.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan workspace: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			err = json.Unmarshal(metadataJSON, &workspace.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		workspaces = append(workspaces, &workspace)
	}

	return workspaces, nil
}

// Update modifies an existing workspace
func (r *workspaceRepository) Update(ctx context.Context, workspace *dataset.Workspace) error {
	metadataJSON, err := json.Marshal(workspace.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `UPDATE workspaces SET
		name = $2, description = $3, color = $4, is_default = $5, metadata = $6, updated_at = $7
	WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		workspace.ID, workspace.Name, workspace.Description, workspace.Color,
		workspace.IsDefault, metadataJSON, workspace.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to update workspace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("workspace not found: %s", workspace.ID)
	}

	return nil
}

// Delete removes a workspace from the database
func (r *workspaceRepository) Delete(ctx context.Context, id core.ID) error {
	// Check if it's the default workspace (can't delete default)
	var isDefault bool
	err := r.db.QueryRowContext(ctx, "SELECT is_default FROM workspaces WHERE id = $1", id).Scan(&isDefault)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("workspace not found: %s", id)
		}
		return fmt.Errorf("failed to check workspace: %w", err)
	}

	if isDefault {
		return fmt.Errorf("cannot delete default workspace")
	}

	query := `DELETE FROM workspaces WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("workspace not found: %s", id)
	}

	return nil
}

// GetDefaultForUser retrieves the default workspace for a user
func (r *workspaceRepository) GetDefaultForUser(ctx context.Context, userID core.ID) (*dataset.Workspace, error) {
	query := `SELECT
		id, user_id, name, description, color, is_default, metadata, created_at, updated_at
	FROM workspaces
	WHERE user_id = $1 AND is_default = true`

	var workspace dataset.Workspace
	var metadataJSON []byte

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&workspace.ID, &workspace.UserID, &workspace.Name, &workspace.Description,
		&workspace.Color, &workspace.IsDefault, &metadataJSON,
		&workspace.CreatedAt, &workspace.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("default workspace not found for user: %s", userID)
		}
		return nil, fmt.Errorf("failed to get default workspace: %w", err)
	}

	// Unmarshal metadata
	if len(metadataJSON) > 0 {
		err = json.Unmarshal(metadataJSON, &workspace.Metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	return &workspace, nil
}

// GetWithDatasets retrieves a workspace with all its datasets and relations
func (r *workspaceRepository) GetWithDatasets(ctx context.Context, id core.ID) (*ports.WorkspaceWithDatasets, error) {
	// Get the workspace
	workspace, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get datasets in this workspace
	datasetRepo := NewDatasetRepository(r.db)
	datasets, err := datasetRepo.GetByUserID(ctx, workspace.UserID, 1000, 0) // Get all datasets for user
	if err != nil {
		return nil, fmt.Errorf("failed to get datasets: %w", err)
	}

	// Filter to only datasets in this workspace
	var workspaceDatasets []*dataset.Dataset
	for _, ds := range datasets {
		if ds.WorkspaceID == id {
			workspaceDatasets = append(workspaceDatasets, ds)
		}
	}

	// Get relations
	relations, err := r.GetRelations(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get relations: %w", err)
	}

	return &ports.WorkspaceWithDatasets{
		Workspace: workspace,
		Datasets:  workspaceDatasets,
		Relations: relations,
	}, nil
}

// CreateRelation creates a new dataset relationship
func (r *workspaceRepository) CreateRelation(ctx context.Context, relation *dataset.DatasetRelation) error {
	metadataJSON, err := json.Marshal(relation.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `INSERT INTO workspace_dataset_relations (
		id, workspace_id, source_dataset_id, target_dataset_id,
		relation_type, confidence, metadata, discovered_at
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8
	)`

	_, err = r.db.ExecContext(ctx, query,
		relation.ID, relation.WorkspaceID, relation.SourceDatasetID, relation.TargetDatasetID,
		relation.RelationType, relation.Confidence, metadataJSON, relation.DiscoveredAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create relation: %w", err)
	}

	return nil
}

// GetRelations retrieves all relationships for a workspace
func (r *workspaceRepository) GetRelations(ctx context.Context, workspaceID core.ID) ([]*dataset.DatasetRelation, error) {
	query := `SELECT
		id, workspace_id, source_dataset_id, target_dataset_id,
		relation_type, confidence, metadata, discovered_at
	FROM workspace_dataset_relations
	WHERE workspace_id = $1
	ORDER BY discovered_at DESC`

	rows, err := r.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to query relations: %w", err)
	}
	defer rows.Close()

	var relations []*dataset.DatasetRelation
	for rows.Next() {
		var relation dataset.DatasetRelation
		var metadataJSON []byte

		err := rows.Scan(
			&relation.ID, &relation.WorkspaceID, &relation.SourceDatasetID, &relation.TargetDatasetID,
			&relation.RelationType, &relation.Confidence, &metadataJSON, &relation.DiscoveredAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan relation: %w", err)
		}

		// Unmarshal metadata
		if len(metadataJSON) > 0 {
			err = json.Unmarshal(metadataJSON, &relation.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
			}
		}

		relations = append(relations, &relation)
	}

	return relations, nil
}

// DeleteRelation removes a dataset relationship
func (r *workspaceRepository) DeleteRelation(ctx context.Context, id core.ID) error {
	query := `DELETE FROM workspace_dataset_relations WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete relation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("relation not found: %s", id)
	}

	return nil
}
