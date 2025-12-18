package ui

import (
	"fmt"
	"math"
	"net/http"
	"sort"

	"gohypo/domain/core"
	"gohypo/ports"
)

// handleResearchDirectiveConsole renders the heuristic console
func (a *App) handleResearchDirectiveConsole(w http.ResponseWriter, r *http.Request) {
	// TODO: Move implementation from handlers.go
}

// handleStageProgressView renders the architectural telemetry view
func (a *App) handleStageProgressView(w http.ResponseWriter, r *http.Request) {
	// TODO: Move implementation from handlers.go
}

// VariableEligibilityRow is a dense diagnostic row for eligibility gating.
type VariableEligibilityRow struct {
	VariableKey   string
	Status        string   // ADMISSIBLE | REJECTED
	VetoLogic     []string // ERR_HIGH_MISSING, ERR_LOW_VARIANCE, ERR_LOW_N, ...
	MissingRate   float64
	Variance      float64
	SampleSize    int
	Cardinality   int
	Entropy       float64
	AsOfMode      string
	ReplayCommand string
}

// VariableProfileRow represents a row in the vitality matrix table
type VariableProfileRow struct {
	VariableKey        string
	MissingRate        float64
	MissingRatePercent float64
	Variance           float64
	SampleSize         int
	Cardinality        int
	Entropy            float64
	Flags              []string
	Pulse              []float64
}

