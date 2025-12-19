package postgres

import (
	"context"
	"database/sql"
	"errors"

	"gohypo/models"
	"gohypo/ports"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var defaultUserID = uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")

// UserRepositoryImpl implements UserRepository for PostgreSQL
type UserRepositoryImpl struct {
	db *sqlx.DB
}

// NewUserRepository creates a new PostgreSQL user repository
func NewUserRepository(db *sqlx.DB) ports.UserRepository {
	return &UserRepositoryImpl{db: db}
}

// GetOrCreateDefaultUser gets the default user or creates it if it doesn't exist
func (r *UserRepositoryImpl) GetOrCreateDefaultUser(ctx context.Context) (*models.User, error) {
	// Try to get existing default user
	var user models.User
	err := r.db.GetContext(ctx, &user, `
		SELECT id, email, username, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`, defaultUserID)

	if err == nil {
		return &user, nil
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Create default user
	user = models.User{
		ID:       defaultUserID,
		Email:    "default@grohypo.local",
		Username: "default",
		IsActive: true,
	}

	_, err = r.db.NamedExecContext(ctx, `
		INSERT INTO users (id, email, username, is_active, created_at, updated_at)
		VALUES (:id, :email, :username, :is_active, NOW(), NOW())
	`, user)

	if err != nil {
		// Handle unique constraint violation (user might have been created by another process)
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
			// Try to get the user again
			return r.GetUserByID(ctx, defaultUserID)
		}
		return nil, err
	}

	return &user, nil
}

// GetUserByID retrieves a user by their ID
func (r *UserRepositoryImpl) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.db.GetContext(ctx, &user, `
		SELECT id, email, username, is_active, created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CreateUser creates a new user
func (r *UserRepositoryImpl) CreateUser(ctx context.Context, user *models.User) error {
	user.ID = uuid.New()
	_, err := r.db.NamedExecContext(ctx, `
		INSERT INTO users (id, email, username, is_active, created_at, updated_at)
		VALUES (:id, :email, :username, :is_active, NOW(), NOW())
	`, user)
	return err
}

// ListUsers returns all users (for future multi-user support)
func (r *UserRepositoryImpl) ListUsers(ctx context.Context) ([]*models.User, error) {
	var users []*models.User
	err := r.db.SelectContext(ctx, &users, `
		SELECT id, email, username, is_active, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
	`)
	return users, err
}
