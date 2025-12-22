package ui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"gohypo/adapters/postgres"
	"gohypo/ai"
	"gohypo/domain/core"
	domainDataset "gohypo/domain/dataset"
	"gohypo/internal/analysis"
	"gohypo/internal/analysis/brief"
	"gohypo/internal/api"
	"gohypo/internal/dataset"
	"gohypo/internal/research"
	"gohypo/internal/testkit"
	"gohypo/models"
	"gohypo/ports"
	"gohypo/ui/services"

	"github.com/gin-gonic/gin"
	"github.com/gomarkdown/markdown"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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

	// New dataset processing components
	datasetRepository   ports.DatasetRepository
	workspaceRepository ports.WorkspaceRepository
	userRepository      ports.UserRepository
	datasetProcessor    *dataset.Processor
	sseHub              *api.SSEHub

	// Research components
	researchStorage *research.ResearchStorage
	renderService   *services.RenderService
	hypothesisRepo  ports.HypothesisRepository

	// Evidence components
	evidenceHandler *api.EvidenceHandler

	datasetCache        map[string]interface{}
	cacheMutex          sync.RWMutex
	cacheLoaded         bool
	cacheLastUpdated    time.Time
	excelCacheTimestamp time.Time

	currentDatasetFile string // Current dataset file path
}

// NewServer creates a new web server instance
func NewServer(embeddedFiles embed.FS) *Server {
	return &Server{
		router:           gin.Default(),
		embeddedFiles:    embeddedFiles,
		datasetCache:     make(map[string]interface{}),
		cacheLoaded:      false,
		cacheLastUpdated: time.Now(),
	}
}

// getDefaultUserID returns the default user ID for single-user mode
func (s *Server) getDefaultUserID(ctx context.Context) (core.ID, error) {
	if s.userRepository == nil {
		// Fallback to hardcoded ID if no user repository is available
		return core.ID("550e8400-e29b-41d4-a716-446655440000"), nil
	}

	user, err := s.userRepository.GetOrCreateDefaultUser(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get default user: %w", err)
	}

	return core.ID(user.ID.String()), nil
}

// validateWorkspaceOwnership checks if the workspace exists and belongs to the user
func (s *Server) validateWorkspaceOwnership(ctx context.Context, workspaceID core.ID, userID core.ID) error {
	workspace, err := s.workspaceRepository.GetByID(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found")
	}

	if workspace.UserID != userID {
		return fmt.Errorf("access denied")
	}

	return nil
}