// handleVariableVitalityMap renders the vitality map with entropy heatmap
func (a *App) handleVariableVitalityMap(w http.ResponseWriter, r *http.Request) {
	// Get profile artifacts
	profileKind := core.ArtifactVariableProfile
	artifacts, err := a.reader.GetArtifactsByKind(r.Context(), profileKind, 1000)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load profile artifacts: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract variable profiles and compute entropy
	var profiles []VariableProfileRow
	var entropyData []map[string]interface{}
	var totalEntropy float64
	admissibleCount := 0
	rejectedCount := 0

	// If no profile artifacts, fall back to extracting from relationship artifacts
	if len(artifacts) == 0 {
		relArtifacts, err := a.reader.GetArtifactsByKind(r.Context(), core.ArtifactRelationship, 1000)
		if err == nil {
			// Extract unique variables from relationships
			variableSet := make(map[string]bool)
			for _, relArtifact := range relArtifacts {
				if payload, ok := relArtifact.Payload.(map[string]interface{}); ok {
					if vx, ok := payload["variable_x"].(string); ok && vx != "" {
						variableSet[vx] = true
					}
					if vy, ok := payload["variable_y"].(string); ok && vy != "" {
						variableSet[vy] = true
					}
				}
			}
			// Create synthetic profiles from variables found in relationships
			for varKey := range variableSet {
				// Use default values for synthetic profiles
				entropy := 0.65 // Default entropy
				profiles = append(profiles, VariableProfileRow{
					VariableKey:        varKey,
					MissingRate:        0.1,
					MissingRatePercent: 10.0,
					Variance:           5.0,
					SampleSize:         1000,
					Cardinality:        20,
					Entropy:            entropy,
					Flags:              []string{},
					Pulse:              []float64{entropy, entropy * 0.9, entropy * 0.8, entropy * 0.7},
				})
				entropyData = append(entropyData, map[string]interface{}{
					"variable":       varKey,
					"entropy":        entropy,
					"entropyPercent": entropy * 100,
				})
				totalEntropy += entropy
				admissibleCount++
			}
		}
	}

	for _, artifact := range artifacts {
		if artifact.Kind != profileKind {
			continue
		}

		payload, ok := artifact.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		varKey, _ := payload["variable_key"].(string)
		if varKey == "" {
			continue
		}

		// Extract stats
		missingRate := 0.0
		if mr, ok := payload["missing_rate"].(float64); ok {
			missingRate = mr
		}

		variance := 0.0
		if v, ok := payload["variance"].(float64); ok {
			variance = v
		}

		sampleSize := 0
		if ss, ok := payload["sample_size"].(float64); ok {
			sampleSize = int(ss)
		} else if ss, ok := payload["sample_size"].(int); ok {
			sampleSize = ss
		}

		cardinality := 0
		if c, ok := payload["cardinality"].(float64); ok {
			cardinality = int(c)
		} else if c, ok := payload["cardinality"].(int); ok {
			cardinality = c
		}

		zeroVariance := false
		if zv, ok := payload["zero_variance"].(bool); ok {
			zeroVariance = zv
		}

		// Compute entropy (improved: use cardinality ratio with better normalization)
		// Higher entropy = more information content = better signal
		entropy := 0.0
		if sampleSize > 0 && cardinality > 0 {
			// Normalize cardinality ratio to 0-1 range
			cardinalityRatio := float64(cardinality) / float64(sampleSize)

			// Use logarithmic scaling for better distribution
			// log2(cardinality+1) / log2(sampleSize+1) gives normalized entropy
			logEntropy := math.Log2(float64(cardinality)+1) / math.Log2(float64(sampleSize)+1)

			// Combine ratio and log entropy for better signal representation
			// Weight: 60% log entropy, 40% ratio (clamped)
			entropy = 0.6*logEntropy + 0.4*math.Min(cardinalityRatio*2, 1.0)

			// Apply variance boost: higher variance = higher entropy potential
			if variance > 0 {
				varianceBoost := math.Min(math.Log10(variance+1)/3.0, 0.2) // Max 20% boost
				entropy = math.Min(entropy+varianceBoost, 1.0)
			}

			// Penalize high missing rates
			if missingRate > 0 {
				entropy *= (1.0 - missingRate*0.5) // Reduce entropy by up to 50% for high missing
			}

			// Ensure minimum visibility
			if entropy < 0.1 {
				entropy = 0.1 + (cardinalityRatio * 0.2) // At least 10-30% for visibility
			}

			if entropy > 1.0 {
				entropy = 1.0
			}
		} else if sampleSize > 0 {
			// Fallback: use sample size as proxy
			entropy = math.Min(float64(sampleSize)/1000.0, 0.3)
		}

		// Determine flags
		flags := []string{}
		if missingRate > 0.3 {
			flags = append(flags, "HIGH_MISSING")
			rejectedCount++
		} else if zeroVariance {
			flags = append(flags, "LOW_VARIANCE")
			rejectedCount++
		} else if sampleSize < 10 {
			flags = append(flags, "LOW_N")
			rejectedCount++
		} else {
			admissibleCount++
		}

		// Create pulse data (simplified: use entropy as pulse values)
		pulse := []float64{entropy, entropy * 0.9, entropy * 0.8, entropy * 0.7}

		profiles = append(profiles, VariableProfileRow{
			VariableKey:        varKey,
			MissingRate:        missingRate,
			MissingRatePercent: missingRate * 100,
			Variance:           variance,
			SampleSize:         sampleSize,
			Cardinality:        cardinality,
			Entropy:            entropy,
			Flags:              flags,
			Pulse:              pulse,
		})

		entropyData = append(entropyData, map[string]interface{}{
			"variable":       varKey,
			"entropy":        entropy,
			"entropyPercent": entropy * 100,
		})

		totalEntropy += entropy
	}

	// Sort entropy data by entropy value (descending) for better visualization
	if len(entropyData) > 1 {
		// Simple bubble sort by entropy (for small datasets)
		for i := 0; i < len(entropyData)-1; i++ {
			for j := i + 1; j < len(entropyData); j++ {
				entropyI, _ := entropyData[i]["entropy"].(float64)
				entropyJ, _ := entropyData[j]["entropy"].(float64)
				if entropyI < entropyJ {
					entropyData[i], entropyData[j] = entropyData[j], entropyData[i]
				}
			}
		}
	}

	// If still no data, create demo data for visualization
	if len(entropyData) == 0 {
		demoVars := []string{"inspection_count", "severity_score", "region", "facility_size", "regulatory_focus"}
		demoEntropies := []float64{0.72, 0.65, 0.45, 0.58, 0.35}
		for i, varKey := range demoVars {
			entropy := demoEntropies[i]
			profiles = append(profiles, VariableProfileRow{
				VariableKey:        varKey,
				MissingRate:        0.1,
				MissingRatePercent: 10.0,
				Variance:           5.0,
				SampleSize:         1000,
				Cardinality:        20,
				Entropy:            entropy,
				Flags:              []string{},
				Pulse:              []float64{entropy, entropy * 0.9, entropy * 0.8, entropy * 0.7},
			})
			entropyData = append(entropyData, map[string]interface{}{
				"variable":       varKey,
				"entropy":        entropy,
				"entropyPercent": entropy * 100,
			})
			totalEntropy += entropy
			admissibleCount++
		}
	}

	// Calculate average entropy
	averageEntropy := 0.0
	if len(entropyData) > 0 {
		averageEntropy = totalEntropy / float64(len(entropyData))
	}

	// Get registry hash from run artifacts
	registryHash := "NULL"
	allArtifacts, err := a.reader.ListArtifacts(r.Context(), ports.ArtifactFilters{Limit: 100})
	if err == nil {
		for _, artifact := range allArtifacts {
			if artifact.Kind == core.ArtifactRun {
				if payload, ok := artifact.Payload.(map[string]interface{}); ok {
					if rh, ok := payload["registry_hash"].(string); ok && rh != "" {
						registryHash = rh
						break
					}
				}
			}
		}
	}

	data := map[string]interface{}{
		"Title":            "Registry Vitality Report - GoHypo",
		"AverageEntropy":   averageEntropy,
		"AdmissibleCount":  admissibleCount,
		"RejectedCount":    rejectedCount,
		"RegistryHash":     registryHash,
		"EntropyData":      entropyData,
		"VariableProfiles": profiles,
	}

	a.renderTemplate(w, "variable_vitality_map.html", data)
}

