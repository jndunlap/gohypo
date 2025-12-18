package ui

import (
	"net/http"
)

// handleFragmentRelationships returns relationships table fragment for HTMX
func (a *App) handleFragmentRelationships(w http.ResponseWriter, r *http.Request) {
	// TODO: Move implementation from handlers.go
}

// handleFragmentTimeline returns evidence timeline fragment for HTMX
func (a *App) handleFragmentTimeline(w http.ResponseWriter, r *http.Request) {
	// TODO: Move implementation from handlers.go
}

// handleFragmentDiagnostics returns a "Diagnostic Mode" panel for cold-start debugging.
// It surfaces:
// - variable-level profile stats (missingness/variance/cardinality)
// - counts of skipped pairwise tests by reason (LOW_N/HIGH_MISSING/LOW_VARIANCE)
// - the latest sweep manifest, if present
func (a *App) handleFragmentDiagnostics(w http.ResponseWriter, r *http.Request) {
	// TODO: Move implementation from handlers.go
}
