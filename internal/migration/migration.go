package migration

import (
	"context"
	"fmt"

	"gohypo/internal/errors"

	"github.com/jmoiron/sqlx"
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

	if err := r.createResearchPromptsTable(ctx, db); err != nil {
		return errors.Wrap(err, "failed to create research_prompts table")
	}

	if err := r.createHypothesisResultsTable(ctx, db); err != nil {
		return errors.Wrap(err, "failed to create hypothesis_results table")
	}

	if err := r.createIndexes(ctx, db); err != nil {
		return errors.Wrap(err, "failed to create indexes")
	}

	if err := r.insertDefaultUser(ctx, db); err != nil {
		return errors.Wrap(err, "failed to insert default user")
	}

	if err := r.runDatasetMigrations(ctx, db); err != nil {
		return errors.Wrap(err, "failed to run dataset migrations")
	}

	if err := r.runWorkspaceBindingMigrations(ctx, db); err != nil {
		return errors.Wrap(err, "failed to run workspace binding migrations")
	}

	if err := r.addHypothesisWorkspaceColumn(ctx, db); err != nil {
		return errors.Wrap(err, "failed to add workspace_id to hypothesis_results")
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
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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

func (r *MigrationRunner) createResearchPromptsTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS research_prompts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			session_id UUID NOT NULL REFERENCES research_sessions(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			prompt_content TEXT NOT NULL,
			prompt_type VARCHAR(50) DEFAULT 'research_directive',
			metadata JSONB,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	return err
}

func (r *MigrationRunner) createHypothesisResultsTable(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS hypothesis_results (
			id VARCHAR(50) PRIMARY KEY,
			session_id UUID NOT NULL REFERENCES research_sessions(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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

func (r *MigrationRunner) createIndexes(ctx context.Context, db *sqlx.DB) error {
	indexes := []string{
		// Research sessions indexes
		"CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON research_sessions(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_user_state ON research_sessions(user_id, state)",
		"CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON research_sessions(started_at DESC)",

		// Research prompts indexes
		"CREATE INDEX IF NOT EXISTS idx_prompts_user_id ON research_prompts(user_id)",
		"CREATE INDEX IF NOT EXISTS idx_prompts_session_id ON research_prompts(session_id)",
		"CREATE INDEX IF NOT EXISTS idx_prompts_user_session ON research_prompts(user_id, session_id)",
		"CREATE INDEX IF NOT EXISTS idx_prompts_type ON research_prompts(prompt_type)",
		"CREATE INDEX IF NOT EXISTS idx_prompts_created_at ON research_prompts(created_at DESC)",

		// Hypothesis results indexes
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

// runWorkspaceBindingMigrations adds workspace relationships to research entities
func (r *MigrationRunner) runWorkspaceBindingMigrations(ctx context.Context, db *sqlx.DB) error {
	// Temporarily simplified for development - workspace relationships handled by application logic
	// TODO: Add proper schema migrations when database schema is stable
	fmt.Println("Workspace binding migrations skipped for development")
	return nil
}

// addHypothesisWorkspaceColumn adds workspace_id column to hypothesis_results table
func (r *MigrationRunner) addHypothesisWorkspaceColumn(ctx context.Context, db *sqlx.DB) error {
	_, err := db.ExecContext(ctx, `
		DO $$
		BEGIN
			-- Add workspace_id column if it doesn't exist
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'hypothesis_results' AND column_name = 'workspace_id'
			) THEN
				ALTER TABLE hypothesis_results ADD COLUMN workspace_id UUID;
				-- Set default workspace for existing hypotheses
				UPDATE hypothesis_results SET workspace_id = '550e8400-e29b-41d4-a716-446655440001'
				WHERE workspace_id IS NULL;
			END IF;
		END $$;
	`)
	if err != nil {
		return err
	}

	fmt.Println("Added workspace_id column to hypothesis_results table")
	return nil
}

// runDatasetMigrations runs the newer dataset and workspace migrations
func (r *MigrationRunner) runDatasetMigrations(ctx context.Context, db *sqlx.DB) error {
	migrations := []string{
		// Migration 002: Research Ledger v2.0 - Evidence tracking and UI synchronization
		`
-- GoHypo Research Ledger v2.0
-- Extends existing schema with evidence tracking, UI synchronization, and scientific auditability

-- Add new columns to existing research_sessions table
ALTER TABLE research_sessions
ADD COLUMN IF NOT EXISTS total_hypotheses INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS completed_hypotheses INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS workspace_id UUID,
ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'idle',
ADD COLUMN IF NOT EXISTS ui_state JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS scientific_efficiency DECIMAL(5,3) DEFAULT 0.0;

-- Add new columns to existing hypothesis_results table
ALTER TABLE hypothesis_results
ADD COLUMN IF NOT EXISTS phase_e_values JSONB DEFAULT '[]'::jsonb,
ADD COLUMN IF NOT EXISTS feasibility_score DECIMAL(3,2) CHECK (feasibility_score >= 0 AND feasibility_score <= 1.0),
ADD COLUMN IF NOT EXISTS risk_level TEXT CHECK (risk_level IN ('low', 'medium', 'high', 'critical')),
ADD COLUMN IF NOT EXISTS data_topology JSONB DEFAULT '{}',
ADD COLUMN IF NOT EXISTS total_validation_time INTERVAL,
ADD COLUMN IF NOT EXISTS phase_completion_times INTERVAL[] DEFAULT ARRAY[]::INTERVAL[];

-- Add check constraints for array lengths
ALTER TABLE hypothesis_results
ADD CONSTRAINT check_phase_times_length CHECK (array_length(phase_completion_times, 1) <= 3);
`,
		// Migration 004: Workspaces for dataset organization
		`
-- Create workspaces table if it doesn't exist
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    color VARCHAR(7) DEFAULT '#3B82F6',
    is_default BOOLEAN DEFAULT false,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create default workspace if it doesn't exist
INSERT INTO workspaces (id, user_id, name, description, is_default, metadata)
VALUES (
    '550e8400-e29b-41d4-a716-446655440001',
    '550e8400-e29b-41d4-a716-446655440000',
    'Default Workspace',
    'Your primary workspace for data analysis and research',
    true,
    '{"auto_discover_relations": true, "max_datasets": 50}'
) ON CONFLICT (id) DO NOTHING;

-- Basic indexes
CREATE INDEX IF NOT EXISTS idx_workspaces_user_id ON workspaces(user_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_user_default ON workspaces(user_id, is_default);
`,
		// Migration 005: Dataset storage with AI-powered naming
		`
-- Migration 005: Dataset storage with AI-powered naming
-- Supports user-uploaded datasets with Forensic Scout AI analysis

-- Datasets table for storing uploaded dataset metadata with AI-generated names
DO $$
BEGIN
    -- Create datasets table if it doesn't exist
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'datasets') THEN
        CREATE TABLE datasets (
            id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
            user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,

            -- File information
            original_filename VARCHAR(255) NOT NULL,
            file_path TEXT,
            file_size BIGINT,
            mime_type VARCHAR(100),

            -- AI-generated naming and context (from Forensic Scout)
            display_name VARCHAR(255),        -- AI-generated descriptive name (3-5 words in snake_case)
            domain VARCHAR(100),             -- AI-detected business domain (1-2 words)
            description TEXT,                -- AI-generated summary/description

            -- Dataset metadata
            record_count INTEGER,
            field_count INTEGER,
            missing_rate DECIMAL(5,4) DEFAULT 0.0, -- 0.0000 to 1.0000
            source VARCHAR(50) DEFAULT 'upload', -- 'upload', 'excel', 'api'

            -- Processing status
            status VARCHAR(50) DEFAULT 'pending', -- pending, processing, ready, failed
            error_message TEXT,

            -- Rich metadata stored as JSONB (fields, samples, AI analysis)
            metadata JSONB,

            created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
            updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
        );
    ELSE
        -- Add workspace_id column if it doesn't exist
        IF NOT EXISTS (
            SELECT 1 FROM information_schema.columns
            WHERE table_name = 'datasets' AND column_name = 'workspace_id'
        ) THEN
            ALTER TABLE datasets ADD COLUMN workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE;
            -- Set default workspace for existing datasets
            UPDATE datasets SET workspace_id = '550e8400-e29b-41d4-a716-446655440001'
            WHERE workspace_id IS NULL;
            -- Make it NOT NULL after setting defaults
            ALTER TABLE datasets ALTER COLUMN workspace_id SET NOT NULL;
        END IF;
    END IF;
END $$;

-- Create indexes if they don't exist (done inside DO block for safety)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'datasets') THEN
        -- Indexes for performance
        CREATE INDEX IF NOT EXISTS idx_datasets_user_id ON datasets(user_id);
        CREATE INDEX IF NOT EXISTS idx_datasets_workspace_id ON datasets(workspace_id);
        CREATE INDEX IF NOT EXISTS idx_datasets_user_workspace ON datasets(user_id, workspace_id);
        CREATE INDEX IF NOT EXISTS idx_datasets_status ON datasets(status);
        CREATE INDEX IF NOT EXISTS idx_datasets_source ON datasets(source);
        CREATE INDEX IF NOT EXISTS idx_datasets_created_at ON datasets(created_at DESC);
        CREATE INDEX IF NOT EXISTS idx_datasets_domain ON datasets(domain);
        CREATE INDEX IF NOT EXISTS idx_datasets_user_created ON datasets(user_id, created_at DESC);
        CREATE INDEX IF NOT EXISTS idx_datasets_workspace_created ON datasets(workspace_id, created_at DESC);

        -- Insert the existing "current" dataset as a special Excel-based record
        -- This maintains backward compatibility with the existing Excel workflow
        INSERT INTO datasets (
            id,
            user_id,
            workspace_id,
            original_filename,
            display_name,
            domain,
            source,
            status,
            description
        ) VALUES (
            '550e8400-e29b-41d4-a716-446655440000', -- Special ID for current dataset
            '550e8400-e29b-41d4-a716-446655440000', -- Default user
            '550e8400-e29b-41d4-a716-446655440001', -- Default workspace
            'current_dataset.xlsx',
            'current_dataset',
            'Data Analysis',
            'excel',
            'ready',
            'Primary dataset loaded from Excel file for analysis'
        ) ON CONFLICT (id) DO NOTHING;
    END IF;
END $$;
`,
		// Migration 006: Workspace Dataset Relations
		`
-- Workspace dataset relations for linking related datasets
CREATE TABLE IF NOT EXISTS workspace_dataset_relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    source_dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    target_dataset_id UUID NOT NULL REFERENCES datasets(id) ON DELETE CASCADE,
    relation_type VARCHAR(100) NOT NULL,
    confidence DECIMAL(3,2) DEFAULT 1.0, -- Confidence score 0.00 to 1.00
    metadata JSONB,
    discovered_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Ensure no self-references and unique relations
    CONSTRAINT no_self_reference CHECK (source_dataset_id != target_dataset_id),
    UNIQUE(workspace_id, source_dataset_id, target_dataset_id, relation_type)
);

-- Indexes for workspace dataset relations
CREATE INDEX IF NOT EXISTS idx_relations_workspace_id ON workspace_dataset_relations(workspace_id);
CREATE INDEX IF NOT EXISTS idx_relations_source_dataset ON workspace_dataset_relations(source_dataset_id);
CREATE INDEX IF NOT EXISTS idx_relations_target_dataset ON workspace_dataset_relations(target_dataset_id);
CREATE INDEX IF NOT EXISTS idx_relations_discovered_at ON workspace_dataset_relations(discovered_at DESC);
`,
		// Migration 003: Add missing hypothesis columns
		`
-- GoHypo Migration 003: Add missing columns to hypothesis_results table
-- Adds columns that were referenced in code but missing from schema

-- Add missing columns to hypothesis_results table
ALTER TABLE hypothesis_results
ADD COLUMN IF NOT EXISTS current_e_value DECIMAL(10,4) DEFAULT 0.0,
ADD COLUMN IF NOT EXISTS normalized_e_value DECIMAL(3,2) CHECK (normalized_e_value >= 0 AND normalized_e_value <= 1.0),
ADD COLUMN IF NOT EXISTS confidence DECIMAL(3,2) CHECK (confidence >= 0 AND confidence <= 1.0),
ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'pending';

-- Update existing records with default values if needed
UPDATE hypothesis_results
SET
    current_e_value = 0.0,
    normalized_e_value = 0.0,
    confidence = 0.0,
    status = 'pending'
WHERE current_e_value IS NULL OR normalized_e_value IS NULL OR confidence IS NULL OR status IS NULL;

-- Add indexes for the new columns
CREATE INDEX IF NOT EXISTS idx_hypotheses_current_e_value ON hypothesis_results(current_e_value);
CREATE INDEX IF NOT EXISTS idx_hypotheses_normalized_e_value ON hypothesis_results(normalized_e_value);
CREATE INDEX IF NOT EXISTS idx_hypotheses_confidence ON hypothesis_results(confidence);
CREATE INDEX IF NOT EXISTS idx_hypotheses_status ON hypothesis_results(status);

-- Add comments for documentation
COMMENT ON COLUMN hypothesis_results.current_e_value IS 'Current evidence value from latest validation';
COMMENT ON COLUMN hypothesis_results.normalized_e_value IS 'Normalized e-value on 0-1 scale for UI display';
COMMENT ON COLUMN hypothesis_results.confidence IS 'Statistical confidence level (0.0 to 1.0)';
COMMENT ON COLUMN hypothesis_results.status IS 'Hypothesis validation status (pending, running, completed, failed)';
`,
		// Migration 007: LLM Usage Tracking
		`
-- LLM usage tracking migration - simplified for development
-- Table creation deferred until schema is stable
SELECT 1; -- No-op migration for now
`,
	}

	for i, migration := range migrations {
		migrationNum := i + 2  // Start from migration 002
		if i == 1 { migrationNum = 3 } // Migration 003: Add missing hypothesis columns
		if i == 2 { migrationNum = 4 } // Migration 004: Workspaces for dataset organization
		if i >= 3 { migrationNum = i + 2 } // Continue normal numbering
		fmt.Printf("Running migration %03d...\n", migrationNum)
		if _, err := db.ExecContext(ctx, migration); err != nil {
			return fmt.Errorf("failed to run migration %03d: %w", migrationNum, err)
		}
		fmt.Printf("Migration %03d completed successfully\n", migrationNum)
	}

	return nil
}
