package ports

import (
	"context"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
)

// DatasetRepository defines the interface for dataset storage operations
type DatasetRepository interface {
	// Core CRUD operations
	Create(ctx context.Context, ds *dataset.Dataset) error
	GetByID(ctx context.Context, id core.ID) (*dataset.Dataset, error)
	GetByUserID(ctx context.Context, userID core.ID, limit, offset int) ([]*dataset.Dataset, error)
	GetByWorkspace(ctx context.Context, workspaceID core.ID, limit, offset int) ([]*dataset.Dataset, error)
	Update(ctx context.Context, ds *dataset.Dataset) error
	Delete(ctx context.Context, id core.ID) error

	// Special queries
	GetCurrent(ctx context.Context) (*dataset.Dataset, error) // Get the "current" Excel dataset
	ListByStatus(ctx context.Context, status dataset.DatasetStatus) ([]*dataset.Dataset, error)
	ListByDomain(ctx context.Context, domain string) ([]*dataset.Dataset, error)

	// Bulk operations
	UpdateStatus(ctx context.Context, id core.ID, status dataset.DatasetStatus, errorMsg string) error
}
