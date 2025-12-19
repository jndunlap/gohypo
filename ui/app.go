package ui

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gohypo/domain/core"
	"gohypo/internal/testkit"
	"gohypo/ports"
)

//go:embed templates/** static/**
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
		// Back-compat: some templates use `multiply` rather than `mul`.
		"multiply": func(a, b float64) float64 { return a * b },
		// Format sample sizes as e.g. "12k" for 12000.
		// Accepts int/int64/float64 to tolerate JSON + Go struct inputs.
		"kfmt": func(v interface{}) string {
			switch t := v.(type) {
			case int:
				return fmt.Sprintf("%dk", t/1000)
			case int64:
				return fmt.Sprintf("%dk", t/1000)
			case float64:
				return fmt.Sprintf("%dk", int64(t/1000))
			case float32:
				return fmt.Sprintf("%dk", int64(float64(t)/1000))
			default:
				return "—"
			}
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b float64) float64 { return a - b },
		"minInt": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"max": func(a, b float64) float64 {
			if a > b {
				return a
			}
			return b
		},
		"upper": strings.ToUpper,
		"until": func(n int) []int {
			res := make([]int, n)
			for i := range res {
				res[i] = i
			}
			return res
		},
	}
	templates, err := template.New("").Funcs(funcMap).ParseFS(embeddedFiles, "templates/*.html")
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
	// Blueprint Dashboard - Single Page Application
	a.router.Get("/", a.handleIndex)

	// API endpoints for blueprint dashboard data
	a.router.Get("/api/fields/list", a.handleFieldsList)
}

// handleFieldsList returns all unique fields/variables
func (a *App) handleFieldsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get all artifacts to extract unique variables
	allArtifacts, err := a.reader.ListArtifacts(r.Context(), ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		http.Error(w, "Failed to load artifacts", http.StatusInternalServerError)
		return
	}

	fieldSet := make(map[string]bool)
	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok && vx != "" {
					fieldSet[vx] = true
				}
				if vy, ok := payload["variable_y"].(string); ok && vy != "" {
					fieldSet[vy] = true
				}
			}
		} else if artifact.Kind == core.ArtifactVariableProfile {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vk, ok := payload["variable_key"].(string); ok && vk != "" {
					fieldSet[vk] = true
				}
			}
		}
	}

	fields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		fields = append(fields, field)
	}

	a.renderJSON(w, map[string]interface{}{
		"fields": fields,
		"count":  len(fields),
	})
}

// Start starts the HTTP server
func (a *App) Start() error {
	port := ":8081"
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

// JSON helpers
func (a *App) renderJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("JSON encoding error: %v", err)
		http.Error(w, "JSON encoding error", http.StatusInternalServerError)
	}
}
