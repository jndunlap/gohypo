package container

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"time"

	"gohypo/adapters/postgres"
	"gohypo/ai"
	"gohypo/domain/core"
	"gohypo/domain/dataset"
	"gohypo/internal/api"
	"gohypo/internal/config"
	"gohypo/internal/referee"
	"gohypo/internal/research"
	"gohypo/internal/testkit"
	"gohypo/ports"

	"github.com/jmoiron/sqlx"
)

// Container holds all application dependencies and manages their lifecycle
type Container struct {
	Config *config.Config

	// Infrastructure
	DB *sqlx.DB

	// Repositories (data access layer)
	UserRepo       ports.UserRepository
	SessionRepo    ports.SessionRepository
	HypothesisRepo ports.HypothesisRepository
	PromptRepo     ports.PromptRepository
	WorkspaceRepo  ports.WorkspaceRepository
	EvidenceRepo   *postgres.EvidenceRepository
	UIStateRepo    *postgres.UIStateRepository

	// Research components
	SessionManager  *research.SessionManager
	ResearchWorker  *research.ResearchWorker
	ResearchStorage *research.ResearchStorage
	SSEHub          *api.SSEHub
	UIBroadcaster   *research.ResearchUIBroadcaster

	// AI and intelligence components
	HypothesisAnalyzer *ai.HypothesisAnalysisAgent

	// Validation components
	EValueCalibrator *referee.EValueCalibrator
	DynamicSelector  *referee.DynamicSelector
	ValidationEngine *referee.ValidationEngine

	// Test infrastructure (temporary - should be moved to proper test setup)
	TestKit *testkit.TestKit
}

// New creates a new dependency injection container
func New(cfg *config.Config) (*Container, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	c := &Container{
		Config: cfg,
	}

	return c, nil
}

// InitWithDatabase initializes components that require database access
func (c *Container) InitWithDatabase(db *sqlx.DB) error {
	if db == nil {
		return fmt.Errorf("database connection cannot be nil")
	}

	c.DB = db

	// Test database connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("database connection test failed: %w", err)
	}

	// Initialize repositories
	if err := c.initRepositories(); err != nil {
		return fmt.Errorf("failed to initialize repositories: %w", err)
	}

	// Initialize test infrastructure
	if err := c.initTestInfrastructure(); err != nil {
		return fmt.Errorf("failed to initialize test infrastructure: %w", err)
	}

	// Initialize research components
	if err := c.initResearch(); err != nil {
		return fmt.Errorf("failed to initialize research components: %w", err)
	}

	log.Printf("Container initialized successfully with database connection")
	return nil
}

// initRepositories initializes data access repositories
func (c *Container) initRepositories() error {
	c.UserRepo = postgres.NewUserRepository(c.DB)
	c.SessionRepo = postgres.NewSessionRepository(c.DB)
	c.HypothesisRepo = postgres.NewHypothesisRepository(c.DB)
	c.PromptRepo = postgres.NewPromptRepository(c.DB)
	c.WorkspaceRepo = postgres.NewWorkspaceRepository(c.DB)
	c.EvidenceRepo = postgres.NewEvidenceRepository(c.DB)
	c.UIStateRepo = postgres.NewUIStateRepository(c.DB)
	return nil
}

// initTestInfrastructure initializes test components
func (c *Container) initTestInfrastructure() error {
	var err error
	c.TestKit, err = testkit.NewTestKit()
	return err
}

