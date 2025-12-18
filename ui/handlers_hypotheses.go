package ui

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gohypo/domain/core"
	"gohypo/ports"
)

// handleHypotheses renders the hypotheses screen
func (a *App) handleHypotheses(w http.ResponseWriter, r *http.Request) {
	// Get hypothesis artifacts
	kind := core.ArtifactHypothesis
	filters := ports.ArtifactFilters{
		Kind:  &kind,
		Limit: 100,
	}

	artifacts, err := a.reader.ListArtifacts(r.Context(), filters)
	if err != nil {
		http.Error(w, "Failed to load hypotheses", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":     "Hypotheses - GoHypo",
		"Artifacts": artifacts,
	}
	a.renderTemplate(w, "hypotheses.html", data)
}

// handleHypothesesList returns hypotheses list for HTMX
func (a *App) handleHypothesesList(w http.ResponseWriter, r *http.Request) {
	// Get hypothesis artifacts
	kind := core.ArtifactHypothesis
	filters := ports.ArtifactFilters{
		Kind:  &kind,
		Limit: 100,
	}

	// Apply status filter from query params
	if status := r.URL.Query().Get("status"); status != "" {
		// Filter would be applied here in real implementation
	}

	artifacts, err := a.reader.ListArtifacts(r.Context(), filters)
	if err != nil {
		http.Error(w, "Failed to load hypotheses", http.StatusInternalServerError)
		return
	}

	// Render just the hypotheses list
	data := map[string]interface{}{
		"Artifacts": artifacts,
	}
	a.renderPartial(w, "hypotheses_list.html", data)
}

// handleHypothesisDraft generates hypothesis drafts
func (a *App) handleHypothesisDraft(w http.ResponseWriter, r *http.Request) {
	// Parse relationship IDs from form (for future use)
	_ = r.FormValue("relationship_ids")

	// In real implementation, this would call LLM service
	// For MVP, simulate hypothesis generation

	hypotheses := []map[string]interface{}{
		{
			"cause_key":          "inspection_count",
			"effect_key":         "severity_score",
			"mechanism_category": "direct_causal",
			"rationale":          "Strong correlation suggests inspection frequency directly influences severity detection",
			"confounders":        []string{"facility_size", "regulatory_focus"},
		},
	}

	if isHTMX(r) {
		data := map[string]interface{}{
			"Hypotheses": hypotheses,
		}
		a.renderPartial(w, "hypothesis_drafts.html", data)
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(hypotheses)
	}
}

// handleHypothesisValidate validates a hypothesis
func (a *App) handleHypothesisValidate(w http.ResponseWriter, r *http.Request) {
	hypothesisID := chi.URLParam(r, "id")

	// In real implementation, this would call validation service
	// For MVP, simulate validation results

	validation := map[string]interface{}{
		"hypothesis_id": hypothesisID,
		"verdict":       "supported",
		"checklist": map[string]interface{}{
			"phantom_gate":             true,
			"confounder_stress":        true,
			"conditional_independence": true,
			"nested_model_result":      0.15,
			"stability_score":          0.85,
		},
		"evidence": []string{
			"Sample size adequate (n=12,847)",
			"Effect size stable across subgroups",
			"No significant confounding detected",
		},
	}

	if isHTMX(r) {
		data := validation
		a.renderPartial(w, "validation_results.html", data)
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(validation)
	}
}
