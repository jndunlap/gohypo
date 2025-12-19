package container

import (
	"context"

	"github.com/jmoiron/sqlx"
	"gohypo/adapters/postgres"
	"gohypo/internal/api"
	"gohypo/internal/config"
	"gohypo/internal/research"
	"gohypo/internal/testkit"
	"gohypo/ports"
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

	// Research components
	SessionManager *research.SessionManager
	ResearchWorker *research.ResearchWorker
	ResearchStorage *research.ResearchStorage
	SSEHub         *api.SSEHub

	// Test infrastructure (temporary - should be moved to proper test setup)
	TestKit *testkit.TestKit
}

// New creates a new dependency injection container
func New(cfg *config.Config) (*Container, error) {
	c := &Container{
		Config: cfg,
	}

	return c, nil
}

// InitWithDatabase initializes components that require database access
func (c *Container) InitWithDatabase(db *sqlx.DB) error {
	c.DB = db

	// Initialize repositories
	if err := c.initRepositories(); err != nil {
		return err
	}

	// Initialize test infrastructure
	if err := c.initTestInfrastructure(); err != nil {
		return err
	}

	// Initialize research components
	if err := c.initResearch(); err != nil {
		return err
	}

	return nil
}

// initRepositories initializes data access repositories
func (c *Container) initRepositories() error {
	c.UserRepo = postgres.NewUserRepository(c.DB)
	c.SessionRepo = postgres.NewSessionRepository(c.DB)
	c.HypothesisRepo = postgres.NewHypothesisRepository(c.DB)
	c.PromptRepo = postgres.NewPromptRepository(c.DB)
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
	c.SessionManager = research.NewSessionManager(c.SessionRepo, c.UserRepo)
	c.ResearchStorage = research.NewResearchStorage(c.HypothesisRepo, c.UserRepo)
	c.SSEHub = api.NewSSEHub()

	// ResearchWorker will be initialized later if AI services are available
	// This maintains the existing conditional initialization pattern

	return nil
}

// Shutdown gracefully shuts down all components
func (c *Container) Shutdown(ctx context.Context) error {
	if c.DB != nil {
		return c.DB.Close()
	}
	return nil
}