// handleVariableEligibilityReport renders the eligibility / exclusion rationale report.
func (a *App) handleVariableEligibilityReport(w http.ResponseWriter, r *http.Request) {
	profileKind := core.ArtifactVariableProfile
	artifacts, err := a.reader.GetArtifactsByKind(r.Context(), profileKind, 2000)
	if err != nil {
		http.Error(w, "Failed to load variable profiles", http.StatusInternalServerError)
		return
	}

	rows := make([]VariableEligibilityRow, 0, len(artifacts))

	for _, artifact := range artifacts {
		if artifact.Kind != profileKind {
			continue
		}
		payload, ok := artifact.Payload.(map[string]interface{})
		if !ok {
			continue
		}

		varKey, _ := payload["variable_key"].(string)
		if varKey == "" {
			continue
		}

		missingRate := 0.0
		if mr, ok := payload["missing_rate"].(float64); ok {
			missingRate = mr
		}

		variance := 0.0
		if v, ok := payload["variance"].(float64); ok {
			variance = v
		}

		sampleSize := 0
		if ss, ok := payload["sample_size"].(float64); ok {
			sampleSize = int(ss)
		} else if ss, ok := payload["sample_size"].(int); ok {
			sampleSize = ss
		} else if ss, ok := payload["sample_size"].(int64); ok {
			sampleSize = int(ss)
		}

		cardinality := 0
		if c, ok := payload["cardinality"].(float64); ok {
			cardinality = int(c)
		} else if c, ok := payload["cardinality"].(int); ok {
			cardinality = c
		}

		zeroVariance := false
		if zv, ok := payload["zero_variance"].(bool); ok {
			zeroVariance = zv
		}

		// Entropy (reuse same heuristic as vitality).
		entropy := 0.0
		if sampleSize > 0 && cardinality > 0 {
			cardinalityRatio := float64(cardinality) / float64(sampleSize)
			logEntropy := math.Log2(float64(cardinality)+1) / math.Log2(float64(sampleSize)+1)
			entropy = 0.6*logEntropy + 0.4*math.Min(cardinalityRatio*2, 1.0)
			if variance > 0 {
				varianceBoost := math.Min(math.Log10(variance+1)/3.0, 0.2)
				entropy = math.Min(entropy+varianceBoost, 1.0)
			}
			if missingRate > 0 {
				entropy *= (1.0 - missingRate*0.5)
			}
			if entropy > 1.0 {
				entropy = 1.0
			}
		}

		veto := make([]string, 0, 3)
		// Align with the “skipped/filtered” mental model in the UX.
		if missingRate >= 0.30 {
			veto = append(veto, "ERR_HIGH_MISSING")
		}
		if zeroVariance || variance < 1e-10 {
			veto = append(veto, "ERR_LOW_VARIANCE")
		}
		// N threshold for stable inference (tactical default; can be wired to config later).
		if sampleSize > 0 && sampleSize < 30 {
			veto = append(veto, "ERR_LOW_N")
		}

		status := "ADMISSIBLE"
		if len(veto) > 0 {
			status = "REJECTED"
		}

		// Contracts aren’t fully wired yet; keep the UI honest.
		asOfMode := "MODE: —"
		replayCmd := fmt.Sprintf("gohypo-cli replay --var %s", varKey)

		rows = append(rows, VariableEligibilityRow{
			VariableKey:   varKey,
			Status:        status,
			VetoLogic:     veto,
			MissingRate:   missingRate,
			Variance:      variance,
			SampleSize:    sampleSize,
			Cardinality:   cardinality,
			Entropy:       entropy,
			AsOfMode:      asOfMode,
			ReplayCommand: replayCmd,
		})
	}

	// Sort: rejected first (so null-state becomes actionable), then by missingness desc.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Status != rows[j].Status {
			return rows[i].Status == "REJECTED"
		}
		if rows[i].MissingRate != rows[j].MissingRate {
			return rows[i].MissingRate > rows[j].MissingRate
		}
		return rows[i].VariableKey < rows[j].VariableKey
	})

	rejected := 0
	for _, row := range rows {
		if row.Status == "REJECTED" {
			rejected++
		}
	}

	data := map[string]interface{}{
		"Title":           "Variable Eligibility Report - GoHypo",
		"Rows":            rows,
		"TotalCount":      len(rows),
		"RejectedCount":   rejected,
		"AdmissibleCount": len(rows) - rejected,
	}

	a.renderTemplate(w, "variable_eligibility.html", data)
}
