package api

import (
	"fmt"
	"math"
	"time"

	"gohypo/domain/core"
)

// SchemaDriftDetector detects changes in API data schema and statistical properties
type SchemaDriftDetector struct {
	driftThresholds DriftThresholds
}

// DriftThresholds configures sensitivity for different types of drift
type DriftThresholds struct {
	EntropyChangeThreshold   float64 // Maximum allowed entropy change (0-1 scale)
	KurtosisChangeThreshold  float64 // Maximum allowed kurtosis change
	CardinalityChangePercent float64 // Maximum allowed cardinality change (%)
	TypeChangePenalty        float64 // Severity multiplier for type changes
	FieldRemovalPenalty      float64 // Severity multiplier for field removals
	FieldAdditionPenalty     float64 // Severity multiplier for field additions
}

// DefaultDriftThresholds returns sensible defaults for drift detection
func DefaultDriftThresholds() DriftThresholds {
	return DriftThresholds{
		EntropyChangeThreshold:   0.3,  // 30% entropy change
		KurtosisChangeThreshold:  2.0,  // 2.0 kurtosis change (significant fat-tail shift)
		CardinalityChangePercent: 50.0, // 50% cardinality change
		TypeChangePenalty:        3.0,  // High penalty for type changes
		FieldRemovalPenalty:      5.0,  // Very high penalty for field removal
		FieldAdditionPenalty:     0.5,  // Low penalty for field addition
	}
}

// NewSchemaDriftDetector creates a new drift detector
func NewSchemaDriftDetector(thresholds DriftThresholds) *SchemaDriftDetector {
	return &SchemaDriftDetector{
		driftThresholds: thresholds,
	}
}

// DetectDrift compares current data against the baseline fingerprint
func (sdd *SchemaDriftDetector) DetectDrift(
	dataSourceID core.ID,
	currentData []map[string]interface{},
	baselineFingerprint *SchemaFingerprint,
) (*SchemaDriftReport, error) {

	if baselineFingerprint == nil {
		return nil, fmt.Errorf("no baseline fingerprint available for comparison")
	}

	report := &SchemaDriftReport{
		DataSourceID:  dataSourceID,
		Severity:      DriftSeverityNone,
		Changes:       []FieldChange{},
		ImpactScore:   0.0,
		DetectionTime: time.Now(),
	}

	// Compute current fingerprint
	reader := &APIReader{} // We can create a minimal one for fingerprinting
	currentFingerprint, err := reader.ComputeSchemaFingerprint(currentData)
	if err != nil {
		return nil, fmt.Errorf("failed to compute current fingerprint: %w", err)
	}

	// Compare fingerprints field by field
	sdd.compareFields(baselineFingerprint, currentFingerprint, report)

	// Calculate overall severity and impact
	report.Severity = sdd.calculateOverallSeverity(report.Changes)
	report.ImpactScore = sdd.calculateImpactScore(report.Changes)

	// Generate recommendations
	report.Recommendations = sdd.generateRecommendations(report)

	return report, nil
}

// compareFields compares field fingerprints and detects changes
func (sdd *SchemaDriftDetector) compareFields(
	baseline, current *SchemaFingerprint,
	report *SchemaDriftReport,
) {

	// Check for removed fields
	for fieldName, baselineField := range baseline.Fields {
		if _, exists := current.Fields[fieldName]; !exists {
			change := FieldChange{
				FieldName: fieldName,
				ChangeType: ChangeTypeFieldRemoved,
				OldValue:  baselineField.DataType,
				Severity:  DriftSeverityHigh,
				Impact:    "Field removal may break dependent analyses and hypotheses",
			}
			report.Changes = append(report.Changes, change)
		}
	}

	// Check for added fields and modified existing fields
	for fieldName, currentField := range current.Fields {
		baselineField, existed := baseline.Fields[fieldName]

		if !existed {
			// New field added
			change := FieldChange{
				FieldName: fieldName,
				ChangeType: ChangeTypeFieldAdded,
				NewValue:  currentField.DataType,
				Severity:  DriftSeverityLow,
				Impact:    "New field available for analysis",
			}
			report.Changes = append(report.Changes, change)
			continue
		}

		// Compare existing fields
		sdd.compareFieldProperties(fieldName, baselineField, currentField, report)
	}
}

