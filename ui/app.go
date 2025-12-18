package ui

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gohypo/internal/testkit"
	"gohypo/ports"
)

//go:embed templates/* static/*
var embeddedFiles embed.FS

// App represents the UI application
type App struct {
	router    *chi.Mux
	testkit   *testkit.TestKit
	reader    ports.LedgerReaderPort
	templates *template.Template
}

// Config holds UI application configuration
type Config struct {
	Port string
}

// NewApp creates a new UI application
func NewApp(config Config) (*App, error) {
	// Initialize test kit for demo data
	kit, err := testkit.NewTestKit()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize test kit: %w", err)
	}

	// Get reader port for artifact access
	reader := kit.LedgerReaderAdapter()

	// Parse templates (including fragments)
	funcMap := template.FuncMap{
		"mul": func(a, b float64) float64 { return a * b },
		"add": func(a, b int) int { return a + b },
		"max": func(a, b float64) float64 {
			if a > b {
				return a
			}
			return b
		},
		"until": func(n int) []int {
			res := make([]int, n)
			for i := range res {
				res[i] = i
			}
			return res
		},
	}
	templates, err := template.New("").Funcs(funcMap).ParseFS(embeddedFiles, "templates/*.html", "templates/fragments/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	app := &App{
		router:    chi.NewRouter(),
		testkit:   kit,
		reader:    reader,
		templates: templates,
	}

	app.setupMiddleware()
	app.setupRoutes()

	return app, nil
}

// setupMiddleware configures HTTP middleware
func (a *App) setupMiddleware() {
	a.router.Use(middleware.Logger)
	a.router.Use(middleware.Recoverer)
	a.router.Use(middleware.Compress(5))

	// Serve static files
	staticFS := http.FileServer(http.FS(embeddedFiles))
	a.router.Handle("/static/", http.StripPrefix("/static/", staticFS))
}

// setupRoutes configures the application routes
func (a *App) setupRoutes() {
	// Main pages
	a.router.Get("/", a.handleIndex)
	a.router.Get("/datasets", a.handleDatasets)
	a.router.Get("/datasets/{id}", a.handleDatasetDetail)
	a.router.Get("/relationships", a.handleRelationships)
	a.router.Get("/hypotheses", a.handleHypotheses)
	a.router.Get("/validation/{id}", a.handleValidation)

	// New Forensic Tactical Interface pages
	a.router.Get("/contracts", a.handleContractSpecification)
	a.router.Get("/ledger", a.handleArtifactLedger)
	a.router.Get("/directives", a.handleResearchDirectiveConsole)
	a.router.Get("/telemetry", a.handleStageProgressView)
	a.router.Get("/vitality", a.handleVariableVitalityMap)
	a.router.Get("/eligibility", a.handleVariableEligibilityReport)

	// API endpoints
	a.router.Post("/api/datasets/upload", a.handleDatasetUpload)
	a.router.Post("/api/datasets/{id}/profile", a.handleDatasetProfile)
	a.router.Post("/api/datasets/{id}/sweep", a.handleDatasetSweep)
	a.router.Get("/api/artifacts", a.handleListArtifacts)
	a.router.Get("/api/artifacts/{id}", a.handleGetArtifact)
	a.router.Post("/api/hypotheses/draft", a.handleHypothesisDraft)
	a.router.Post("/api/hypotheses/{id}/validate", a.handleHypothesisValidate)

	// Contract API endpoints
	a.router.Get("/api/contracts/list", a.handleListContracts)
	a.router.Post("/api/contracts/create", a.handleCreateContract)

	// Artifact Ledger API endpoints
	a.router.Get("/api/artifacts/filtered", a.handleFilteredArtifacts)
	a.router.Get("/api/artifacts/{id}/detail", a.handleArtifactDetail)

	// Directive API endpoints
	a.router.Post("/api/directives/generate", a.handleGenerateHypotheses)

	// Pipeline API endpoints
	a.router.Get("/api/pipeline/status", a.handlePipelineStatus)
	a.router.Get("/api/pipeline/notifications", a.handlePipelineNotifications)
	a.router.Get("/api/pipeline/seals", a.handlePipelineSeals)
	a.router.Post("/api/pipeline/start", a.handleStartPipeline)
	a.router.Post("/api/pipeline/pause", a.handlePausePipeline)
	a.router.Post("/api/pipeline/replay/{id}", a.handleReplayPipeline)

	// HTMX endpoints for dynamic content
	a.router.Get("/api/relationships/table", a.handleRelationshipsTable)
	a.router.Get("/api/hypotheses/list", a.handleHypothesesList)

	// HTMX fragment endpoints
	a.router.Get("/api/fragments/relationships", a.handleFragmentRelationships)
	a.router.Get("/api/fragments/timeline", a.handleFragmentTimeline)
	a.router.Get("/api/fragments/diagnostics", a.handleFragmentDiagnostics)

	// API endpoints for D3.js visualization
	a.router.Get("/api/relationships/json", a.handleRelationshipsJSON)
	a.router.Get("/api/datasets/info", a.handleDatasetsInfo)
	a.router.Get("/api/fields/list", a.handleFieldsList)
}

// Start starts the HTTP server
func (a *App) Start() error {
	port := ":8080"
	log.Printf("Starting GoHypo UI server on %s", port)
	return http.ListenAndServe(port, a.router)
}

// Template helpers
func (a *App) renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html")
	if err := a.templates.ExecuteTemplate(w, templateName, data); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

// HTMX helpers
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func (a *App) renderPartial(w http.ResponseWriter, templateName string, data interface{}) {
	a.renderTemplate(w, templateName, data)
}