func (s *Server) Initialize(kit *testkit.TestKit, reader ports.LedgerReaderPort, embeddedFiles embed.FS, greenfieldService interface{}, analysisEngine *brief.StatisticalEngine, aiConfig *models.AIConfig, db *sqlx.DB, sseHub *api.SSEHub, userRepo ports.UserRepository, hypothesisRepo ports.HypothesisRepository) error {
	s.sseHub = sseHub
	s.testkit = kit
	s.reader = reader
	s.greenfieldService = greenfieldService
	s.analysisEngine = analysisEngine
	s.userRepository = userRepo
	s.hypothesisRepo = hypothesisRepo

	// Initialize evidence handler
	evidencePackager := analysis.NewEvidencePackager()
	s.evidenceHandler = api.NewEvidenceHandler(evidencePackager, hypothesisRepo)

	// Initialize forensic scout for UI display using the same config as main app
	if aiConfig != nil {
		s.forensicScout = ai.NewForensicScout(aiConfig)
		log.Printf("[Initialize] Forensic scout initialized for UI context display using shared config")
	} else {
		log.Printf("[Initialize] No AI config provided - forensic scout will not be available")
	}

	// Initialize dataset and workspace components
	if db != nil {
		s.datasetRepository = postgres.NewDatasetRepository(db)
		s.workspaceRepository = postgres.NewWorkspaceRepository(db)

		// Initialize file storage with cloud-ready configuration
		storageConfig := dataset.DefaultStorageConfig()
		// Override with environment-specific settings if needed
		if basePath := os.Getenv("DATASET_STORAGE_PATH"); basePath != "" {
			storageConfig.BasePath = basePath
		}
		if maxSize := os.Getenv("DATASET_MAX_SIZE_MB"); maxSize != "" {
			if size, err := strconv.ParseInt(maxSize, 10, 64); err == nil {
				storageConfig.MaxFileSize = size * 1024 * 1024
			}
		}
		fileStorage := dataset.NewLocalFileStorage(storageConfig)

		// Initialize dataset processor with forensic scout and SSE hub
		if s.forensicScout != nil && sseHub != nil && s.workspaceRepository != nil {
			s.datasetProcessor = dataset.NewProcessorWithConfig(s.forensicScout, s.datasetRepository, s.workspaceRepository, fileStorage, sseHub, db, storageConfig)
			log.Printf("[Initialize] Dataset processor initialized with Forensic Scout, SSE, and merge capabilities (max file size: %d MB)", storageConfig.MaxFileSize/(1024*1024))
		} else {
			log.Printf("[Initialize] Required dependencies not available - dataset processing will be limited")
		}

		// Ensure default workspace exists for the default user
		defaultUserID := core.ID("550e8400-e29b-41d4-a716-446655440000")
		if _, err := s.ensureDefaultWorkspace(context.Background(), defaultUserID); err != nil {
			log.Printf("[Initialize] Warning: Failed to ensure default workspace: %v", err)
		}
	} else {
		log.Printf("[Initialize] No database provided - dataset processing will not be available")
	}

	if err := s.setupTemplates(); err != nil {
		return err
	}

	s.setupMiddleware()
	s.setupRoutes()
	// Datasets should only be loaded via UI upload - no automatic loading

	return nil
}

