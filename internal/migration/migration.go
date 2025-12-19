package migration

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"gohypo/internal/errors"
)

// Migrator defines the interface for database migration operations
type Migrator interface {
	Run(ctx context.Context, db *sqlx.DB) error
	Version() string
}

// MigrationRunner handles database schema migrations
type MigrationRunner struct {
	version string
}

// NewRunner creates a new migration runner
func NewRunner() *MigrationRunner {
	return &MigrationRunner{
		version: "1.0.0",
	}
}

// Version returns the migration version
func (r *MigrationRunner) Version() string {
	return r.version
}

// Run executes all database migrations in the correct order
func (r *MigrationRunner) Run(ctx context.Context, db *sqlx.DB) error {
	if err := r.createUsersTable(ctx, db); err != nil {
		return errors.Wrap(err, "failed to create users table")
	}

	if err := r.createResearchSessionsTable(ctx, db); err != nil {
		return errors.Wrap(err, "failed to create research_sessions table")
	}

	if err := r.addResearchSessionsColumns(ctx, db); err != nil {
		return errors.Wrap(err, "failed to add research_sessions columns")
	}

	if err := r.createHypothesisResultsTable(ctx, db); err != nil {
		return errors.Wrap(err, "failed to create hypothesis_results table")
	}

	if err := r.addHypothesisResultsConstraints(ctx, db); err != nil {
		return errors.Wrap(err, "failed to add hypothesis_results constraints")
	}

	if err := r.createIndexes(ctx, db); err != nil {
		return errors.Wrap(err, "failed to create indexes")
	}

	if err := r.insertDefaultUser(ctx, db); err != nil {
		return errors.Wrap(err, "failed to insert default user")
	}

	return nil
}

func (r *MigrationRunner) createUsersTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(100) UNIQUE,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	return err
}

func (r *MigrationRunner) createResearchSessionsTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS research_sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			state VARCHAR(50) NOT NULL DEFAULT 'idle',
			progress DECIMAL(5,2) DEFAULT 0.0,
			current_hypothesis TEXT,
			started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			completed_at TIMESTAMP WITH TIME ZONE,
			error_message TEXT,
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	return err
}

func (r *MigrationRunner) addResearchSessionsColumns(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			-- Add state column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'state'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN state VARCHAR(50) NOT NULL DEFAULT 'idle';
			END IF;

			-- Add progress column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'progress'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN progress DECIMAL(5,2) DEFAULT 0.0;
			END IF;

			-- Add current_hypothesis column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'current_hypothesis'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN current_hypothesis TEXT;
			END IF;

			-- Add started_at column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'started_at'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
			END IF;

			-- Add completed_at column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'completed_at'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN completed_at TIMESTAMP WITH TIME ZONE;
			END IF;

			-- Add error_message column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'error_message'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN error_message TEXT;
			END IF;

			-- Add metadata column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'metadata'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN metadata JSONB;
			END IF;

			-- Add created_at column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'created_at'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
			END IF;

			-- Add updated_at column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'updated_at'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();
			END IF;

			-- Add title column if it doesn't exist (for existing schemas that require it)
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'research_sessions' AND column_name = 'title'
			) THEN
				ALTER TABLE research_sessions ADD COLUMN title VARCHAR(255);
			ELSE
				-- If title exists but is NOT NULL, make it nullable or add default
				IF EXISTS (
					SELECT 1 FROM information_schema.columns
					WHERE table_name = 'research_sessions'
					AND column_name = 'title'
					AND is_nullable = 'NO'
				) THEN
					ALTER TABLE research_sessions ALTER COLUMN title DROP NOT NULL;
				END IF;
			END IF;
		END $$;
	`)
	return err
}

func (r *MigrationRunner) createHypothesisResultsTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS hypothesis_results (
			id VARCHAR(50) PRIMARY KEY,
			session_id UUID NOT NULL,
			user_id UUID NOT NULL,
			business_hypothesis TEXT NOT NULL,
			science_hypothesis TEXT NOT NULL,
			null_case TEXT,
			referee_results JSONB,
			tri_gate_result JSONB,
			passed BOOLEAN NOT NULL,
			validation_timestamp TIMESTAMP WITH TIME ZONE,
			standards_version VARCHAR(20),
			execution_metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	return err
}

func (r *MigrationRunner) addHypothesisResultsConstraints(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'hypothesis_results_session_id_fkey'
			) THEN
				ALTER TABLE hypothesis_results
				ADD CONSTRAINT hypothesis_results_session_id_fkey
				FOREIGN KEY (session_id) REFERENCES research_sessions(id) ON DELETE CASCADE;
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'hypothesis_results_user_id_fkey'
			) THEN
				ALTER TABLE hypothesis_results
				ADD CONSTRAINT hypothesis_results_user_id_fkey
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
			END IF;
		END $$;
	`)
	return err
}

func (r *MigrationRunner) createIndexes(ctx context.Context, db *sqlx.DB) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON research_sessions(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_user_state ON research_sessions(user_id, state)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON research_sessions(started_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_hypotheses_user_id ON hypothesis_results(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_hypotheses_session_id ON hypothesis_results(session_id)",
		"CREATE INDEX IF NOT EXISTS idx_hypotheses_user_session ON hypothesis_results(user_id, session_id)",
		"CREATE INDEX IF NOT EXISTS idx_hypotheses_user_created ON hypothesis_results(user_id, created_at DESC)",
		"CREATE INDEX IF NOT EXISTS idx_hypotheses_passed ON hypothesis_results(passed)",
		"CREATE INDEX IF NOT EXISTS idx_hypotheses_created_at ON hypothesis_results(created_at DESC)",
	}

	for _, idxSQL := range indexes {
		if _, err := db.ExecContext(ctx, idxSQL); err != nil {
			// Log but don't fail on index creation errors
			fmt.Printf("Warning: failed to create index: %v\n", err)
		}
	}

	return nil
}

func (r *MigrationRunner) insertDefaultUser(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO users (id, email, username, is_active)
		VALUES ('550e8400-e29b-41d4-a716-446655440000', 'default@grohypo.local', 'default', true)
		ON CONFLICT (email) DO NOTHING
	`)
	if err != nil {
		// Log but don't fail on default user insertion
		fmt.Printf("Warning: failed to insert default user: %v\n", err)
	}
	return nil
}