// compareFieldProperties detects changes in field statistical properties
func (sdd *SchemaDriftDetector) compareFieldProperties(
	fieldName string,
	baseline, current FieldFingerprint,
	report *SchemaDriftReport,
) {

	// Check for type changes
	if baseline.DataType != current.DataType {
		severity := sdd.assessTypeChangeSeverity(baseline.DataType, current.DataType)
		change := FieldChange{
			FieldName:  fieldName,
			ChangeType: ChangeTypeTypeChanged,
			OldValue:   baseline.DataType,
			NewValue:   current.DataType,
			Severity:   severity,
			Impact:     sdd.generateTypeChangeImpact(baseline.DataType, current.DataType),
		}
		report.Changes = append(report.Changes, change)
		return // Type change is the most significant, skip other checks
	}

	// Check for statistical property changes
	if sdd.hasSignificantEntropyChange(baseline.Entropy, current.Entropy) {
		change := FieldChange{
			FieldName:  fieldName,
			ChangeType: ChangeTypeEntropySpike,
			OldValue:   baseline.Entropy,
			NewValue:   current.Entropy,
			Severity:   DriftSeverityMedium,
			Impact:     "Data distribution has changed significantly, may indicate data quality issues or source changes",
		}
		report.Changes = append(report.Changes, change)
	}

	if sdd.hasSignificantKurtosisChange(baseline.Kurtosis, current.Kurtosis) {
		change := FieldChange{
			FieldName:  fieldName,
			ChangeType: ChangeTypeKurtosisShift,
			OldValue:   baseline.Kurtosis,
			NewValue:   current.Kurtosis,
			Severity:   DriftSeverityMedium,
			Impact:     "Distribution shape has changed (fat-tail shift), may affect statistical tests and risk assessments",
		}
		report.Changes = append(report.Changes, change)
	}

	if sdd.hasSignificantCardinalityChange(baseline.Cardinality, current.Cardinality) {
		change := FieldChange{
			FieldName:  fieldName,
			ChangeType: ChangeTypeCardinalityChange,
			OldValue:   baseline.Cardinality,
			NewValue:   current.Cardinality,
			Severity:   DriftSeverityLow,
			Impact:     "Number of unique values has changed significantly",
		}
		report.Changes = append(report.Changes, change)
	}
}

// assessTypeChangeSeverity determines the severity of a type change
func (sdd *SchemaDriftDetector) assessTypeChangeSeverity(oldType, newType string) DriftSeverity {
	// Safe conversions
	if (oldType == "int" && newType == "float") ||
	   (oldType == "float" && newType == "int") {
		return DriftSeverityLow
	}

	// Potentially safe conversions
	if (oldType == "string" && newType == "numeric") ||
	   (oldType == "numeric" && newType == "string") {
		return DriftSeverityMedium
	}

	// Breaking changes
	return DriftSeverityHigh
}

// generateTypeChangeImpact provides context about type change implications
func (sdd *SchemaDriftDetector) generateTypeChangeImpact(oldType, newType string) string {
	switch {
	case oldType == "int" && newType == "float":
		return "Safe conversion: integer precision preserved, decimal values now possible"
	case oldType == "float" && newType == "int":
		return "Potentially lossy: decimal precision will be truncated"
	case oldType == "string" && newType == "numeric":
		return "May fail for non-numeric strings, enables mathematical operations"
	case oldType == "numeric" && newType == "string":
		return "Disables mathematical operations, preserves all values as strings"
	default:
		return "Type conversion may require analysis pipeline adjustments"
	}
}

// Statistical change detection methods
func (sdd *SchemaDriftDetector) hasSignificantEntropyChange(oldEntropy, newEntropy float64) bool {
	if oldEntropy == 0 {
		return newEntropy > sdd.driftThresholds.EntropyChangeThreshold
	}

	changePercent := math.Abs(newEntropy-oldEntropy) / oldEntropy
	return changePercent > sdd.driftThresholds.EntropyChangeThreshold
}

func (sdd *SchemaDriftDetector) hasSignificantKurtosisChange(oldKurtosis, newKurtosis float64) bool {
	change := math.Abs(newKurtosis - oldKurtosis)
	return change > sdd.driftThresholds.KurtosisChangeThreshold
}

func (sdd *SchemaDriftDetector) hasSignificantCardinalityChange(oldCard, newCard int) bool {
	if oldCard == 0 {
		return newCard > 10 // Arbitrary threshold for new fields
	}

	changePercent := math.Abs(float64(newCard-oldCard)) / float64(oldCard) * 100
	return changePercent > sdd.driftThresholds.CardinalityChangePercent
}

// calculateOverallSeverity determines the highest severity among all changes
func (sdd *SchemaDriftDetector) calculateOverallSeverity(changes []FieldChange) DriftSeverity {
	maxSeverity := DriftSeverityNone

	for _, change := range changes {
		if change.Severity > maxSeverity {
			maxSeverity = change.Severity
		}
	}

	return maxSeverity
}