func (s *Server) setupTemplates() error {
	// #region agent log
	log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H1","location":"ui/server.go:87","message":"Starting template setup","data":{},"timestamp":%d}`, time.Now().UnixMilli())
	// #endregion

	// Check what files are being parsed
	// #region agent log
	log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H1","location":"ui/server.go:89","message":"Template patterns to parse","data":{"patterns":["ui/templates/*.html","ui/templates/fragments/**/*.html"]},"timestamp":%d}`, time.Now().UnixMilli())
	// #endregion

	log.Printf("[setupTemplates] Parsing embedded HTML templates...")

	// Define custom template functions
	funcMap := template.FuncMap{
		"dict": func(values ...interface{}) map[string]interface{} {
			if len(values)%2 != 0 {
				panic("dict: odd number of arguments")
			}
			dict := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					panic("dict: keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict
		},
		"or": func(a, b interface{}) interface{} {
			if a != nil && a != false && a != "" {
				return a
			}
			return b
		},
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
		"upper": func(s string) string {
			return strings.ToUpper(s)
		},
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"formatTime": func(t interface{}) string {
			var timestamp time.Time
			switch v := t.(type) {
			case time.Time:
				timestamp = v
			case int64:
				timestamp = time.Unix(v/1000, (v%1000)*1000000) // Handle milliseconds
			case string:
				// Try parsing as RFC3339 first, then as Unix timestamp
				if parsed, err := time.Parse(time.RFC3339, v); err == nil {
					timestamp = parsed
				} else if parsed, err := time.Parse("2006-01-02T15:04:05Z07:00", v); err == nil {
					timestamp = parsed
				} else {
					return v // Return as-is if can't parse
				}
			default:
				return fmt.Sprintf("%v", t)
			}
			return timestamp.Format("Jan 2, 15:04")
		},
		"jsonPretty": func(data interface{}) string {
			if data == nil {
				return "{}"
			}
			// Marshal to JSON with indentation
			jsonBytes, err := json.MarshalIndent(data, "", "  ")
			if err != nil {
				return fmt.Sprintf("Error formatting JSON: %v", err)
			}
			return string(jsonBytes)
		},
		"div": func(a, b interface{}) float64 {
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
			if bf == 0 {
				return 0 // Avoid division by zero
			}
			return af / bf
		},
		"markdown": func(text string) template.HTML {
			if text == "" {
				return ""
			}
			html := markdown.ToHTML([]byte(text), nil, nil)
			return template.HTML(html)
		},

		// JSON encoding for template data
		"toJson": func(v interface{}) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},

		// Safe HTML output (use with caution)
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// Create a new template with custom functions
	tmpl := template.New("").Funcs(funcMap)

	// #region agent log
	log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H1","location":"ui/server.go:198","message":"About to parse templates","data":{"templateCount":%d},"timestamp":%d}`, len(funcMap), time.Now().UnixMilli())
	// #endregion

	// Parse all templates from embedded files (recursive for fragments)
	tmpl, err := tmpl.ParseFS(s.embeddedFiles,
		"ui/templates/*.html",
		"ui/templates/fragments/**/*.html")
	if err != nil {
		// #region agent log
		log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H1","location":"ui/server.go:201","message":"Template parsing failed","data":{"error":"%s"},"timestamp":%d}`, err.Error(), time.Now().UnixMilli())
		// #endregion
		log.Printf("[setupTemplates] ❌ Failed to parse templates: %v", err)
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	s.templates = tmpl

	// #region agent log
	log.Printf(`{"sessionId":"debug-session","runId":"initial","hypothesisId":"H1","location":"ui/server.go:207","message":"Templates parsed successfully","data":{},"timestamp":%d}`, time.Now().UnixMilli())
	// #endregion

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

	// Workspace API endpoints
	s.router.GET("/api/workspaces", s.handleGetWorkspaces)
	s.router.POST("/api/workspaces", s.handleCreateWorkspace)
	s.router.GET("/api/workspaces/:id", s.handleGetWorkspace)
	s.router.PUT("/api/workspaces/:id", s.handleUpdateWorkspace)
	s.router.DELETE("/api/workspaces/:id", s.handleDeleteWorkspace)
	s.router.GET("/api/workspaces/:id/datasets", s.handleGetWorkspaceDatasets)

	// Dataset API endpoints
	s.router.GET("/api/datasets/list", s.handleDatasetsList)
	s.router.GET("/api/datasets/:id", s.handleGetDataset)
	s.router.GET("/api/datasets/:id/fields", s.handleDatasetFields)
	s.router.GET("/api/datasets/:id/preview", s.handleDatasetPreview)
	s.router.GET("/api/fields/:name/details", s.handleFieldDetails)

	// Dataset relationships and discovery
	s.router.GET("/api/workspaces/:id/relations", s.handleGetWorkspaceRelations)
	s.router.GET("/api/workspaces/:id/relationships", s.handleGetWorkspaceRelationships)
	s.router.GET("/api/workspaces/:id/hypotheses", s.handleGetWorkspaceHypotheses)
	s.router.POST("/api/workspaces/:id/discover", s.handleDiscoverRelationships)
	s.router.POST("/api/workspaces/:id/auto-merge", s.handleAutoMergeSuggestions)

	// Manifold visualization endpoints
	s.router.GET("/api/hypotheses/:hypothesisId/manifold", s.handleGetHypothesisManifold)
	s.router.GET("/api/hypotheses/:hypothesisId/evidence", s.handleGetHypothesisEvidence)

	// Dataset merging
	s.router.POST("/api/datasets/merge", s.handleMergeDatasets)
	s.router.GET("/api/datasets/merge/:id/status", s.handleMergeStatus)
}

// Manifold visualization handler
func (s *Server) handleGetHypothesisManifold(c *gin.Context) {
	hypothesisID := c.Param("hypothesisId")
	userIDStr := c.GetString("userID")

	if userIDStr == "" {
		c.JSON(401, gin.H{"error": "Unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get hypothesis from repository
	hypothesis, err := s.hypothesisRepo.GetHypothesis(c.Request.Context(), userID, hypothesisID)
	if err != nil {
		log.Printf("Failed to get hypothesis %s: %v", hypothesisID, err)
		c.JSON(404, gin.H{"error": "Hypothesis not found"})
		return
	}

	// Get evidence brief (this would need to be implemented)
	// For now, we'll create mock evidence based on the hypothesis
	evidence := s.createMockEvidenceFromHypothesis(hypothesis)

	// Analyze manifold
	analyzer := analysis.NewManifoldAnalyzer()
	topologicalData := analyzer.AnalyzeHypothesis(hypothesis, evidence)

	c.JSON(200, topologicalData)
}

// handleGetHypothesisEvidence returns raw evidence for a hypothesis
func (s *Server) handleGetHypothesisEvidence(c *gin.Context) {
	s.evidenceHandler.GetHypothesisEvidence(c)
}

// createMockEvidenceFromHypothesis creates mock evidence for demonstration
// In a real implementation, this would retrieve the actual evidence used for the hypothesis
func (s *Server) createMockEvidenceFromHypothesis(hypothesis *models.HypothesisResult) *analysis.EvidenceBrief {
	// Create mock associations
	associations := []analysis.AssociationResult{
		{
			EvidenceID:            "mock_assoc_001",
			Feature:               "price",
			Outcome:               "purchase_conversion",
			RawEffect:             0.35,
			PValue:                0.02,
			Method:                "pearson",
			ConfidenceLevel:       analysis.ConfidenceStrong,
			PracticalSignificance: analysis.SignificanceMedium,
			BusinessFeatureName:   "Product Price",
			BusinessOutcomeName:   "Purchase Conversion Rate",
		},
	}

	// Create mock breakpoints
	breakpoints := []analysis.BreakpointResult{
		{
			EvidenceID:            "mock_bp_001",
			Feature:               "discount_percentage",
			Outcome:               "purchase_conversion",
			Threshold:             15.0,
			EffectBelow:           0.25,
			EffectAbove:           0.45,
			Delta:                 0.20,
			PValue:                0.01,
			Method:                "segmented_regression",
			ConfidenceLevel:       analysis.ConfidenceStrong,
			PracticalSignificance: analysis.SignificanceMedium,
			BusinessFeatureName:   "Discount Percentage",
			BusinessOutcomeName:   "Purchase Conversion Rate",
		},
	}

	return &analysis.EvidenceBrief{
		Version:           "1.0.0",
		Timestamp:         time.Now(),
		DatasetName:       "customer_transaction_data",
		RowCount:          100000,
		ColumnCount:       20,
		Associations:      associations,
		Breakpoints:       breakpoints,
		Interactions:      []analysis.InteractionResult{},
		StructuralBreaks:  []analysis.StructuralBreakResult{},
		TransferEntropies: []analysis.TransferEntropyResult{},
		HysteresisEffects: []analysis.HysteresisResult{},
	}
}

// ensureDefaultWorkspace ensures a default workspace exists for the given user
func (s *Server) ensureDefaultWorkspace(ctx context.Context, userID core.ID) (*domainDataset.Workspace, error) {
	// First try to get existing default workspace
	defaultWorkspace, err := s.workspaceRepository.GetDefaultForUser(ctx, userID)
	if err == nil {
		// Default workspace already exists
		return defaultWorkspace, nil
	}

	// Create default workspace
	defaultWorkspace = domainDataset.NewDefaultWorkspace(userID)

	// Generate a unique ID for the default workspace based on user ID
	// This ensures each user gets their own default workspace
	defaultWorkspace.ID = core.NewID()

	if err := s.workspaceRepository.Create(ctx, defaultWorkspace); err != nil {
		return nil, fmt.Errorf("failed to create default workspace: %w", err)
	}

	log.Printf("[ensureDefaultWorkspace] Created default workspace %s for user: %s", defaultWorkspace.ID, userID)
	return defaultWorkspace, nil
}

func (s *Server) Start(addr string) error {
	log.Printf("Starting GoHypo UI on http://%s", addr)
	log.Printf("[Start] Dataset loader should be running in background - page will show loading state until dataset is ready")
	return s.router.Run(addr)
}
