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
	if err := s.templates.ExecuteTemplate(&buf, "fragments/hypothesis_cards_grid.html", data); err != nil {
		log.Printf("[ERROR] Failed to render hypothesis cards template: %v", err)
		return `<div class="text-center py-12 text-red-600">Error rendering hypothesis cards</div>`
	}

	return buf.String()
}


