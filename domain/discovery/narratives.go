package discovery

import (
	"math"
	"sort"

	"gohypo/domain/core"
	"gohypo/domain/stats"
)

// ============================================================================
// BEHAVIORAL NARRATIVE DETECTION
// ============================================================================

// DetectSilenceAcceleration identifies when variables suddenly stop moving together
// This indicates potential intervention, regime change, or structural break
func DetectSilenceAcceleration(
	relationships []stats.RelationshipArtifact,
	variableKey core.VariableKey,
	timeWindow int, // Number of recent periods to analyze
) SilenceAcceleration {

	if len(relationships) < timeWindow*2 {
		return SilenceAcceleration{Detected: false, Confidence: 0.0}
	}

	// Extract correlation history for this variable
	correlations := extractCorrelationHistory(relationships, variableKey)
	if len(correlations) < timeWindow*2 {
		return SilenceAcceleration{Detected: false, Confidence: 0.0}
	}

	// Split into recent vs historical periods
	recent := correlations[:timeWindow]
	historical := correlations[timeWindow:]

	// Calculate average correlations
	recentAvg := mean(recent)
	historicalAvg := mean(historical)

	// Calculate acceleration rate (how quickly correlation is dropping)
	accelerationRate := historicalAvg - recentAvg
	if accelerationRate <= 0 {
		return SilenceAcceleration{Detected: false, Confidence: 0.0}
	}

	// Count periods of "silence" (very low correlation)
	silenceThreshold := 0.1 // |correlation| < 0.1 considered silent
	silenceCount := 0
	for _, corr := range recent {
		if math.Abs(corr) < silenceThreshold {
			silenceCount++
		}
	}

	detected := accelerationRate > 0.2 && silenceCount >= timeWindow/2
	confidence := min(accelerationRate*2.0, 1.0) * (float64(silenceCount) / float64(timeWindow))

	return SilenceAcceleration{
		Detected:         detected,
		AccelerationRate: accelerationRate,
		SilencePeriod:    silenceCount,
		Confidence:       confidence,
	}
}

// DetectBlastRadius measures how much a variable's change affects the broader system
// This identifies central variables and potential cascade effects
func DetectBlastRadius(
	relationships []stats.RelationshipArtifact,
	variableKey core.VariableKey,
	minEffectSize float64, // Minimum effect size to consider
) BlastRadius {

	affectedVars := make(map[core.VariableKey]bool)
	totalEffect := 0.0
	strongConnections := 0

	// Find all relationships involving this variable
	for _, rel := range relationships {
		var connectedVar core.VariableKey

		if rel.Key.VariableX == variableKey {
			connectedVar = rel.Key.VariableY
		} else if rel.Key.VariableY == variableKey {
			connectedVar = rel.Key.VariableX
		} else {
			continue // Not related to our variable
		}

		effectSize := math.Abs(rel.Metrics.EffectSize)
		if effectSize >= minEffectSize {
			affectedVars[connectedVar] = true
			totalEffect += effectSize
			strongConnections++

			// Check for potential domino effects (secondary connections)
			for _, rel2 := range relationships {
				if rel2.Key.VariableX == connectedVar || rel2.Key.VariableY == connectedVar {
					if rel2.Key.VariableX != variableKey && rel2.Key.VariableY != variableKey {
						// This creates a chain: variable -> connectedVar -> otherVar
						affectedVars[rel2.Key.VariableX] = true
						affectedVars[rel2.Key.VariableY] = true
					}
				}
			}
		}
	}

	// Remove self-reference
	delete(affectedVars, variableKey)

	affectedList := make([]core.VariableKey, 0, len(affectedVars))
	for v := range affectedVars {
		affectedList = append(affectedList, v)
	}

	// Calculate centrality score (normalized effect strength)
	centralityScore := min(totalEffect/float64(max(1, strongConnections)), 1.0)

	// Calculate radius score (breadth of impact)
	radiusScore := min(float64(len(affectedList))/10.0, 1.0) // Assume max 10 connections

	// Detect domino effect (has secondary connections)
	dominoEffect := len(affectedList) > strongConnections

	// Calculate path length (longest chain)
	pathLength := calculateMaxPathLength(relationships, variableKey, 3) // Max depth 3

	return BlastRadius{
		RadiusScore:       (centralityScore + radiusScore) / 2.0,
		AffectedVariables: affectedList,
		DominoEffect:      dominoEffect,
		CentralityScore:   centralityScore,
		PathLength:        pathLength,
	}
}

