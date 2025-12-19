package ui

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"os"
	"sync"
	"time"

	"gohypo/adapters/excel"
	"gohypo/ai"
	"gohypo/internal/analysis/brief"
	"gohypo/internal/testkit"
	"gohypo/models"
	"gohypo/ports"

	"github.com/gin-gonic/gin"
)

type Server struct {
	router            *gin.Engine
	testkit           *testkit.TestKit
	reader            ports.LedgerReaderPort
	templates         *template.Template
	embeddedFiles     embed.FS
	greenfieldService interface{}
	analysisEngine    *brief.StatisticalEngine
	forensicScout     *ai.ForensicScout

	datasetCache     map[string]interface{}
	cacheMutex       sync.RWMutex
	cacheLoaded      bool
	cacheLastUpdated time.Time

	excelDataCache      *excel.ExcelData
	excelColumnTypes    map[string]string
	excelCacheMutex     sync.RWMutex
	excelCacheLoaded    bool
	excelCacheTimestamp time.Time

	currentDatasetFile string // Current dataset file path
}

// NewServer creates a new web server instance
func NewServer(embeddedFiles embed.FS) *Server {
	return &Server{
		router:             gin.Default(),
		embeddedFiles:      embeddedFiles,
		datasetCache:       make(map[string]interface{}),
		cacheLoaded:        false,
		cacheLastUpdated:   time.Now(),
		currentDatasetFile: os.Getenv("EXCEL_FILE"), // Initialize with default
	}
}

func (s *Server) Initialize(kit *testkit.TestKit, reader ports.LedgerReaderPort, embeddedFiles embed.FS, greenfieldService interface{}, analysisEngine *brief.StatisticalEngine, aiConfig *models.AIConfig) error {
	s.testkit = kit
	s.reader = reader
	s.greenfieldService = greenfieldService
	s.analysisEngine = analysisEngine

	// Initialize forensic scout for UI display using the same config as main app
	if aiConfig != nil {
		s.forensicScout = ai.NewForensicScout(aiConfig)
		log.Printf("[Initialize] Forensic scout initialized for UI context display using shared config")
	} else {
		log.Printf("[Initialize] No AI config provided - forensic scout will not be available")
	}

	if err := s.setupTemplates(); err != nil {
		return err
	}

	s.setupMiddleware()
	s.setupRoutes()
	s.startDatasetLoader()

	return nil
}

func (s *Server) setupTemplates() error {
	log.Printf("[setupTemplates] Parsing embedded HTML templates...")

	// Define custom template functions
	funcMap := template.FuncMap{
		"substr": func(s string, start, length int) string {
			if start < 0 {
				start = 0
			}
			if start >= len(s) {
				return ""
			}
			end := start + length
			if end > len(s) {
				end = len(s)
			}
			return s[start:end]
		},
		"mul": func(a, b interface{}) float64 {
			var af, bf float64
			switch v := a.(type) {
			case float64:
				af = v
			case float32:
				af = float64(v)
			case int:
				af = float64(v)
			case int64:
				af = float64(v)
			default:
				return 0
			}
			switch v := b.(type) {
			case float64:
				bf = v
			case float32:
				bf = float64(v)
			case int:
				bf = float64(v)
			case int64:
				bf = float64(v)
			default:
				return 0
			}
			return af * bf
		},
		"multiply": func(a, b interface{}) float64 {
			// Alias for mul
			var af, bf float64
			switch v := a.(type) {
			case float64:
				af = v
			case float32:
				af = float64(v)
			case int:
				af = float64(v)
			case int64:
				af = float64(v)
			default:
				return 0
			}
			switch v := b.(type) {
			case float64:
				bf = v
			case float32:
				bf = float64(v)
			case int:
				bf = float64(v)
			case int64:
				bf = float64(v)
			default:
				return 0
			}
			return af * bf
		},
		"minInt": func(a, b int) int {
			if a < b {
				return a
			}
			return b
		},
		"kfmt": func(n interface{}) string {
			var num float64
			switch v := n.(type) {
			case float64:
				num = v
			case float32:
				num = float64(v)
			case int:
				num = float64(v)
			case int64:
				num = float64(v)
			default:
				return fmt.Sprintf("%v", n)
			}

			if num >= 1000000 {
				return fmt.Sprintf("%.1fM", num/1000000)
			} else if num >= 1000 {
				return fmt.Sprintf("%.1fk", num/1000)
			}
			return fmt.Sprintf("%.0f", num)
		},
	}

	// Create a new template with custom functions
	tmpl := template.New("").Funcs(funcMap)

	// Parse all templates from embedded files
	tmpl, err := tmpl.ParseFS(s.embeddedFiles, "ui/templates/**/*.html")
	if err != nil {
		log.Printf("[setupTemplates] ❌ Failed to parse templates: %v", err)
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	s.templates = tmpl
	log.Printf("[setupTemplates] ✅ Templates parsed successfully")
	return nil
}

func (s *Server) setupRoutes() {
	s.router.GET("/", s.handleIndex)
	s.router.GET("/mission-control", s.handleMissionControl)
	s.router.GET("/api/fields/list", s.handleFieldsList)
	s.router.GET("/api/dataset/status", s.handleDatasetStatus)
	s.router.GET("/api/dataset/info", s.handleDatasetInfo)
	s.router.GET("/api/fields/load-more", s.handleLoadMoreFields)

	// File upload endpoint
	s.router.POST("/api/dataset/upload", s.handleFileUpload)

	// DataSpaces API endpoints
	s.router.GET("/api/datasets/list", s.handleDatasetsList)
	s.router.GET("/api/datasets/:id/fields", s.handleDatasetFields)
	s.router.GET("/api/fields/:name/details", s.handleFieldDetails)
}

func (s *Server) Start(addr string) error {
	log.Printf("Starting GoHypo UI on http://%s", addr)
	log.Printf("[Start] Dataset loader should be running in background - page will show loading state until dataset is ready")
	return s.router.Run(addr)
}
