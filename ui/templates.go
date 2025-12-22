package ui

import (
	"bytes"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

// renderTemplate executes a template with the given data
func (s *Server) renderTemplate(c *gin.Context, templateName string, data interface{}) {
	// First render to a buffer to catch any errors before writing to response
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		log.Printf("Template error for %s: %v", templateName, err)
		log.Printf("Template data type: %T", data)
		if dataMap, ok := data.(map[string]interface{}); ok {
			log.Printf("Template data keys: %v", getMapKeys(dataMap))
		}
		c.AbortWithStatusJSON(500, gin.H{"error": "Template rendering failed", "details": err.Error()})
		return
	}

	// Check if the rendered content looks complete
	content := buf.String()
	if !strings.Contains(content, "</html>") {
		log.Printf("WARNING: Rendered template %s appears truncated - missing </html> tag", templateName)
		log.Printf("Content length: %d, ends with: %s", len(content), content[len(content)-100:])
	}

	// Write the buffer to the response
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(200)
	if _, err := buf.WriteTo(c.Writer); err != nil {
		log.Printf("Error writing template response: %v", err)
	}
}

// Helper function to get map keys for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