// calculateImpactScore computes a weighted impact score from 0-1
func (sdd *SchemaDriftDetector) calculateImpactScore(changes []FieldChange) float64 {
	if len(changes) == 0 {
		return 0.0
	}

	totalImpact := 0.0
	for _, change := range changes {
		severityWeight := sdd.getSeverityWeight(change.Severity)
		changeMultiplier := sdd.getChangeTypeMultiplier(change.ChangeType)
		totalImpact += severityWeight * changeMultiplier
	}

	// Normalize to 0-1 scale
	maxPossibleImpact := float64(len(changes)) * sdd.getSeverityWeight(DriftSeverityCritical) * sdd.driftThresholds.FieldRemovalPenalty
	if maxPossibleImpact == 0 {
		return 0.0
	}

	score := totalImpact / maxPossibleImpact
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// getSeverityWeight converts severity to numeric weight
func (sdd *SchemaDriftDetector) getSeverityWeight(severity DriftSeverity) float64 {
	switch severity {
	case DriftSeverityLow:
		return 1.0
	case DriftSeverityMedium:
		return 2.0
	case DriftSeverityHigh:
		return 3.0
	case DriftSeverityCritical:
		return 4.0
	default:
		return 0.0
	}
}

// getChangeTypeMultiplier applies type-specific multipliers
func (sdd *SchemaDriftDetector) getChangeTypeMultiplier(changeType ChangeType) float64 {
	switch changeType {
	case ChangeTypeFieldRemoved:
		return sdd.driftThresholds.FieldRemovalPenalty
	case ChangeTypeTypeChanged:
		return sdd.driftThresholds.TypeChangePenalty
	case ChangeTypeFieldAdded:
		return sdd.driftThresholds.FieldAdditionPenalty
	case ChangeTypeEntropySpike, ChangeTypeKurtosisShift:
		return 2.0 // Statistical changes are moderately important
	case ChangeTypeCardinalityChange:
		return 1.5
	default:
		return 1.0
	}
}

// generateRecommendations provides actionable advice based on detected changes
func (sdd *SchemaDriftDetector) generateRecommendations(report *SchemaDriftReport) []string {
	recommendations := []string{}

	if report.Severity >= DriftSeverityHigh {
		recommendations = append(recommendations,
			"High-severity drift detected: Manual review required before accepting changes")
	}

	hasTypeChanges := false
	hasFieldRemovals := false
	hasStatisticalChanges := false

	for _, change := range report.Changes {
		switch change.ChangeType {
		case ChangeTypeTypeChanged:
			hasTypeChanges = true
		case ChangeTypeFieldRemoved:
			hasFieldRemovals = true
		case ChangeTypeEntropySpike, ChangeTypeKurtosisShift:
			hasStatisticalChanges = true
		}
	}

	if hasTypeChanges {
		recommendations = append(recommendations,
			"Type changes detected: Review data transformation logic in analysis pipelines")
	}

	if hasFieldRemovals {
		recommendations = append(recommendations,
			"Field removals detected: Update any hardcoded field references in analyses")
	}

	if hasStatisticalChanges {
		recommendations = append(recommendations,
			"Statistical properties changed: Re-validate hypothesis tests and statistical assumptions")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations,
			"Low-impact changes detected: Safe to auto-accept if automated testing passes")
	}

	return recommendations
}

// ValidateDataQuality performs additional quality checks beyond schema comparison
func (sdd *SchemaDriftDetector) ValidateDataQuality(data []map[string]interface{}) []string {
	issues := []string{}

	if len(data) == 0 {
		issues = append(issues, "No data records received")
		return issues
	}

	// Check for completely empty records
	emptyRecords := 0
	for _, record := range data {
		if len(record) == 0 {
			emptyRecords++
		}
	}

	if emptyRecords > 0 {
		emptyPercent := float64(emptyRecords) / float64(len(data)) * 100
		if emptyPercent > 50 {
			issues = append(issues, fmt.Sprintf("%.1f%% of records are completely empty", emptyPercent))
		}
	}

	// Check for potential data corruption (all values identical)
	for fieldName := range data[0] {
		allIdentical := true
		firstValue := ""
		for _, record := range data {
			if val, exists := record[fieldName]; exists {
				valStr := fmt.Sprintf("%v", val)
				if firstValue == "" {
					firstValue = valStr
				} else if valStr != firstValue {
					allIdentical = false
					break
				}
			}
		}

		if allIdentical && len(data) > 1 {
			issues = append(issues, fmt.Sprintf("Field '%s' has identical values across all records - possible data corruption", fieldName))
		}
	}

	return issues
}
