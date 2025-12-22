package ui

import (
	"io/fs"
	"log"
	"net/http"

	"gohypo/ui/middleware"

	"github.com/gin-gonic/gin"
)

// setupMiddleware configures Gin middleware
func (s *Server) setupMiddleware() {
	// Add workspace middleware to ensure default workspace exists
	if s.workspaceRepository != nil {
		s.router.Use(middleware.EnsureWorkspace(s.workspaceRepository))
	}

	staticFS, err := fs.Sub(s.embeddedFiles, "ui/static")
	if err != nil {
		log.Printf("[setupMiddleware] Error creating static filesystem: %v", err)
		s.router.GET("/static/css/research.css", func(c *gin.Context) {
			log.Printf("[Static] Serving research.css fallback")
			c.Header("Content-Type", "text/css")
			content, err := s.embeddedFiles.ReadFile("ui/static/css/research.css")
			if err != nil {
				log.Printf("[Static] CSS file not found: %v", err)
				c.String(404, "CSS file not found")
				return
			}
			log.Printf("[Static] Served research.css (%d bytes)", len(content))
			c.String(200, string(content))
		})
		s.router.GET("/static/js/research.js", func(c *gin.Context) {
			log.Printf("[Static] Serving research.js fallback")
			c.Header("Content-Type", "application/javascript")
			content, err := s.embeddedFiles.ReadFile("ui/static/js/research.js")
			if err != nil {
				log.Printf("[Static] JS file not found: %v", err)
				c.String(404, "JS file not found")
				return
			}
			log.Printf("[Static] Served research.js (%d bytes)", len(content))
			c.String(200, string(content))
		})
	} else {
		log.Printf("[Static] Serving static files from embedded FS at /static")
		s.router.StaticFS("/static", http.FS(staticFS))
	}
}
