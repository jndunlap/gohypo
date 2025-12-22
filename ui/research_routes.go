package ui

import (
	"html/template"
	"log"

	"gohypo/internal/api"
	"gohypo/internal/research"
	"gohypo/ui/services"
)

func (s *Server) AddResearchRoutes(sessionMgr *research.SessionManager, storage *research.ResearchStorage, worker *research.ResearchWorker, sseHub *api.SSEHub, appContainer interface{}) {
	// Set research components on server
	s.researchStorage = storage
	s.renderService = services.NewRenderService(s.templates)

	// Initialize services
	dataService := services.NewDataService(s.reader, s.datasetRepository)
	renderService := s.renderService

	// Initialize handlers
	researchHandler := NewResearchHandler(dataService)
	dataHandler := NewDataHandler(renderService)
	industryHandler := NewIndustryHandler(s.greenfieldService)

	// Initialize UI broadcaster with templates if container supports it
	if container, ok := appContainer.(interface {
		InitializeUIBroadcaster(*template.Template) error
	}); ok {
		if err := container.InitializeUIBroadcaster(s.templates); err != nil {
			log.Printf("Warning: Failed to initialize UI broadcaster with templates: %v", err)
		}
	}

	// Set up routes
	api := s.router.Group("/api")
	{
		// Research endpoints
		research := api.Group("/research")
		{
			research.POST("/initiate", researchHandler.HandleInitiateResearch(sessionMgr, worker, sseHub))
			research.GET("/status", researchHandler.HandleResearchStatus(sessionMgr))
			research.GET("/ledger", dataHandler.HandleResearchLedger(storage))
			research.GET("/download/:id", dataHandler.HandleDownloadHypothesis(storage))
			research.GET("/industry-context", industryHandler.HandleIndustryContext())
			research.GET("/sse", sseHub.HandleSSE) // SSE endpoint for real-time updates
		}

		// Hypothesis management endpoints
		api.GET("/hypothesis/:id", dataHandler.HandleHypothesisCard(storage))
		api.GET("/hypothesis/:id/toggle", dataHandler.HandleHypothesisToggle(storage))
		api.GET("/hypothesis/:id/evidence", dataHandler.HandleHypothesisEvidence(storage))
	}
}
