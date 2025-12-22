package analysis

import (
	"fmt"
	"strings"

	"gohypo/models"
)

// EvidencePackager extracts raw statistical evidence for visualization
type EvidencePackager struct{}

// NewEvidencePackager creates a new evidence packager
func NewEvidencePackager() *EvidencePackager {
	return &EvidencePackager{}
}

// HypothesisEvidence represents the raw evidence supporting a hypothesis
type HypothesisEvidence struct {
	HypothesisID   string                 `json:"hypothesis_id"`
	EvidenceType   string                 `json:"evidence_type"` // "correlation", "breakpoint", "hysteresis"
	Fields         []FieldInfo            `json:"fields"`
	Relationships  []RelationshipInfo     `json:"relationships"`
	Breakpoints    []BreakpointInfo       `json:"breakpoints"`
	Hysteresis     []HysteresisInfo       `json:"hysteresis"`
	Coordinates    LandscapeCoordinates   `json:"coordinates"`
	Confidence     float64                `json:"confidence"`
	PValue         float64                `json:"p_value"`
	Description    string                 `json:"description"`
}

// FieldInfo describes a data field
type FieldInfo struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Range       string      `json:"range"`
	SampleCount int         `json:"sample_count"`
	Statistics  FieldStats  `json:"statistics"`
}

// FieldStats contains field statistics
type FieldStats struct {
	Mean   float64 `json:"mean,omitempty"`
	StdDev float64 `json:"std_dev,omitempty"`
	Min    float64 `json:"min,omitempty"`
	Max    float64 `json:"max,omitempty"`
}

// RelationshipInfo describes relationships between fields
type RelationshipInfo struct {
	Field1       string  `json:"field1"`
	Field2       string  `json:"field2"`
	Correlation  float64 `json:"correlation"`
	PValue       float64 `json:"p_value"`
	Method       string  `json:"method"`
	Strength     string  `json:"strength"` // "weak", "moderate", "strong"
}

// BreakpointInfo describes discontinuity points
type BreakpointInfo struct {
	Field         string  `json:"field"`
	Threshold     float64 `json:"threshold"`
	EffectBefore  float64 `json:"effect_before"`
	EffectAfter   float64 `json:"effect_after"`
	Confidence    float64 `json:"confidence"`
	PValue        float64 `json:"p_value"`
	Delta         float64 `json:"delta"`
}

// HysteresisInfo describes path-dependent effects
type HysteresisInfo struct {
	Field         string  `json:"field"`
	Strength      float64 `json:"strength"`
	RecoveryTime  string  `json:"recovery_time"`
	PathDependent bool    `json:"path_dependent"`
}

// LandscapeCoordinates for 3D positioning
type LandscapeCoordinates struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// PackageHypothesisEvidence extracts evidence for a specific hypothesis
func (ep *EvidencePackager) PackageHypothesisEvidence(
	hypothesis *models.HypothesisResult,
	associations []AssociationResult,
	breakpoints []BreakpointResult,
	hysteresis []HysteresisResult,
) *HypothesisEvidence {

	// Extract relevant fields mentioned in hypothesis
	fields := ep.extractRelevantFields(hypothesis, associations)

	// Get relationships relevant to this hypothesis
	relationships := ep.extractRelevantRelationships(hypothesis, associations)

	// Get breakpoints if applicable
	breakpointInfos := ep.extractRelevantBreakpoints(hypothesis, breakpoints)

	// Get hysteresis if applicable
	hysteresisInfos := ep.extractRelevantHysteresis(hypothesis, hysteresis)

	// Calculate 3D coordinates for visualization
	coordinates := ep.calculateLandscapeCoordinates(hypothesis, associations, breakpoints, hysteresis)

	// Get confidence metrics
	confidence := ep.calculateOverallConfidence(relationships, breakpointInfos, hysteresisInfos)

	// Create description
	description := ep.generateEvidenceDescription(hypothesis, relationships, breakpointInfos, hysteresisInfos)

	return &HypothesisEvidence{
		HypothesisID:  hypothesis.ID,
		EvidenceType:  ep.inferEvidenceType(hypothesis),
		Fields:        fields,
		Relationships: relationships,
		Breakpoints:   breakpointInfos,
		Hysteresis:    hysteresisInfos,
		Coordinates:   *coordinates,
		Confidence:    confidence,
		Description:   description,
	}
}

