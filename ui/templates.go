package ui

import (
	"log"

	"github.com/gin-gonic/gin"
)

// renderTemplate executes a template with the given data
func (s *Server) renderTemplate(c *gin.Context, templateName string, data interface{}) {
	if err := s.templates.ExecuteTemplate(c.Writer, templateName, data); err != nil {
		log.Printf("Template error: %v", err)
		c.AbortWithStatusJSON(500, gin.H{"error": "Template rendering failed"})
	}
}
