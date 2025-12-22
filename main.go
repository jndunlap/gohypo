package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"time"

	"gohypo/adapters/excel"
	"gohypo/adapters/llm"
	"gohypo/ai"
	"gohypo/app"
	"gohypo/internal/analysis/brief"
	"gohypo/internal/config"
	"gohypo/internal/container"
	"gohypo/internal/errors"
	"gohypo/internal/migration"
	"gohypo/internal/research"
	"gohypo/internal/testkit"
	"gohypo/internal/validation"
	"gohypo/models"
	"gohypo/ports"
	"gohypo/ui"

	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

//go:embed ui/templates/** ui/static/*
var embeddedFiles embed.FS

// resetDatabase drops all tables and recreates the database schema
func resetDatabase(db *sqlx.DB) error {
	log.Println("🔄 Resetting database - dropping all tables...")

	// Drop tables in reverse dependency order
	dropTables := []string{
		"workspace_dataset_relations",
		"datasets",
		"workspaces",
		"hypothesis_results",
		"research_prompts",
		"research_sessions",
		"users",
	}

	for _, table := range dropTables {
		_, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if err != nil {
			log.Printf("Warning: failed to drop table %s: %v", table, err)
		}
	}

	log.Println("✅ Database reset complete")
	return nil
}

// initDatabase initializes the PostgreSQL database connection
func initDatabase(appConfig *config.Config) (*sqlx.DB, error) {
	if appConfig.Database.URL == "" {
		return nil, errors.ConfigInvalid("DATABASE_URL is required")
	}

	db, err := sqlx.Connect("postgres", appConfig.Database.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to database")
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, errors.Wrap(err, "failed to ping database")
	}

	// Reset database on each bootup (development mode)
	if err := resetDatabase(db); err != nil {
		return nil, errors.Wrap(err, "database reset failed")
	}

	// Run migrations
	migrator := migration.NewRunner()
	if err := migrator.Run(context.Background(), db); err != nil {
		return nil, errors.Wrap(err, "database migration failed")
	}

	return db, nil
}

