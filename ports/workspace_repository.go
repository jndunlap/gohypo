package ports

import (
	"context"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

// WorkspaceWithDatasets represents a workspace with its associated datasets
type WorkspaceWithDatasets struct {
	*dataset.Workspace
	Datasets  []*dataset.Dataset         `json:"datasets"`
	Relations []*dataset.DatasetRelation `json:"relations"`
}

// WorkspaceRepository defines the interface for workspace storage operations
type WorkspaceRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, workspace *dataset.Workspace) error
	GetByID(ctx context.Context, id core.ID) (*dataset.Workspace, error)
	GetByUserID(ctx context.Context, userID core.ID) ([]*dataset.Workspace, error)
	Update(ctx context.Context, workspace *dataset.Workspace) error
	Delete(ctx context.Context, id core.ID) error

	// Special queries
	GetDefaultForUser(ctx context.Context, userID core.ID) (*dataset.Workspace, error)
	GetWithDatasets(ctx context.Context, id core.ID) (*WorkspaceWithDatasets, error)

	// Dataset relationship operations
	CreateRelation(ctx context.Context, relation *dataset.DatasetRelation) error
	GetRelations(ctx context.Context, workspaceID core.ID) ([]*dataset.DatasetRelation, error)
	DeleteRelation(ctx context.Context, id core.ID) error
}
