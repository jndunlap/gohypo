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
	}{
		Hypotheses: hypotheses,
	}

	var buf strings.Builder
	if err := s.templates.ExecuteTemplate(&buf, "hypothesis_list_compact", data); err != nil {
		log.Printf("[ERROR] Failed to render hypothesis cards template: %v", err)
		return `<div class="text-center py-12 text-red-600">Error rendering hypothesis cards</div>`
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
