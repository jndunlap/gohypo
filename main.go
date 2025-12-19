package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	_ "net/http/pprof"

	"gohypo/adapters/excel"
	"gohypo/adapters/llm"
	"gohypo/app"
	"gohypo/internal/analysis/brief"
	"gohypo/internal/config"
	"gohypo/internal/container"
	"gohypo/internal/errors"
	"gohypo/internal/migration"
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

	// Run migrations
	migrator := migration.NewRunner()
	if err := migrator.Run(context.Background(), db); err != nil {
		return nil, errors.Wrap(err, "database migration failed")
	}

	return db, nil
}

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Load application configuration
	appConfig, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

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

	// Configure data source (keeping existing pattern for now)
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
		log.Fatal("EXCEL_FILE not configured")
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

	var greenfieldService *app.GreenfieldService
	if aiConfig.OpenAIKey != "" && aiConfig.PromptsDir != "" {
		greenfieldService = setupGreenfieldServices(aiConfig, kit.LedgerAdapter())
		log.Println("Greenfield research service initialized")
	}

	// Initialize research worker using container repositories
	var worker *research.ResearchWorker
	rngPort := kit.RNGAdapter()
	stageRunner := app.NewStageRunner(kit.LedgerAdapter(), rngPort)
	statsSweepService := app.NewStatsSweepService(stageRunner, kit.LedgerAdapter(), rngPort)

	if greenfieldService != nil {
		successGateway := research.NewSuccessGateway(appContainer.HypothesisRepo, appContainer.UserRepo, appContainer.SessionRepo)
		worker = research.NewResearchWorker(
			appContainer.SessionManager,
			appContainer.ResearchStorage,
			appContainer.PromptRepo,
			greenfieldService,
			aiConfig,
			statsSweepService,
			kit,
			successGateway,
		)
		worker.StartWorkerPool(2)
		log.Println("Research worker pool initialized")
	}

	// Initialize statistical engine
	statisticalEngine := brief.NewStatisticalEngine()

	// Initialize web server
	server := ui.NewServer(embeddedFiles)
	reader := kit.LedgerReaderAdapter()
	if err := server.Initialize(kit, reader, embeddedFiles, greenfieldService, statisticalEngine, aiConfig); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Add research routes using container components
	if worker != nil {
		server.AddResearchRoutes(appContainer.SessionManager, appContainer.ResearchStorage, worker, appContainer.SSEHub)
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
func setupGreenfieldServices(config *models.AIConfig, ledgerPort ports.LedgerPort) *app.GreenfieldService {
	greenfieldAdapter := llm.NewGreenfieldAdapter(config)
	return app.NewGreenfieldService(greenfieldAdapter, ledgerPort)
}
