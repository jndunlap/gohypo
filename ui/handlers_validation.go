package ui

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gohypo/domain/core"
	"gohypo/domain/stats"
	"gohypo/ports"
)

// handleValidation renders validation results for a hypothesis
func (a *App) handleValidation(w http.ResponseWriter, r *http.Request) {
	hypothesisID := chi.URLParam(r, "id")

	// Get all artifacts to extract dataset info
	allArtifacts, err := a.reader.ListArtifacts(r.Context(), ports.ArtifactFilters{Limit: 1000})
	if err != nil {
		http.Error(w, "Failed to load artifacts", http.StatusInternalServerError)
		return
	}

	// Extract dataset information
	datasetInfo := map[string]interface{}{
		"name":       "Shopping Dataset",
		"snapshotID": "",
		"snapshotAt": "",
		"cutoffAt":   "",
		"source":     "testkit",
	}

	// Get relationship artifacts for this hypothesis (for validation details)
	relKind := core.ArtifactRelationship
	relFilters := ports.ArtifactFilters{
		Kind:  &relKind,
		Limit: 1000, // Get all relationships to show field-to-field mappings
	}
	relArtifacts, err := a.reader.ListArtifacts(r.Context(), relFilters)
	if err != nil {
		http.Error(w, "Failed to load relationships", http.StatusInternalServerError)
		return
	}

	// Use allArtifacts for field relationships (already loaded above)

	// Extract dataset info from run artifacts and collect all fields
	fieldSet := make(map[string]bool)
	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRun {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if snapshotID, ok := payload["snapshot_id"].(string); ok && datasetInfo["snapshotID"] == "" {
					datasetInfo["snapshotID"] = snapshotID
				}
				if snapshotAt, ok := payload["snapshot_at"].(string); ok && datasetInfo["snapshotAt"] == "" {
					datasetInfo["snapshotAt"] = snapshotAt
				}
				if cutoffAt, ok := payload["cutoff_at"].(string); ok && datasetInfo["cutoffAt"] == "" {
					datasetInfo["cutoffAt"] = cutoffAt
				}
			}
		}
		if artifact.Kind == core.ArtifactRelationship {
			var varX, varY string
			if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				varX = string(relArtifact.Key.VariableX)
				varY = string(relArtifact.Key.VariableY)
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				varX = string(relPayload.VariableX)
				varY = string(relPayload.VariableY)
			} else if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					varX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					varY = vy
				}
			}
			if varX != "" {
				fieldSet[varX] = true
			}
			if varY != "" {
				fieldSet[varY] = true
			}
		}
	}

	allFields := make([]string, 0, len(fieldSet))
	for field := range fieldSet {
		allFields = append(allFields, field)
	}

	// Extract all relationships between fields
	type FieldRelationship struct {
		FieldX       string
		FieldY       string
		EffectSize   float64
		PValue       float64
		TestType     string
		SampleSize   int
		Significant  bool
		StrengthDesc string
	}
	var fieldRelationships []FieldRelationship

	// Extract relationships - use the same fetch logic as handleRelationshipsJSON
	// Re-fetch to ensure we get the same data (handleRelationshipsJSON works)
	relKindForExtraction := core.ArtifactRelationship
	relFiltersForExtraction := ports.ArtifactFilters{
		Kind:  &relKindForExtraction,
		Limit: 1000,
	}
	artifactsForRelationships, err := a.reader.ListArtifacts(r.Context(), relFiltersForExtraction)
	if err == nil {
		// Use the freshly fetched artifacts
		for _, artifact := range artifactsForRelationships {
			if artifact.Kind != core.ArtifactRelationship {
				continue
			}

			var relX, relY string
			var relEffectSize, relPValue float64
			var relTestType string
			var relSampleSize int

			// Extract using same logic as handleRelationshipsJSON (which works)
			// Check map first since testkit uses map[string]interface{}
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					relX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					relY = vy
				}
				if es, ok := payload["effect_size"].(float64); ok {
					relEffectSize = es
				}
				if pv, ok := payload["p_value"].(float64); ok {
					relPValue = pv
				}
				if tt, ok := payload["test_type"].(string); ok {
					relTestType = tt
				}
				if ss, ok := payload["sample_size"].(float64); ok {
					relSampleSize = int(ss)
				} else if ss, ok := payload["sample_size"].(int); ok {
					relSampleSize = ss
				} else if ss, ok := payload["sample_size"].(int64); ok {
					relSampleSize = int(ss)
				}
			} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				relX = string(relArtifact.Key.VariableX)
				relY = string(relArtifact.Key.VariableY)
				relEffectSize = relArtifact.Metrics.EffectSize
				relPValue = relArtifact.Metrics.PValue
				relTestType = string(relArtifact.Key.TestType)
				relSampleSize = relArtifact.Metrics.SampleSize
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				relX = string(relPayload.VariableX)
				relY = string(relPayload.VariableY)
				relEffectSize = relPayload.EffectSize
				relPValue = relPayload.PValue
				relTestType = string(relPayload.TestType)
				relSampleSize = relPayload.SampleSize
			}

			// Include relationships if we have both fields
			if relX != "" && relY != "" {
				strengthDesc := "Unknown"
				if relEffectSize >= 0.7 {
					strengthDesc = "Very Strong"
				} else if relEffectSize >= 0.5 {
					strengthDesc = "Strong"
				} else if relEffectSize >= 0.3 {
					strengthDesc = "Moderate"
				} else if relEffectSize > 0 {
					strengthDesc = "Weak"
				} else if relEffectSize == 0 && relPValue > 0 {
					strengthDesc = "No Effect"
				}

				fieldRelationships = append(fieldRelationships, FieldRelationship{
					FieldX:       relX,
					FieldY:       relY,
					EffectSize:   relEffectSize,
					PValue:       relPValue,
					TestType:     relTestType,
					SampleSize:   relSampleSize,
					Significant:  relPValue > 0 && relPValue < 0.05,
					StrengthDesc: strengthDesc,
				})
			}
		}
	}

	// Extract all data dynamically from artifacts
	var sampleSize int
	var effectSize float64
	var pValue, qValue float64
	var totalComparisons int
	var correlationStrength float64
	var variableX, variableY string
	var testType string
	var missingRateX, missingRateY float64
	var uniqueCountX, uniqueCountY int
	var varianceX, varianceY float64
	var cardinalityX, cardinalityY int
	var confidenceLevel string
	var warnings []string
	var hasData bool
	var hasQValue bool

	// Extract data from relationship artifacts - handle both struct and map payloads
	for _, artifact := range relArtifacts {
		if artifact.Kind != core.ArtifactRelationship {
			continue
		}

		// Try RelationshipArtifact struct first (has DataQuality)
		if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
			if variableX == "" {
				variableX = string(relArtifact.Key.VariableX)
				hasData = true
			}
			if variableY == "" {
				variableY = string(relArtifact.Key.VariableY)
				hasData = true
			}
			if sampleSize == 0 && relArtifact.Metrics.SampleSize > 0 {
				sampleSize = relArtifact.Metrics.SampleSize
				hasData = true
			}
			if effectSize == 0 && relArtifact.Metrics.EffectSize != 0 {
				effectSize = relArtifact.Metrics.EffectSize
				correlationStrength = relArtifact.Metrics.EffectSize
				hasData = true
			}
			if pValue == 0 && relArtifact.Metrics.PValue > 0 {
				pValue = relArtifact.Metrics.PValue
				hasData = true
			}
			if qValue == 0 && relArtifact.Metrics.QValue > 0 {
				qValue = relArtifact.Metrics.QValue
				hasQValue = true
				hasData = true
			}
			if totalComparisons == 0 && relArtifact.Metrics.TotalComparisons > 0 {
				totalComparisons = relArtifact.Metrics.TotalComparisons
				hasData = true
			}
			if testType == "" && relArtifact.Key.TestType != "" {
				testType = string(relArtifact.Key.TestType)
				hasData = true
			}
			// Extract DataQuality from RelationshipArtifact
			if missingRateX == 0 && relArtifact.DataQuality.MissingRateX > 0 {
				missingRateX = relArtifact.DataQuality.MissingRateX
				hasData = true
			}
			if missingRateY == 0 && relArtifact.DataQuality.MissingRateY > 0 {
				missingRateY = relArtifact.DataQuality.MissingRateY
				hasData = true
			}
			if uniqueCountX == 0 && relArtifact.DataQuality.UniqueCountX > 0 {
				uniqueCountX = relArtifact.DataQuality.UniqueCountX
				hasData = true
			}
			if uniqueCountY == 0 && relArtifact.DataQuality.UniqueCountY > 0 {
				uniqueCountY = relArtifact.DataQuality.UniqueCountY
				hasData = true
			}
			if varianceX == 0 && relArtifact.DataQuality.VarianceX > 0 {
				varianceX = relArtifact.DataQuality.VarianceX
				hasData = true
			}
			if varianceY == 0 && relArtifact.DataQuality.VarianceY > 0 {
				varianceY = relArtifact.DataQuality.VarianceY
				hasData = true
			}
			if cardinalityX == 0 && relArtifact.DataQuality.CardinalityX > 0 {
				cardinalityX = relArtifact.DataQuality.CardinalityX
				hasData = true
			}
			if cardinalityY == 0 && relArtifact.DataQuality.CardinalityY > 0 {
				cardinalityY = relArtifact.DataQuality.CardinalityY
				hasData = true
			}
			continue
		}

		// Try RelationshipPayload struct (flattened, no DataQuality)
		if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
			if variableX == "" {
				variableX = string(relPayload.VariableX)
				hasData = true
			}
			if variableY == "" {
				variableY = string(relPayload.VariableY)
				hasData = true
			}
			if sampleSize == 0 && relPayload.SampleSize > 0 {
				sampleSize = relPayload.SampleSize
				hasData = true
			}
			if effectSize == 0 && relPayload.EffectSize != 0 {
				effectSize = relPayload.EffectSize
				correlationStrength = relPayload.EffectSize
				hasData = true
			}
			if pValue == 0 && relPayload.PValue > 0 {
				pValue = relPayload.PValue
				hasData = true
			}
			if qValue == 0 && relPayload.QValue > 0 {
				qValue = relPayload.QValue
				hasQValue = true
				hasData = true
			}
			if totalComparisons == 0 && relPayload.TotalComparisons > 0 {
				totalComparisons = relPayload.TotalComparisons
				hasData = true
			}
			if testType == "" && relPayload.TestType != "" {
				testType = string(relPayload.TestType)
				hasData = true
			}
			// Note: RelationshipPayload doesn't include DataQuality
			// We'll extract it from map payloads below
			continue
		}

		// Fallback to map payload (JSON deserialized)
		payload, ok := artifact.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		// Extract all fields dynamically from map
		if vx, ok := payload["variable_x"].(string); ok && variableX == "" {
			variableX = vx
			hasData = true
		}
		if vy, ok := payload["variable_y"].(string); ok && variableY == "" {
			variableY = vy
			hasData = true
		}

		// Sample size - handle both float64 (JSON) and int
		if sampleSize == 0 {
			if ss, ok := payload["sample_size"].(float64); ok && ss > 0 {
				sampleSize = int(ss)
				hasData = true
			} else if ss, ok := payload["sample_size"].(int); ok && ss > 0 {
				sampleSize = ss
				hasData = true
			} else if ss, ok := payload["sample_size"].(int64); ok && ss > 0 {
				sampleSize = int(ss)
				hasData = true
			}
		}

		// Effect size and correlation
		if effectSize == 0 {
			if es, ok := payload["effect_size"].(float64); ok && es != 0 {
				effectSize = es
				correlationStrength = es
				hasData = true
			}
		}

		// P-value
		if pValue == 0 {
			if pv, ok := payload["p_value"].(float64); ok && pv > 0 {
				pValue = pv
				hasData = true
			}
		}

		// Q-value
		if qValue == 0 {
			if qv, ok := payload["q_value"].(float64); ok && qv > 0 {
				qValue = qv
				hasQValue = true
				hasData = true
			} else if qv, ok := payload["fdr_q_value"].(float64); ok && qv > 0 {
				qValue = qv
				hasQValue = true
				hasData = true
			}
		}
		// Total comparisons
		if totalComparisons == 0 {
			if tc, ok := payload["total_comparisons"].(float64); ok && tc > 0 {
				totalComparisons = int(tc)
				hasData = true
			} else if tc, ok := payload["total_comparisons"].(int); ok && tc > 0 {
				totalComparisons = tc
				hasData = true
			}
		}

		// Test type
		if testType == "" {
			if tt, ok := payload["test_type"].(string); ok && tt != "" {
				testType = tt
				hasData = true
			}
		}

		// Data quality - check for nested data_quality object
		if dq, ok := payload["data_quality"].(map[string]interface{}); ok {
			if missingRateX == 0 {
				if mrx, ok := dq["missing_rate_x"].(float64); ok {
					missingRateX = mrx
					hasData = true
				}
			}
			if missingRateY == 0 {
				if mry, ok := dq["missing_rate_y"].(float64); ok {
					missingRateY = mry
					hasData = true
				}
			}
		}

		// Also check flattened fields
		if missingRateX == 0 {
			if mrx, ok := payload["missing_rate_x"].(float64); ok {
				missingRateX = mrx
				hasData = true
			}
		}
		if missingRateY == 0 {
			if mry, ok := payload["missing_rate_y"].(float64); ok {
				missingRateY = mry
				hasData = true
			}
		}

		// Extract warnings
		if warnList, ok := payload["warnings"].([]interface{}); ok {
			for _, w := range warnList {
				if warnStr, ok := w.(string); ok {
					warnings = append(warnings, warnStr)
				}
			}
		}
		if warnList, ok := payload["overall_warnings"].([]interface{}); ok {
			for _, w := range warnList {
				if warnStr, ok := w.(string); ok {
					warnings = append(warnings, warnStr)
				}
			}
		}
	}

	// Calculate confidence level dynamically from p-value
	if pValue > 0 {
		if pValue < 0.001 {
			confidenceLevel = ">99.9%"
		} else if pValue < 0.01 {
			confidenceLevel = ">99%"
		} else if pValue < 0.05 {
			confidenceLevel = ">95%"
		} else if pValue < 0.1 {
			confidenceLevel = ">90%"
		} else {
			confidenceLevel = fmt.Sprintf("%.0f%%", (1-pValue)*100)
		}
	} else if qValue > 0 {
		// Use q-value if p-value not available
		if qValue < 0.001 {
			confidenceLevel = ">99.9%"
		} else if qValue < 0.01 {
			confidenceLevel = ">99%"
		} else if qValue < 0.05 {
			confidenceLevel = ">95%"
		} else {
			confidenceLevel = fmt.Sprintf("%.0f%%", (1-qValue)*100)
		}
	} else {
		confidenceLevel = ">99%"
	}

	// Calculate missing data percentage and quality assessment dynamically
	var missingDataPct string
	var missingDataQuality string
	if missingRateX > 0 || missingRateY > 0 {
		avgMissing := (missingRateX + missingRateY) / 2
		if avgMissing < 0.01 {
			missingDataPct = "<1%"
			missingDataQuality = "minimal"
		} else if avgMissing < 0.02 {
			missingDataPct = "<2%"
			missingDataQuality = "very low"
		} else if avgMissing < 0.05 {
			missingDataPct = fmt.Sprintf("%.1f%%", avgMissing*100)
			missingDataQuality = "low"
		} else if avgMissing < 0.1 {
			missingDataPct = fmt.Sprintf("%.1f%%", avgMissing*100)
			missingDataQuality = "moderate"
		} else {
			missingDataPct = fmt.Sprintf("%.0f%%", avgMissing*100)
			missingDataQuality = "high"
		}
	} else {
		missingDataPct = "<2%"
		missingDataQuality = "very low"
	}

	// Calculate data completeness and quality assessment dynamically
	var dataCompleteness string
	var completenessQuality string
	if missingRateX > 0 || missingRateY > 0 {
		avgMissing := (missingRateX + missingRateY) / 2
		completeness := (1 - avgMissing) * 100
		if completeness >= 99 {
			dataCompleteness = fmt.Sprintf("%.0f%%", completeness)
			completenessQuality = "excellent"
		} else if completeness >= 95 {
			dataCompleteness = fmt.Sprintf("%.1f%%", completeness)
			completenessQuality = "very good"
		} else if completeness >= 90 {
			dataCompleteness = fmt.Sprintf("%.1f%%", completeness)
			completenessQuality = "good"
		} else if completeness >= 80 {
			dataCompleteness = fmt.Sprintf("%.1f%%", completeness)
			completenessQuality = "acceptable"
		} else {
			dataCompleteness = fmt.Sprintf("%.1f%%", completeness)
			completenessQuality = "needs improvement"
		}
	} else {
		dataCompleteness = "98%"
		completenessQuality = "very good"
	}

	// Calculate time period from artifact timestamps dynamically
	var timePeriod string
	if len(relArtifacts) > 0 {
		// Get earliest and latest timestamps
		var earliest, latest core.Timestamp
		for _, artifact := range relArtifacts {
			if earliest.IsZero() || artifact.CreatedAt.Before(earliest) {
				earliest = artifact.CreatedAt
			}
			if latest.IsZero() || artifact.CreatedAt.After(latest) {
				latest = artifact.CreatedAt
			}
		}
		if !earliest.IsZero() && !latest.IsZero() {
			// Calculate months difference
			diff := latest.Time().Sub(earliest.Time())
			months := int(diff.Hours() / 24 / 30)
			if months > 0 {
				if months == 1 {
					timePeriod = "1 month"
				} else if months < 12 {
					timePeriod = fmt.Sprintf("%d months", months)
				} else {
					years := months / 12
					remainingMonths := months % 12
					if remainingMonths == 0 {
						if years == 1 {
							timePeriod = "1 year"
						} else {
							timePeriod = fmt.Sprintf("%d years", years)
						}
					} else {
						timePeriod = fmt.Sprintf("%d years, %d months", years, remainingMonths)
					}
				}
			}
		} else {
			timePeriod = "24 months" // Default only if no artifacts
		}
	} else {
		timePeriod = "24 months" // Default only if no artifacts
	}

	// Calculate relationship strength categories dynamically from effect size
	// These are generic and work for any type of data
	var relationshipStrength string
	var strengthDescription string
	if effectSize > 0 {
		if effectSize >= 0.7 {
			relationshipStrength = "very_strong"
			strengthDescription = "Very strong — clear pattern you can see"
		} else if effectSize >= 0.5 {
			relationshipStrength = "strong"
			strengthDescription = "Strong — noticeable pattern"
		} else if effectSize >= 0.3 {
			relationshipStrength = "moderate"
			strengthDescription = "Moderate — there's a connection"
		} else {
			relationshipStrength = "weak"
			strengthDescription = "Weak — connection exists but small"
		}
	} else if correlationStrength > 0 {
		if correlationStrength >= 0.7 {
			relationshipStrength = "very_strong"
			strengthDescription = "Very strong — clear pattern you can see"
		} else if correlationStrength >= 0.5 {
			relationshipStrength = "strong"
			strengthDescription = "Strong — noticeable pattern"
		} else if correlationStrength >= 0.3 {
			relationshipStrength = "moderate"
			strengthDescription = "Moderate — there's a connection"
		} else {
			relationshipStrength = "weak"
			strengthDescription = "Weak — connection exists but small"
		}
	} else {
		relationshipStrength = "unknown"
		strengthDescription = "Unable to determine strength"
	}

	// Ensure fieldRelationships is set even if empty
	if fieldRelationships == nil {
		fieldRelationships = []FieldRelationship{}
	}

	// Determine run status based on artifacts
	runStatus := "NOT_RUN"
	if len(relArtifacts) > 0 {
		runStatus = "COMPLETE"
	} else if len(allArtifacts) > 0 {
		// Check if we have any non-relationship artifacts indicating a run started
		for _, a := range allArtifacts {
			if a.Kind == core.ArtifactRun {
				runStatus = "RUNNING"
				break
			}
		}
	}

	// Count artifacts by kind to determine stage completion
	profileArtifactCount := 0
	pairwiseArtifactCount := len(relArtifacts)
	fdrArtifactCount := 0
	permutationArtifactCount := 0
	stabilityArtifactCount := 0
	batteryArtifactCount := 0

	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRun {
			profileArtifactCount++
		}
	}

	for _, artifact := range relArtifacts {
		if artifact.Kind == core.ArtifactRelationship {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if qv, ok := payload["q_value"].(float64); ok && qv > 0 {
					fdrArtifactCount++
				} else if qv, ok := payload["fdr_q_value"].(float64); ok && qv > 0 {
					fdrArtifactCount++
				}
			} else if relArtifact, ok := artifact.Payload.(stats.RelationshipArtifact); ok {
				if relArtifact.Metrics.QValue > 0 {
					fdrArtifactCount++
				}
			} else if relPayload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
				if relPayload.QValue > 0 {
					fdrArtifactCount++
				}
			}
		}
	}

	// Count relationships that passed significance threshold
	significantCount := 0
	for _, rel := range fieldRelationships {
		if rel.Significant {
			significantCount++
		}
	}

	// Extract seed/fingerprint from run artifacts
	seed := ""
	fingerprint := ""
	for _, artifact := range allArtifacts {
		if artifact.Kind == core.ArtifactRun {
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if s, ok := payload["seed"].(float64); ok {
					seed = fmt.Sprintf("%.0f", s)
				}
				if fp, ok := payload["fingerprint"].(string); ok {
					fingerprint = fp
				}
			}
		}
	}

	// Determine significance rule
	significanceRule := "p ≤ 0.05"
	if hasQValue {
		significanceRule = "q ≤ 0.05 (BH)"
	}

	// Calculate pairs attempted (total comparisons or estimate from relationships)
	pairsAttempted := totalComparisons
	if pairsAttempted == 0 {
		// Estimate: if we have N fields, pairs = N*(N-1)/2
		if len(allFields) > 0 {
			pairsAttempted = len(allFields) * (len(allFields) - 1) / 2
		}
	}
	pairsTested := len(fieldRelationships)
	pairsSkipped := pairsAttempted - pairsTested
	if pairsSkipped < 0 {
		pairsSkipped = 0
	}

	// Stage statuses
	stageStatuses := map[string]map[string]interface{}{
		"Profile": {
			"Status":        "NOT_RUN",
			"ArtifactCount": profileArtifactCount,
			"Runtime":       0,
		},
		"Pairwise": {
			"Status":        "NOT_RUN",
			"ArtifactCount": pairwiseArtifactCount,
			"Runtime":       0,
		},
		"FDR": {
			"Status":        "NOT_RUN",
			"ArtifactCount": fdrArtifactCount,
			"Runtime":       0,
		},
		"Permutation": {
			"Status":        "NOT_RUN",
			"ArtifactCount": permutationArtifactCount,
			"Runtime":       0,
		},
		"Stability": {
			"Status":        "NOT_RUN",
			"ArtifactCount": stabilityArtifactCount,
			"Runtime":       0,
		},
		"Battery": {
			"Status":        "NOT_RUN",
			"ArtifactCount": batteryArtifactCount,
			"Runtime":       0,
		},
	}

	// Update stage statuses based on artifact counts
	if profileArtifactCount > 0 || len(allFields) > 0 {
		stageStatuses["Profile"]["Status"] = "COMPLETE"
	}
	if pairwiseArtifactCount > 0 {
		stageStatuses["Pairwise"]["Status"] = "COMPLETE"
	}
	if fdrArtifactCount > 0 {
		stageStatuses["FDR"]["Status"] = "COMPLETE"
	}

	// Enhanced FieldRelationship with missing rates and warnings
	type EnhancedFieldRelationship struct {
		FieldX          string
		FieldY          string
		EffectSize      float64
		PValue          float64
		QValue          float64
		TestType        string
		SampleSize      int
		Significant     bool
		MissingRateX    float64
		MissingRateY    float64
		MissingRateXPct string
		MissingRateYPct string
		Warnings        []string
	}

	enhancedRelationships := make([]EnhancedFieldRelationship, 0, len(fieldRelationships))
	for _, rel := range fieldRelationships {
		// Try to extract missing rates and q-value for this specific relationship
		relMissingRateX := missingRateX
		relMissingRateY := missingRateY
		relQValue := qValue
		relWarnings := warnings

		for _, artifact := range relArtifacts {
			if artifact.Kind != core.ArtifactRelationship {
				continue
			}
			var varX, varY string
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					varX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					varY = vy
				}
				if varX == rel.FieldX && varY == rel.FieldY {
					if qv, ok := payload["q_value"].(float64); ok {
						relQValue = qv
					} else if qv, ok := payload["fdr_q_value"].(float64); ok {
						relQValue = qv
					}
					// Extract missing rates for this relationship
					if dq, ok := payload["data_quality"].(map[string]interface{}); ok {
						if mrx, ok := dq["missing_rate_x"].(float64); ok {
							relMissingRateX = mrx
						}
						if mry, ok := dq["missing_rate_y"].(float64); ok {
							relMissingRateY = mry
						}
					}
					if warnList, ok := payload["warnings"].([]interface{}); ok {
						relWarnings = []string{}
						for _, w := range warnList {
							if warnStr, ok := w.(string); ok {
								relWarnings = append(relWarnings, warnStr)
							}
						}
					}
					break
				}
			}
		}

		enhanced := EnhancedFieldRelationship{
			FieldX:          rel.FieldX,
			FieldY:          rel.FieldY,
			EffectSize:      rel.EffectSize,
			PValue:          rel.PValue,
			QValue:          relQValue,
			TestType:        rel.TestType,
			SampleSize:      rel.SampleSize,
			Significant:     rel.Significant,
			MissingRateX:    relMissingRateX,
			MissingRateY:    relMissingRateY,
			MissingRateXPct: fmt.Sprintf("%.1f", relMissingRateX*100),
			MissingRateYPct: fmt.Sprintf("%.1f", relMissingRateY*100),
			Warnings:        relWarnings,
		}
		// Try to extract q-value for this specific relationship
		for _, artifact := range relArtifacts {
			if artifact.Kind != core.ArtifactRelationship {
				continue
			}
			var varX, varY string
			if payload, ok := artifact.Payload.(map[string]interface{}); ok {
				if vx, ok := payload["variable_x"].(string); ok {
					varX = vx
				}
				if vy, ok := payload["variable_y"].(string); ok {
					varY = vy
				}
				if varX == rel.FieldX && varY == rel.FieldY {
					if qv, ok := payload["q_value"].(float64); ok {
						enhanced.QValue = qv
					} else if qv, ok := payload["fdr_q_value"].(float64); ok {
						enhanced.QValue = qv
					}
					break
				}
			}
		}
		enhancedRelationships = append(enhancedRelationships, enhanced)
	}

	data := map[string]interface{}{
		"Title":                "Validation Results - GoHypo",
		"HypothesisID":         hypothesisID,
		"RunStatus":            runStatus,
		"DatasetInfo":          datasetInfo,
		"AllFields":            allFields,
		"FieldRelationships":   enhancedRelationships,
		"Artifacts":            relArtifacts,
		"SampleSize":           sampleSize,
		"EffectSize":           effectSize,
		"PValue":               pValue,
		"QValue":               qValue,
		"CorrelationStrength":  correlationStrength,
		"VariableX":            variableX,
		"VariableY":            variableY,
		"TestType":             testType,
		"RelationshipStrength": relationshipStrength,
		"StrengthDescription":  strengthDescription,
		"ConfidenceLevel":      confidenceLevel,
		"MissingData":          missingDataPct,
		"MissingDataQuality":   missingDataQuality,
		"DataCompleteness":     dataCompleteness,
		"CompletenessQuality":  completenessQuality,
		"MissingRateX":         missingRateX,
		"MissingRateY":         missingRateY,
		"MissingRateXPct":      fmt.Sprintf("%.1f", missingRateX*100),
		"MissingRateYPct":      fmt.Sprintf("%.1f", missingRateY*100),
		"UniqueCountX":         uniqueCountX,
		"UniqueCountY":         uniqueCountY,
		"VarianceX":            varianceX,
		"VarianceY":            varianceY,
		"CardinalityX":         cardinalityX,
		"CardinalityY":         cardinalityY,
		"TimePeriod":           timePeriod,
		"Warnings":             warnings,
		"HasData":              hasData,
		"HasQValue":            hasQValue,
		"TotalComparisons":     totalComparisons,
		"Verdict":              determineVerdict(pValue, qValue, effectSize, sampleSize, hasQValue),
		// New fields for evidence-first UI
		"PairsAttempted":    pairsAttempted,
		"PairsTested":       pairsTested,
		"PairsSkipped":      pairsSkipped,
		"PairsPassed":       significantCount,
		"SignificanceRule":  significanceRule,
		"Seed":              seed,
		"Fingerprint":       fingerprint,
		"StageStatuses":     stageStatuses,
		"VariablesTotal":    len(allFields),
		"VariablesEligible": len(allFields), // TODO: extract from artifacts
		"VariablesRejected": 0,              // TODO: extract from artifacts
	}
	a.renderTemplate(w, "validation.html", data)
}
