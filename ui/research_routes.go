package ui

import (
	"gohypo/internal/api"
	"gohypo/internal/research"
	"gohypo/ui/services"
)

func (s *Server) AddResearchRoutes(sessionMgr *research.SessionManager, storage *research.ResearchStorage, worker *research.ResearchWorker, sseHub *api.SSEHub) {
	// Initialize services
	dataService := services.NewDataService(s.reader)
	renderService := services.NewRenderService(s.templates)

	// Initialize handlers
	researchHandler := NewResearchHandler(dataService)
	dataHandler := NewDataHandler(renderService)
	industryHandler := NewIndustryHandler(s.greenfieldService)

	// Set up routes
	api := s.router.Group("/api/research")
	{
		api.POST("/initiate", researchHandler.HandleInitiateResearch(sessionMgr, worker, sseHub))
		api.GET("/status", researchHandler.HandleResearchStatus(sessionMgr))
		api.GET("/ledger", dataHandler.HandleResearchLedger(storage))
		api.GET("/download/:id", dataHandler.HandleDownloadHypothesis(storage))
		api.GET("/industry-context", industryHandler.HandleIndustryContext())
		api.GET("/sse", sseHub.HandleSSE) // SSE endpoint for real-time updates
	}
}