// inferEvidenceType determines what type of evidence supports this hypothesis
func (ep *EvidencePackager) inferEvidenceType(hypothesis *models.HypothesisResult) string {
	scienceHyp := hypothesis.ScienceHypothesis
	businessHyp := hypothesis.BusinessHypothesis

	// Check for breakpoint indicators
	if containsAny(scienceHyp, "threshold", "breakpoint", "critical", "tipping") ||
	   containsAny(businessHyp, "drops at", "changes at", "breaks at") {
		return "breakpoint"
	}

	// Check for hysteresis indicators
	if containsAny(scienceHyp, "hysteresis", "path-dependent", "memory", "recovery") ||
	   containsAny(businessHyp, "takes time", "recovery", "irreversible") {
		return "hysteresis"
	}

	// Check for correlation indicators
	if containsAny(scienceHyp, "correlation", "relationship", "association", "predicts") ||
	   containsAny(businessHyp, "linked to", "related to", "affects", "influences") {
		return "correlation"
	}

	return "correlation" // default
}

// extractRelevantFields finds fields mentioned in the hypothesis
func (ep *EvidencePackager) extractRelevantFields(
	hypothesis *models.HypothesisResult,
	associations []AssociationResult,
) []FieldInfo {

	var fields []FieldInfo
	fieldNames := ep.extractFieldNamesFromHypothesis(hypothesis)

	for _, fieldName := range fieldNames {
		// Find this field in the associations
		for _, assoc := range associations {
			if assoc.Feature == fieldName || assoc.Outcome == fieldName {
				fields = append(fields, FieldInfo{
					Name:        fieldName,
					Type:        "numeric", // Assume numeric for now
					SampleCount: assoc.NEffective,
					Statistics: FieldStats{
						Mean:   assoc.RawEffect, // Approximation
						StdDev: 0,
						Min:    0,
						Max:    0,
					},
				})
				break
			}
		}
	}

	return fields
}

// extractFieldNamesFromHypothesis parses field names from hypothesis text
func (ep *EvidencePackager) extractFieldNamesFromHypothesis(hypothesis *models.HypothesisResult) []string {
	var fieldNames []string

	// Look for quoted field names
	// This is a simple implementation - would need more sophisticated NLP
	// For now, return empty slice - would be populated by actual NLP

	return fieldNames
}

// extractRelevantRelationships gets correlations relevant to hypothesis
func (ep *EvidencePackager) extractRelevantRelationships(
	hypothesis *models.HypothesisResult,
	associations []AssociationResult,
) []RelationshipInfo {

	var relationships []RelationshipInfo

	for _, assoc := range associations {
		// Include all associations for now - could filter by confidence level
		strength := "moderate"
		absEffect := assoc.RawEffect
		if assoc.Direction < 0 {
			absEffect = -absEffect
		}
		if absEffect > 0.7 {
			strength = "strong"
		} else if absEffect < 0.3 {
			strength = "weak"
		}

		relationships = append(relationships, RelationshipInfo{
			Field1:      assoc.Feature,
			Field2:      assoc.Outcome,
			Correlation: assoc.RawEffect,
			PValue:      assoc.PValueAdj,
			Method:      assoc.Method,
			Strength:    strength,
		})
	}

	return relationships
}

// extractRelevantBreakpoints gets breakpoints relevant to hypothesis
func (ep *EvidencePackager) extractRelevantBreakpoints(
	hypothesis *models.HypothesisResult,
	breakpoints []BreakpointResult,
) []BreakpointInfo {

	var breakpointInfos []BreakpointInfo

	for _, bp := range breakpoints {
		// Include all breakpoints for now
		breakpointInfos = append(breakpointInfos, BreakpointInfo{
			Field:        bp.Feature,
			Threshold:    bp.Threshold,
			EffectBefore: bp.EffectBelow,
			EffectAfter:  bp.EffectAbove,
			Confidence:   bp.PValue,
			PValue:       bp.PValue,
			Delta:        bp.Delta,
		})
	}

	return breakpointInfos
}

