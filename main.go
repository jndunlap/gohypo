package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"gohypo/adapters/excel"
	"gohypo/adapters/llm"
	"gohypo/adapters/postgres"
	"gohypo/app"
	"gohypo/internal/api"
	"gohypo/internal/research"
	"gohypo/internal/testkit"
	"gohypo/models"
	"gohypo/ports"
	"gohypo/ui"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

//go:embed ui/templates/** ui/static/*
var embeddedFiles embed.FS

// initDatabase initializes the PostgreSQL database connection
func initDatabase() (*sqlx.DB, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}

	db, err := sqlx.Connect("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// runMigrations executes database schema migrations
func runMigrations(db *sqlx.DB) error {
	// Step 1: Create users table first (no dependencies)
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email VARCHAR(255) UNIQUE NOT NULL,
			username VARCHAR(100) UNIQUE,
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Step 2: Create research_sessions table (depends on users)
	_, err = db.Exec(`
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
	if err != nil {
		return fmt.Errorf("failed to create research_sessions table: %w", err)
	}

	// Step 3: Add missing columns to research_sessions if table already exists
	_, err = db.Exec(`
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
	if err != nil {
		fmt.Printf("Warning: failed to add columns to research_sessions: %v\n", err)
	}

	// Step 4: Add foreign key constraint to research_sessions if it doesn't exist
	_, err = db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint 
				WHERE conname = 'research_sessions_user_id_fkey'
			) THEN
				ALTER TABLE research_sessions 
				ADD CONSTRAINT research_sessions_user_id_fkey 
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
			END IF;
		END $$;
	`)
	if err != nil {
		// If DO block fails, try direct ALTER TABLE (might fail if constraint exists)
		_, _ = db.Exec(`ALTER TABLE research_sessions ADD CONSTRAINT research_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`)
	}

	// Step 5: Create hypothesis_results table without foreign keys first
	_, err = db.Exec(`
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
	if err != nil {
		return fmt.Errorf("failed to create hypothesis_results table: %w", err)
	}

	// Step 6: Add foreign key constraints to hypothesis_results if they don't exist
	_, err = db.Exec(`
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
	if err != nil {
		// If DO block fails, try direct ALTER TABLE (might fail if constraints exist)
		_, _ = db.Exec(`ALTER TABLE hypothesis_results ADD CONSTRAINT hypothesis_results_session_id_fkey FOREIGN KEY (session_id) REFERENCES research_sessions(id) ON DELETE CASCADE`)
		_, _ = db.Exec(`ALTER TABLE hypothesis_results ADD CONSTRAINT hypothesis_results_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`)
	}

	// Step 7: Create indexes
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
		if _, err := db.Exec(idxSQL); err != nil {
			// Log but don't fail on index creation errors
			fmt.Printf("Warning: failed to create index: %v\n", err)
		}
	}

	// Step 8: Insert default user
	_, err = db.Exec(`
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

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Configure data source
	var kit *testkit.TestKit
	var err error
	// Check for Excel configuration
	if excelFile := os.Getenv("EXCEL_FILE"); excelFile != "" {
		// Excel data source configuration
		excelConfig := excel.DefaultExcelConfig()
		excelConfig.FilePath = excelFile
		excelConfig.Enabled = true

		log.Printf("Using Excel data source: %s (Sheet1, auto-detected entity column)", excelConfig.FilePath)

		kit, err = testkit.NewTestKitWithExcel(&excelConfig)
		if err != nil {
			log.Fatal("Failed to initialize test kit:", err)
		}
	} else {
		// Default synthetic data
		log.Fatal("Failed to initialize test kit: EXCEL_FILE not set")
	}

	// Get reader port for artifact access
	reader := kit.LedgerReaderAdapter()

	// Setup AI configuration for greenfield services
	aiConfig := &models.AIConfig{
		OpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:   os.Getenv("LLM_MODEL"),
		SystemContext: "You are a statistical research assistant",
		MaxTokens:     80000,
		Temperature:   1,
		PromptsDir:    os.Getenv("PROMPTS_DIR"),
	}

	// Setup greenfield service if AI is configured
	var greenfieldService *app.GreenfieldService
	if aiConfig.OpenAIKey != "" && aiConfig.PromptsDir != "" {
		greenfieldService = setupGreenfieldServices(aiConfig, kit.LedgerAdapter())
		log.Println("Greenfield research service initialized")
	} else {
		log.Println("Greenfield research service not configured (missing OPENAI_API_KEY or PROMPTS_DIR)")
	}

	// Initialize core services
	rngPort := kit.RNGAdapter()
	stageRunner := app.NewStageRunner(kit.LedgerAdapter(), rngPort)

	// Initialize stats sweep service
	statsSweepService := app.NewStatsSweepService(stageRunner, kit.LedgerAdapter(), rngPort)
	log.Println("Stats sweep service initialized")

	// Initialize database
	db, err := initDatabase()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	log.Println("Database connection established")

	// Initialize repositories
	userRepo := postgres.NewUserRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	hypothesisRepo := postgres.NewHypothesisRepository(db)
	promptRepo := postgres.NewPromptRepository(db)

	// Initialize Success-Only Gateway for Layer 3 persistence
	successGateway := research.NewSuccessGateway(hypothesisRepo, userRepo, sessionRepo)

	// Initialize research system with database-backed components
	sessionMgr := research.NewSessionManager(sessionRepo, userRepo)
	storage := research.NewResearchStorage(hypothesisRepo, userRepo)

	// Initialize research worker
	var worker *research.ResearchWorker
	if greenfieldService != nil {
		worker = research.NewResearchWorker(sessionMgr, storage, promptRepo, greenfieldService, aiConfig, statsSweepService, kit, successGateway)
		// Start worker pool
		worker.StartWorkerPool(2)
		log.Println("Research worker pool initialized")
	} else {
		log.Println("Research worker not initialized (missing greenfield service)")
	}

	// Initialize web server
	server := ui.NewServer(embeddedFiles)
	if err := server.Initialize(kit, reader, embeddedFiles, greenfieldService); err != nil {
		log.Fatal("Failed to initialize server:", err)
	}

	// Initialize SSE hub for real-time updates
	sseHub := api.NewSSEHub()
	log.Println("SSE hub initialized for real-time research updates")

	// Add research routes if worker is available
	if worker != nil {
		server.AddResearchRoutes(sessionMgr, storage, worker, sseHub)
		log.Println("Research API routes added with SSE support")
	}

	// Start pprof server for performance profiling
	go func() {
		log.Println("🚀 Performance profiling server starting on :6060")
		log.Println("💡 View profiles: go tool pprof -http=:8081 http://localhost:6060/debug/pprof/profile?seconds=30")
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Printf("❌ pprof server failed: %v", err)
		}
	}()

	// Start the server
	log.Fatal(server.Start(":8080"))
}

// setupGreenfieldServices creates and configures the greenfield research service
func setupGreenfieldServices(config *models.AIConfig, ledgerPort ports.LedgerPort) *app.GreenfieldService {
	// Create the LLM adapter with external prompts
	greenfieldAdapter := llm.NewGreenfieldAdapter(config)

	// Create the service
	greenfieldService := app.NewGreenfieldService(
		greenfieldAdapter, // implements GreenfieldResearchPort
		ledgerPort,        // your existing ledger
	)

	return greenfieldService
}