// DetectTwinSegments identifies nearly identical behavioral segments
// This reveals redundancy and potential confounding
func DetectTwinSegments(
	relationships []stats.RelationshipArtifact,
	minSimilarity float64, // Minimum similarity threshold
) TwinSegments {

	if len(relationships) < 2 {
		return TwinSegments{Detected: false}
	}

	// Build relationship profiles for each variable
	varProfiles := buildVariableProfiles(relationships)

	// Find similar pairs
	var pairs []SegmentPair
	totalSimilarity := 0.0

	for i := 0; i < len(varProfiles)-1; i++ {
		for j := i + 1; j < len(varProfiles); j++ {
			var1, profile1 := varProfiles[i].Variable, varProfiles[i].Profile
			var2, profile2 := varProfiles[j].Variable, varProfiles[j].Profile

			similarity := calculateProfileSimilarity(profile1, profile2)
			if similarity >= minSimilarity {
				overlapCount := countSharedRelationships(profile1, profile2)
				pairs = append(pairs, SegmentPair{
					Segment1:     var1,
					Segment2:     var2,
					Similarity:   similarity,
					OverlapCount: overlapCount,
				})
				totalSimilarity += similarity
			}
		}
	}

	detected := len(pairs) > 0
	avgSimilarity := 0.0
	if len(pairs) > 0 {
		avgSimilarity = totalSimilarity / float64(len(pairs))
	}

	// Assess redundancy risk
	redundancyRisk := "low"
	if len(pairs) > 3 {
		redundancyRisk = "high"
	} else if len(pairs) > 1 {
		redundancyRisk = "medium"
	}

	// Calculate confounding risk (high similarity suggests potential confounding)
	confoundingRisk := min(avgSimilarity*2.0, 1.0)

	return TwinSegments{
		Detected:        detected,
		SegmentPairs:    pairs,
		SimilarityScore: avgSimilarity,
		RedundancyRisk:  redundancyRisk,
		ConfoundingRisk: confoundingRisk,
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// VariableProfile represents a variable's relationship pattern
type VariableProfile struct {
	Variable core.VariableKey
	Profile  map[core.VariableKey]float64 // Connected variable -> effect size
}

// extractCorrelationHistory pulls historical correlations for a variable
func extractCorrelationHistory(relationships []stats.RelationshipArtifact, variableKey core.VariableKey) []float64 {
	var correlations []float64

	for _, rel := range relationships {
		if rel.Key.VariableX == variableKey || rel.Key.VariableY == variableKey {
			correlations = append(correlations, rel.Metrics.EffectSize)
		}
	}

	// Sort by discovery time (assuming more recent first)
	sort.Slice(correlations, func(i, j int) bool {
		return correlations[i] > correlations[j] // Descending order
	})

	return correlations
}

// buildVariableProfiles creates relationship profiles for all variables
func buildVariableProfiles(relationships []stats.RelationshipArtifact) []VariableProfile {
	profiles := make(map[core.VariableKey]map[core.VariableKey]float64)

	// Build profiles
	for _, rel := range relationships {
		// Add to X's profile
		if profiles[rel.Key.VariableX] == nil {
			profiles[rel.Key.VariableX] = make(map[core.VariableKey]float64)
		}
		profiles[rel.Key.VariableX][rel.Key.VariableY] = rel.Metrics.EffectSize

		// Add to Y's profile
		if profiles[rel.Key.VariableY] == nil {
			profiles[rel.Key.VariableY] = make(map[core.VariableKey]float64)
		}
		profiles[rel.Key.VariableY][rel.Key.VariableX] = rel.Metrics.EffectSize
	}

	// Convert to slice
	var result []VariableProfile
	for variable, profile := range profiles {
		result = append(result, VariableProfile{
			Variable: variable,
			Profile:  profile,
		})
	}

	return result
}

// calculateProfileSimilarity computes similarity between two relationship profiles
func calculateProfileSimilarity(profile1, profile2 map[core.VariableKey]float64) float64 {
	if len(profile1) == 0 && len(profile2) == 0 {
		return 1.0
	}

	// Find common variables
	commonVars := 0
	totalEffectDiff := 0.0
	maxPossibleCommon := min(float64(len(profile1)), float64(len(profile2)))

	for var1, effect1 := range profile1 {
		if effect2, exists := profile2[var1]; exists {
			commonVars++
			totalEffectDiff += math.Abs(effect1 - effect2)
		}
	}

	if commonVars == 0 {
		return 0.0
	}

	// Similarity based on overlap and effect size agreement
	overlapScore := float64(commonVars) / float64(maxPossibleCommon)
	effectAgreement := 1.0 - (totalEffectDiff / float64(commonVars)) // Normalize difference

	return (overlapScore + effectAgreement) / 2.0
}

// countSharedRelationships counts variables that both profiles relate to
func countSharedRelationships(profile1, profile2 map[core.VariableKey]float64) int {
	count := 0
	for var1 := range profile1 {
		if _, exists := profile2[var1]; exists {
			count++
		}
	}
	return count
}

// calculateMaxPathLength finds the longest chain of relationships from a starting variable
func calculateMaxPathLength(relationships []stats.RelationshipArtifact, startVar core.VariableKey, maxDepth int) int {
	// Build adjacency list
	graph := make(map[core.VariableKey][]core.VariableKey)
	for _, rel := range relationships {
		graph[rel.Key.VariableX] = append(graph[rel.Key.VariableX], rel.Key.VariableY)
		graph[rel.Key.VariableY] = append(graph[rel.Key.VariableY], rel.Key.VariableX)
	}

	// BFS to find maximum path length
	visited := make(map[core.VariableKey]bool)
	queue := []struct {
		variable core.VariableKey
		depth    int
	}{{startVar, 0}}

	maxLength := 0
	visited[startVar] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		maxLength = max(maxLength, current.depth)

		if current.depth >= maxDepth {
			continue
		}

		for _, neighbor := range graph[current.variable] {
			if !visited[neighbor] {
				visited[neighbor] = true
				queue = append(queue, struct {
					variable core.VariableKey
					depth    int
				}{neighbor, current.depth + 1})
			}
		}
	}

	return maxLength
}

// Utility functions
func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0.0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