// extractRelevantHysteresis gets hysteresis effects relevant to hypothesis
func (ep *EvidencePackager) extractRelevantHysteresis(
	hypothesis *models.HypothesisResult,
	hysteresis []HysteresisResult,
) []HysteresisInfo {

	var hysteresisInfos []HysteresisInfo

	for _, hyst := range hysteresis {
		// Include all hysteresis effects for now
		hysteresisInfos = append(hysteresisInfos, HysteresisInfo{
			Field:         hyst.Feature,
			Strength:      hyst.HysteresisStrength,
			PathDependent: true,
			RecoveryTime:  "unknown", // Would need to extract from actual analysis
		})
	}

	return hysteresisInfos
}

// calculateLandscapeCoordinates positions evidence in 3D space
func (ep *EvidencePackager) calculateLandscapeCoordinates(
	hypothesis *models.HypothesisResult,
	associations []AssociationResult,
	breakpoints []BreakpointResult,
	hysteresis []HysteresisResult,
) *LandscapeCoordinates {

	// Simple positioning based on evidence strength
	x, y, z := 0.0, 0.0, 0.0

	// Position based on primary relationship
	if len(associations) > 0 {
		assoc := associations[0]
		x = assoc.RawEffect * 50  // Correlation strength
		y = float64(assoc.PValueAdj * 100)  // Statistical significance
		z = 30.0  // Default importance
	}

	return &LandscapeCoordinates{X: x, Y: y, Z: z}
}

// calculateOverallConfidence combines confidence from different evidence types
func (ep *EvidencePackager) calculateOverallConfidence(
	relationships []RelationshipInfo,
	breakpoints []BreakpointInfo,
	hysteresis []HysteresisInfo,
) float64 {

	totalConfidence := 0.0
	count := 0

	// Weight different evidence types
	for _, rel := range relationships {
		if rel.PValue < 0.05 {
			totalConfidence += (1.0 - rel.PValue) * 0.4 // 40% weight for correlations
			count++
		}
	}

	for _, bp := range breakpoints {
		if bp.PValue < 0.05 {
			totalConfidence += (1.0 - bp.PValue) * 0.4 // 40% weight for breakpoints
			count++
		}
	}

	for _, hyst := range hysteresis {
		if hyst.Strength > 0.5 {
			totalConfidence += hyst.Strength * 0.2 // 20% weight for hysteresis
			count++
		}
	}

	if count == 0 {
		return 0.5 // Default moderate confidence
	}

	return totalConfidence / float64(count)
}

// generateEvidenceDescription creates human-readable evidence summary
func (ep *EvidencePackager) generateEvidenceDescription(
	hypothesis *models.HypothesisResult,
	relationships []RelationshipInfo,
	breakpoints []BreakpointInfo,
	hysteresis []HysteresisInfo,
) string {

	var description string

	if len(relationships) > 0 {
		rel := relationships[0]
		description = fmt.Sprintf("Based on %.2f correlation between %s and %s (p=%.3f)",
			rel.Correlation, rel.Field1, rel.Field2, rel.PValue)
	}

	if len(breakpoints) > 0 {
		bp := breakpoints[0]
		description += fmt.Sprintf(" with breakpoint at %s = %.2f", bp.Field, bp.Threshold)
	}

	if len(hysteresis) > 0 {
		hyst := hysteresis[0]
		description += fmt.Sprintf(" showing hysteresis effects (strength: %.2f)", hyst.Strength)
	}

	if description == "" {
		description = "Statistical evidence supporting this hypothesis"
	}

	return description
}

// containsAny checks if text contains any of the given substrings
func containsAny(text string, substrings ...string) bool {
	for _, substr := range substrings {
		if strings.Contains(strings.ToLower(text), substr) {
			return true
		}
	}
	return false
}
