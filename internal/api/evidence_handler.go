package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gohypo/internal/analysis"
	"gohypo/models"
	"gohypo/ports"
)

// EvidenceHandler handles hypothesis evidence requests
type EvidenceHandler struct {
	evidencePackager *analysis.EvidencePackager
	hypothesisRepo   ports.HypothesisRepository
}

// NewEvidenceHandler creates a new evidence handler
func NewEvidenceHandler(
	evidencePackager *analysis.EvidencePackager,
	hypothesisRepo ports.HypothesisRepository,
) *EvidenceHandler {
	return &EvidenceHandler{
		evidencePackager: evidencePackager,
		hypothesisRepo:   hypothesisRepo,
	}
}

// GetHypothesisEvidence returns the raw evidence supporting a hypothesis
func (eh *EvidenceHandler) GetHypothesisEvidence(c *gin.Context) {
	hypothesisID := c.Param("hypothesisId")
	userIDStr := c.GetString("userID")

	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Get the hypothesis
	hypothesis, err := eh.hypothesisRepo.GetHypothesis(c.Request.Context(), userID, hypothesisID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Hypothesis not found"})
		return
	}

	// For now, create mock evidence brief
	// In a real implementation, this would come from the analysis pipeline
	evidenceBrief := eh.createMockEvidenceBrief(hypothesis)

	// Package the evidence for visualization
	evidence := eh.evidencePackager.PackageHypothesisEvidence(
		hypothesis,
		evidenceBrief.Associations,
		evidenceBrief.Breakpoints,
		evidenceBrief.HysteresisEffects,
	)

	c.JSON(http.StatusOK, evidence)
}

// createMockEvidenceBrief creates mock evidence for demonstration
// In production, this would be the actual evidence from the statistical analysis
func (eh *EvidenceHandler) createMockEvidenceBrief(hypothesis *models.HypothesisResult) *analysis.EvidenceBrief {
	// Create mock associations based on hypothesis content
	var associations []analysis.AssociationResult

	// Simple mock - in real implementation, this would be stored with the hypothesis
	mockAssoc := analysis.AssociationResult{
		EvidenceID:            "mock_assoc_001",
		Feature:               "discount_percentage", // Would parse from hypothesis
		Outcome:               "purchase_conversion", // Would parse from hypothesis
		RawEffect:             0.73,
		PValue:                0.001,
		PValueAdj:             0.001,
		Method:                "pearson",
		EffectFamily:          "correlation",
		ConfidenceLevel:       analysis.ConfidenceVeryStrong,
		PracticalSignificance: analysis.SignificanceLarge,
		BusinessFeatureName:   "Discount Percentage",
		BusinessOutcomeName:   "Purchase Conversion",
		ScreeningScore:        0.8,
		Direction:             1,
		NEffective:            95000,
	}

	associations = append(associations, mockAssoc)

	// Create mock breakpoints if applicable
	var breakpoints []analysis.BreakpointResult
	mockBreakpoint := analysis.BreakpointResult{
		EvidenceID:          "mock_bp_001",
		Feature:             "discount_percentage",
		Outcome:             "purchase_conversion",
		Threshold:           25.3,
		EffectBelow:         0.34,
		EffectAbove:         0.12,
		Delta:               -0.22,
		PValue:              0.001,
		PValueAdj:           0.001,
		Method:              "segmented_regression",
		ConfidenceLevel:     analysis.ConfidenceStrong,
		PracticalSignificance: analysis.SignificanceMedium,
		BusinessFeatureName: "Discount Percentage",
		BusinessOutcomeName: "Purchase Conversion",
	}

	breakpoints = append(breakpoints, mockBreakpoint)

	return &analysis.EvidenceBrief{
		Version:         "1.0.0",
		Timestamp:       time.Now(),
		DatasetName:     "customer_transaction_data",
		RowCount:        100000,
		ColumnCount:     20,
		BusinessColumnNames: map[string]string{
			"discount_percentage": "Discount Percentage",
			"purchase_conversion": "Purchase Conversion",
		},
		OutcomeColumn:      "purchase_conversion",
		AllowedVariables:   []string{"discount_percentage", "customer_age", "price"},
		Associations:       associations,
		Breakpoints:        breakpoints,
		Interactions:       []analysis.InteractionResult{},
		StructuralBreaks:   []analysis.StructuralBreakResult{},
		TransferEntropies:  []analysis.TransferEntropyResult{},
		HysteresisEffects:  []analysis.HysteresisResult{},
	}
}
