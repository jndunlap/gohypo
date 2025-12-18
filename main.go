package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"

	"gohypo/adapters/excel"
	"gohypo/adapters/llm"
	"gohypo/app"
	"gohypo/internal/research"
	"gohypo/internal/testkit"
	"gohypo/models"
	"gohypo/ports"
	"gohypo/ui"

	"github.com/joho/godotenv"
)

//go:embed ui/templates/** ui/static/*
var embeddedFiles embed.FS

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

	// Initialize research system
	researchDir := filepath.Join(".", "research_output")
	sessionMgr := research.NewSessionManager()
	storage := research.NewResearchStorage(researchDir)

	// Initialize research worker
	var worker *research.ResearchWorker
	if greenfieldService != nil {
		worker = research.NewResearchWorker(sessionMgr, storage, greenfieldService, aiConfig, statsSweepService, kit)
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

	// Add research routes if worker is available
	if worker != nil {
		server.AddResearchRoutes(sessionMgr, storage, worker)
		log.Println("Research API routes added")
	}

	// Start the server
	log.Fatal(server.Start(":8081"))
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
