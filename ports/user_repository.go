package ports

import (
	"context"

	"gohypo/models"

	"github.com/google/uuid"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	// GetOrCreateDefaultUser gets the default user or creates it if it doesn't exist
	GetOrCreateDefaultUser(ctx context.Context) (*models.User, error)

	// GetUserByID retrieves a user by their ID
	GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error)

	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *models.User) error

	// ListUsers returns all users (for future multi-user support)
	ListUsers(ctx context.Context) ([]*models.User, error)
}