func main() {
	// #region agent log
	log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H2","location":"main.go:57","message":"Application starting","data":{},"timestamp":%d}`, time.Now().UnixMilli())
	// #endregion

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Load application configuration
	appConfig, err := config.Load()
	if err != nil {
		// #region agent log
		log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H2","location":"main.go:66","message":"Configuration loading failed","data":{"error":"%s"},"timestamp":%d}`, err.Error(), time.Now().UnixMilli())
		// #endregion
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// #region agent log
	log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H2","location":"main.go:70","message":"Configuration loaded successfully","data":{},"timestamp":%d}`, time.Now().UnixMilli())
	// #endregion

	// Initialize database
	db, err := initDatabase(appConfig)
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Create dependency injection container
	appContainer, err := container.New(appConfig)
	if err != nil {
		log.Fatalf("Failed to create application container: %v", err)
	}
	defer appContainer.Shutdown(context.Background())

	// Initialize container with database
	if err := appContainer.InitWithDatabase(db); err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}

	// Ensure default workspace exists
	if err := appContainer.EnsureDefaultWorkspace(context.Background()); err != nil {
		log.Fatalf("Failed to ensure default workspace exists: %v", err)
	}

	// Configure data source
	var kit *testkit.TestKit
	if appConfig.Data.ExcelFile != "" {
		excelConfig := excel.DefaultExcelConfig()
		excelConfig.FilePath = appConfig.Data.ExcelFile
		excelConfig.Enabled = true

		log.Printf("Using Excel data source: %s", excelConfig.FilePath)

		var err error
		kit, err = testkit.NewTestKitWithExcel(&excelConfig)
		if err != nil {
			log.Fatalf("Failed to initialize test kit: %v", err)
		}
	} else {
		log.Printf("No Excel file configured, using synthetic data for testing")
		var err error
		kit, err = testkit.NewTestKit()
		if err != nil {
			log.Fatalf("Failed to initialize test kit with synthetic data: %v", err)
		}
	}

	// Setup AI services (keeping existing pattern for now)
	aiConfig := &models.AIConfig{
		OpenAIKey:     appConfig.AI.OpenAIKey,
		OpenAIModel:   appConfig.AI.OpenAIModel,
		SystemContext: appConfig.AI.SystemContext,
		MaxTokens:     appConfig.AI.MaxTokens,
		Temperature:   appConfig.AI.Temperature,
		PromptsDir:    appConfig.AI.PromptsDir,
	}

	// Create hypothesis analyzer if AI is available
	var hypothesisAnalyzer *ai.HypothesisAnalysisAgent
	if aiConfig.OpenAIKey != "" && aiConfig.PromptsDir != "" {
		// TODO: Create proper LLM client here
		// For now, we'll create a placeholder
		hypothesisAnalyzer = nil // Will be set when LLM client is available
	}

	var greenfieldService *app.GreenfieldService
	if aiConfig.OpenAIKey != "" && aiConfig.PromptsDir != "" {
		greenfieldService = setupGreenfieldServices(aiConfig, kit.LedgerAdapter(), hypothesisAnalyzer)
		log.Println("Greenfield research service initialized")
	}

	// Initialize research worker using container repositories
	var worker *research.ResearchWorker
	rngPort := kit.RNGAdapter()
	stageRunner := app.NewStageRunner(kit.LedgerAdapter(), rngPort)
	statsSweepService := app.NewStatsSweepService(stageRunner, kit.LedgerAdapter(), rngPort)

	if greenfieldService != nil {
		// Create advanced validation orchestrator
		validationConfig := validation.ValidationConfig{
			MaxComputationalCapacity: 50,               // Allow 50 concurrent computation units
			CapacityTimeout:          5 * time.Minute,  // Wait up to 5 minutes for capacity
			StabilityEnabled:         true,             // Enable stability selection
			SubsampleCount:           10,               // Use 10 subsamples for stability
			SubsampleFraction:        0.8,              // Use 80% of data per subsample
			StabilityThreshold:       0.8,              // Require 80% stability
			LogicalAuditorEnabled:    true,             // Enable logical auditor
			AuditorModel:             "gpt-4o-mini",    // Use efficient model for auditing
			ValidationTimeout:        10 * time.Minute, // Allow 10 minutes per hypothesis
		}

		// Create LLM client for logical auditor (simplified - would use proper LLM adapter)
		llmClient := createLLMClient(aiConfig)

		// Create heuristic auditor for fallback
		statisticalEngine := brief.NewStatisticalEngine()
		heuristicAuditor := validation.NewHeuristicAuditor(statisticalEngine)

		validationOrchestrator := validation.NewValidationOrchestrator(validationConfig, llmClient, heuristicAuditor, aiConfig.PromptsDir)

		worker = research.NewResearchWorker(
			appContainer.SessionManager,
			appContainer.ResearchStorage,
			appContainer.PromptRepo,
			greenfieldService,
			aiConfig,
			statsSweepService,
			kit,
			appContainer.SSEHub, // Pass SSEHub instead of successGateway
			appContainer.UIBroadcaster,
			appContainer.HypothesisAnalyzer,
			appContainer.ValidationEngine,
			appContainer.DynamicSelector,
			appContainer.HypothesisRepo,
			validationOrchestrator,
		)
		worker.StartWorkerPool(2)
		log.Println("Research worker pool initialized")
	}

	// Initialize statistical engine
	statisticalEngine := brief.NewStatisticalEngine()

	// Initialize web server
	server := ui.NewServer(embeddedFiles)
	reader := kit.LedgerReaderAdapter()
	if err := server.Initialize(kit, reader, embeddedFiles, greenfieldService, statisticalEngine, aiConfig, db, appContainer.SSEHub, appContainer.UserRepo, appContainer.HypothesisRepo); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Add research routes using container components
	if worker != nil {
		server.AddResearchRoutes(appContainer.SessionManager, appContainer.ResearchStorage, worker, appContainer.SSEHub, appContainer, appContainer.HypothesisRepo)
		log.Println("Research API routes added with SSE support")
	}

	// Start pprof server for performance profiling
	if appConfig.Profiling.Enabled {
		go func() {
			log.Printf("🚀 Performance profiling server starting on :%s", appConfig.Profiling.Port)
			log.Printf("💡 View profiles: go tool pprof -http=:8081 http://localhost:%s/debug/pprof/profile?seconds=30", appConfig.Profiling.Port)
			if err := http.ListenAndServe(":"+appConfig.Profiling.Port, nil); err != nil {
				log.Printf("❌ pprof server failed: %v", err)
			}
		}()
	}

	// Start the server
	log.Printf("🚀 Starting GoHypo server on port %s", appConfig.Server.Port)
	log.Fatal(server.Start(":" + appConfig.Server.Port))
}

// setupGreenfieldServices creates and configures the greenfield research service
func setupGreenfieldServices(config *models.AIConfig, ledgerPort ports.LedgerPort, hypothesisAnalyzer *ai.HypothesisAnalysisAgent) *app.GreenfieldService {
	greenfieldAdapter := llm.NewGreenfieldAdapter(config)
	return app.NewGreenfieldService(greenfieldAdapter, ledgerPort, hypothesisAnalyzer)
}

// createLLMClient creates an LLM client for validation purposes
// This is a simplified implementation - in production, this would use the full LLM adapter
func createLLMClient(config *models.AIConfig) ports.LLMClient {
	// Placeholder implementation - returns nil if no API key
	// In production, this would create a proper LLM client
	if config.OpenAIKey == "" {
		return nil
	}

	// For now, return nil - the validation orchestrator will handle this gracefully
	// In a full implementation, this would create an OpenAI client or similar
	return nil
}
