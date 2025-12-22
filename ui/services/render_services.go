package services

import (
	"html/template"
	"log"
	"strings"

	"gohypo/models"
)

type RenderService struct {
	templates *template.Template
}

func NewRenderService(templates *template.Template) *RenderService {
	return &RenderService{
		templates: templates,
	}
}

func (s *RenderService) RenderHypothesisCards(hypotheses []*models.HypothesisResult) string {
	data := struct {
		Hypotheses []*models.HypothesisResult
		Error      string
	}{
		Hypotheses: hypotheses,
		Error:      "",
	}

	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "hypothesis_list_compact", data); err != nil {
		log.Printf("[ERROR] Failed to render hypothesis cards template: %v", err)
		return `<div class="text-center py-12 text-red-600">Error rendering hypothesis cards</div>`
	}

	return buf.String()
}

func (s *RenderService) RenderHypothesisError(errorMsg string) string {
	data := struct {
		Error string
	}{
		Error: errorMsg,
	}

	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "hypothesis_list_compact", data); err != nil {
		log.Printf("[ERROR] Failed to render hypothesis error template: %v", err)
		return `<div class="text-center py-12 px-6">
                  <div class="w-12 h-12 bg-red-100 rounded-lg flex items-center justify-center mx-auto mb-3">
                    <svg class="w-6 h-6 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L3.732 16.5c-.77.833.192 2.5 1.732 2.5z"></path>
                    </svg>
                  </div>
                  <h3 class="text-sm font-semibold text-gray-900 mb-1">Failed to Load Hypotheses</h3>
                  <p class="text-xs text-gray-600">Unable to retrieve research hypotheses. Please try again.</p>
                </div>`
	}

	return buf.String()
}

func (s *RenderService) RenderHypothesisCard(hypothesis *models.HypothesisResult) string {
	// Wrap single hypothesis in array for grid template
	data := struct {
		Hypotheses []*models.HypothesisResult
	}{
		Hypotheses: []*models.HypothesisResult{hypothesis},
	}

	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "hypothesis_list_compact", data); err != nil {
		log.Printf("[ERROR] Failed to render hypothesis card template: %v", err)
		return `<div class="text-center py-12 text-red-600">Error rendering hypothesis card</div>`
	}

	return buf.String()
}

// RenderHypothesisCardExpanded renders a hypothesis card in expanded state
func (s *RenderService) RenderHypothesisCardExpanded(hypothesis *models.HypothesisResult) string {
	data := struct {
		Hypothesis *models.HypothesisResult
	}{
		Hypothesis: hypothesis,
	}

	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "hypothesis_card_expanded", data); err != nil {
		log.Printf("[ERROR] Failed to render expanded hypothesis card: %v", err)
		return `<div class="text-center py-12 text-red-600">Error rendering expanded card</div>`
	}

	return buf.String()
}

// RenderHypothesisEvidence renders the evidence drawer for a hypothesis
func (s *RenderService) RenderHypothesisEvidence(hypothesis *models.HypothesisResult) string {
	data := struct {
		Hypothesis *models.HypothesisResult
	}{
		Hypothesis: hypothesis,
	}

	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "hypothesis_evidence", data); err != nil {
		log.Printf("[ERROR] Failed to render hypothesis evidence: %v", err)
		return `<div class="text-center py-12 text-red-600">Error rendering evidence</div>`
	}

	return buf.String()
}