// initResearch initializes research-related components
func (c *Container) initResearch() error {
	// Initialize core research components
	c.SessionManager = research.NewSessionManager(c.SessionRepo, c.UserRepo)
	if c.SessionManager == nil {
		return fmt.Errorf("failed to create session manager")
	}

	c.ResearchStorage = research.NewResearchStorage(c.HypothesisRepo, c.UserRepo, c.SessionRepo)
	if c.ResearchStorage == nil {
		return fmt.Errorf("failed to create research storage")
	}

	c.SSEHub = api.NewSSEHub()
	if c.SSEHub == nil {
		return fmt.Errorf("failed to create SSE hub")
	}

	// Connect session manager to SSE hub for validation
	c.SSEHub.SetSessionManager(c.SessionManager)

	log.Printf("Core research components initialized: SessionManager, ResearchStorage, SSEHub")

	// Initialize AI components (require config to be available)
	if c.Config != nil && c.Config.AI.OpenAIKey != "" {
		if err := c.initAIComponents(); err != nil {
			log.Printf("Warning: Failed to initialize AI components: %v", err)
			// Continue without AI components for backward compatibility
		}
	}

	// Initialize validation components
	c.EValueCalibrator = referee.NewEValueCalibrator()
	if c.EValueCalibrator == nil {
		return fmt.Errorf("failed to create E-value calibrator")
	}

	c.DynamicSelector = referee.NewDynamicSelector(c.EValueCalibrator)
	if c.DynamicSelector == nil {
		return fmt.Errorf("failed to create dynamic selector")
	}

	// UI broadcaster will be initialized later with templates

	// Initialize validation engine with UI broadcaster
	c.ValidationEngine = referee.NewValidationEngine(c.UIBroadcaster)
	if c.ValidationEngine == nil {
		return fmt.Errorf("failed to create validation engine")
	}

	// Don't start the validation engine - it's not currently used and causes hanging goroutines
	// c.ValidationEngine.Start()

	// Start validation engine
	c.ValidationEngine.Start()
	log.Printf("Validation engine started with UI broadcaster integration")

	// ResearchWorker will be initialized in main.go after greenfield service is created
	// This maintains the existing conditional initialization pattern

	// TODO: Initialize WorkspaceAssembler once LedgerReaderPort is available
	// c.WorkspaceAssembler = research.NewWorkspaceAssembler(c.LedgerReaderPort, c.WorkspaceRepo, nil)

	log.Printf("Research components initialization completed successfully")
	return nil
}

// initAIComponents initializes AI and machine learning components
func (c *Container) initAIComponents() error {
	// Check if AI configuration is available
	if c.Config.AI.OpenAIKey == "" {
		return fmt.Errorf("AI configuration not available - OpenAI key required")
	}

	// TODO: Initialize proper LLM client
	// For now, create placeholder - will need actual LLM client implementation
	// llmClient := adapters.NewLLMClient(c.Config.AI)
	// c.HypothesisAnalyzer = ai.NewHypothesisAnalysisAgent(llmClient, c.Config.AI.PromptsDir)

	// Create placeholder HypothesisAnalyzer - will be replaced with real implementation
	// This allows the system to start without AI while maintaining the interface
	c.HypothesisAnalyzer = nil // TODO: Initialize with actual LLM client

	log.Printf("AI components placeholder initialized - HypothesisAnalyzer needs LLM client implementation")
	return nil
}

// EnsureDefaultWorkspace creates a default workspace for the default user if one doesn't exist
func (c *Container) EnsureDefaultWorkspace(ctx context.Context) error {
	// Default user and workspace IDs from migration
	defaultUserID := core.ID("550e8400-e29b-41d4-a716-446655440000")
	defaultWorkspaceID := core.ID("550e8400-e29b-41d4-a716-446655440001")

	// Check if default workspace exists
	workspace, err := c.WorkspaceRepo.GetByID(ctx, defaultWorkspaceID)
	if err == nil && workspace != nil {
		// Workspace exists
		return nil
	}

	// Create default workspace
	defaultWorkspace := &dataset.Workspace{
		ID:          defaultWorkspaceID,
		UserID:      defaultUserID,
		Name:        "Default Workspace",
		Description: "Your primary workspace for data analysis and research",
		Color:       "#3B82F6",
		IsDefault:   true,
		Metadata: map[string]interface{}{
			"auto_discover_relations": true,
			"max_datasets":            50,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := c.WorkspaceRepo.Create(ctx, defaultWorkspace); err != nil {
		return fmt.Errorf("failed to create default workspace: %w", err)
	}

	log.Printf("Created default workspace: %s", defaultWorkspaceID)
	return nil
}

// InitializeUIBroadcaster initializes the UI broadcaster with templates
func (c *Container) InitializeUIBroadcaster(templates *template.Template) error {
	if c.SSEHub == nil {
		return fmt.Errorf("SSEHub not initialized")
	}
	if c.SessionManager == nil {
		return fmt.Errorf("SessionManager not initialized")
	}

	c.UIBroadcaster = research.NewResearchUIBroadcaster(c.SSEHub, c.SessionManager, templates)
	if c.UIBroadcaster == nil {
		return fmt.Errorf("failed to create UI broadcaster")
	}

	log.Printf("UI broadcaster initialized with templates")
	return nil
}

// Shutdown gracefully shuts down all components
func (c *Container) Shutdown(ctx context.Context) error {
	// Stop validation engine
	if c.ValidationEngine != nil {
		c.ValidationEngine.Stop()
	}

	// Close database connection
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}


