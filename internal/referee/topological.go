package referee

import (
	"fmt"
	"math"
)

// minMaxNormalize scales data to [targetMin, targetMax] range
// Prevents dimensional dominance in topological analysis where variables
// with different scales (e.g., dollars vs. goals) would crush the manifold
func minMaxNormalize(data []float64, targetMin, targetMax float64) []float64 {
	if len(data) == 0 {
		return []float64{}
	}

	// Find current range
	minVal := data[0]
	maxVal := data[0]
	for _, v := range data {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	// Handle constant data
	if maxVal == minVal {
		// Return targetMin for all values
		result := make([]float64, len(data))
		for i := range result {
			result[i] = targetMin
		}
		return result
	}

	// Apply min-max normalization
	result := make([]float64, len(data))
	for i, v := range data {
		normalized := (v - minVal) / (maxVal - minVal)
		result[i] = targetMin + normalized*(targetMax-targetMin)
	}

	return result
}

// PersistentHomology implements topological data analysis via persistent homology
type PersistentHomology struct {
	MaxDimension int
	MaxEpsilon   float64
	NumSteps     int
}

// Execute analyzes topological features of the relationship using persistent homology
func (ph *PersistentHomology) Execute(x, y []float64, metadata map[string]interface{}) RefereeResult {
	if err := ValidateData(x, y); err != nil {
		return RefereeResult{
			GateName:      "Persistent_Homology",
			Passed:        false,
			FailureReason: err.Error(),
		}
	}

	if ph.MaxDimension == 0 {
		ph.MaxDimension = 2
	}
	if ph.MaxEpsilon == 0 {
		ph.MaxEpsilon = 2.0
	}
	if ph.NumSteps == 0 {
		ph.NumSteps = 20
	}

	// CRITICAL: Normalize to unit cube [0,1] to prevent scaling artifacts
	// Variables with different ranges would otherwise crush the topological manifold
	points := ph.createPointCloud(x, y)

	// Compute persistence diagrams
	diagrams := ph.computePersistenceDiagrams(points, ph.MaxDimension, ph.MaxEpsilon, ph.NumSteps)

	// Analyze topological signatures
	topologyScore := ph.analyzeTopologicalSignatures(diagrams)

	// Bootstrap for statistical significance
	topologyScores := ph.bootstrapTopology(points, ph.MaxDimension, ph.MaxEpsilon, ph.NumSteps, 100)
	pValue := ph.computeTopologyPValue(topologyScores, topologyScore)

	// Apply centralized standard: persistence ratio ≥ 3.0 (signal 3x stronger than noise)
	passed := topologyScore >= PERSISTENCE_NOISE_RATIO && pValue < 0.01

	failureReason := ""
	if !passed {
		if topologyScore < 1.5 {
			failureReason = fmt.Sprintf("NO TOPOLOGICAL STRUCTURE: Data appears as featureless point cloud (ratio=%.2f). No persistent topological features detected. Data may be random or relationship too weak for geometric analysis.", topologyScore)
		} else if topologyScore < PERSISTENCE_NOISE_RATIO {
			failureReason = fmt.Sprintf("WEAK TOPOLOGICAL SIGNAL: Some structure detected but signal-to-noise ratio too low (ratio=%.2f < %.1f). Topological features may exist but are obscured by noise or insufficient sample size.", topologyScore, PERSISTENCE_NOISE_RATIO)
		} else {
			failureReason = fmt.Sprintf("INSUFFICIENT TOPOLOGICAL CONFIDENCE: Structure detected but statistical uncertainty too high (p=%.4f). May have genuine topological patterns but requires larger sample or cleaner data.", pValue)
		}
	}

	return RefereeResult{
		GateName:      "Persistent_Homology",
		Passed:        passed,
		Statistic:     topologyScore,
		PValue:        pValue,
		StandardUsed:  fmt.Sprintf("Persistence ratio ≥ %.1f (signal:noise)", PERSISTENCE_NOISE_RATIO),
		FailureReason: failureReason,
	}
}

func (ph *PersistentHomology) createPointCloud(x, y []float64) [][]float64 {
	// CRITICAL: Normalize to unit cube [0,1] to prevent scaling artifacts
	// Variables with different ranges (e.g., dollars vs. goals) would otherwise
	// crush the topological manifold and make higher-order "holes" invisible
	xNorm := minMaxNormalize(x, 0.0, 1.0)
	yNorm := minMaxNormalize(y, 0.0, 1.0)

	points := make([][]float64, len(x))
	for i := range points {
		points[i] = []float64{xNorm[i], yNorm[i]}
	}
	return points
}

func (ph *PersistentHomology) computePersistenceDiagrams(points [][]float64, maxDim int, maxEps float64, numSteps int) []PersistenceDiagram {
	diagrams := make([]PersistenceDiagram, maxDim+1)

	epsStep := maxEps / float64(numSteps-1)

	for dim := 0; dim <= maxDim; dim++ {
		diagram := PersistenceDiagram{Dimension: dim}

		// Simplified persistence computation using Vietoris-Rips filtration
		for step := 0; step < numSteps; step++ {
			epsilon := float64(step) * epsStep

			// Detect topological features at this scale
			features := ph.detectFeaturesAtScale(points, dim, epsilon)

			for _, feature := range features {
				if feature.Birth <= epsilon && feature.Death > epsilon {
					// Feature is alive at this scale
					feature.Death = epsilon // Will be updated if it persists
					diagram.Features = append(diagram.Features, feature)
				}
			}
		}

		// Sort features by persistence (death - birth)
		for i := 0; i < len(diagram.Features)-1; i++ {
			for j := i + 1; j < len(diagram.Features); j++ {
				persistI := diagram.Features[i].Death - diagram.Features[i].Birth
				persistJ := diagram.Features[j].Death - diagram.Features[j].Birth
				if persistJ > persistI {
					diagram.Features[i], diagram.Features[j] = diagram.Features[j], diagram.Features[i]
				}
			}
		}

		diagrams[dim] = diagram
	}

	return diagrams
}

type PersistenceDiagram struct {
	Dimension int
	Features  []TopologicalFeature
}

type TopologicalFeature struct {
	Birth       float64
	Death       float64
	Persistence float64
}

func (ph *PersistentHomology) detectFeaturesAtScale(points [][]float64, dimension int, epsilon float64) []TopologicalFeature {
	features := []TopologicalFeature{}

	switch dimension {
	case 0: // Connected components (H0)
		// Simplified: count connected components in Rips complex
		components := ph.countConnectedComponents(points, epsilon)
		// In real PH, this would track births/deaths, but we simplify
		for i := 0; i < components; i++ {
			features = append(features, TopologicalFeature{
				Birth:       0,
				Death:       epsilon,
				Persistence: epsilon,
			})
		}

	case 1: // Loops/cycles (H1)
		// Simplified: detect potential cycles
		if len(points) >= 6 { // Need minimum points for cycles
			cycles := ph.detectCycles(points, epsilon)
			for range cycles {
				features = append(features, TopologicalFeature{
					Birth:       epsilon * 0.5,
					Death:       epsilon,
					Persistence: epsilon * 0.5,
				})
			}
		}

	case 2: // Voids (H2)
		// Simplified: detect convex hull voids
		if len(points) >= 8 {
			voids := ph.detectVoids(points, epsilon)
			for range voids {
				features = append(features, TopologicalFeature{
					Birth:       epsilon * 0.7,
					Death:       epsilon,
					Persistence: epsilon * 0.3,
				})
			}
		}
	}

	return features
}

func (ph *PersistentHomology) countConnectedComponents(points [][]float64, epsilon float64) int {
	n := len(points)
	visited := make([]bool, n)
	components := 0

	for i := 0; i < n; i++ {
		if !visited[i] {
			// Start DFS/BFS from unvisited point
			ph.dfsVisit(points, visited, i, epsilon)
			components++
		}
	}

	return components
}

func (ph *PersistentHomology) dfsVisit(points [][]float64, visited []bool, start int, epsilon float64) {
	stack := []int{start}

	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if visited[current] {
			continue
		}
		visited[current] = true

		// Add neighbors within epsilon
		for i, point := range points {
			if !visited[i] && ph.euclideanDistance(points[current], point) <= epsilon {
				stack = append(stack, i)
			}
		}
	}
}

func (ph *PersistentHomology) detectCycles(points [][]float64, epsilon float64) []bool {
	// Simplified cycle detection - look for points that form closed loops
	cycles := []bool{}

	// Check if points form a rough circle
	if ph.isRoughlyCircular(points, epsilon) {
		cycles = append(cycles, true)
	}

	return cycles
}

func (ph *PersistentHomology) detectVoids(points [][]float64, epsilon float64) []bool {
	voids := []bool{}

	// Simplified: check if points leave significant empty space
	area := ph.convexHullArea(points)
	if area > 0 {
		density := float64(len(points)) / area
		if density < 0.1 { // Low density suggests voids
			voids = append(voids, true)
		}
	}

	return voids
}

func (ph *PersistentHomology) euclideanDistance(a, b []float64) float64 {
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}

func (ph *PersistentHomology) isRoughlyCircular(points [][]float64, epsilon float64) bool {
	if len(points) < 6 {
		return false
	}

	// Compute centroid
	centroid := make([]float64, len(points[0]))
	for _, point := range points {
		for i, coord := range point {
			centroid[i] += coord
		}
	}
	for i := range centroid {
		centroid[i] /= float64(len(points))
	}

	// Check if distances from centroid are similar
	distances := make([]float64, len(points))
	for i, point := range points {
		distances[i] = ph.euclideanDistance(centroid, point)
	}

	meanDist := 0.0
	for _, dist := range distances {
		meanDist += dist
	}
	meanDist /= float64(len(distances))

	// Check variance in distances
	variance := 0.0
	for _, dist := range distances {
		diff := dist - meanDist
		variance += diff * diff
	}
	variance /= float64(len(distances))

	// Low variance suggests circular arrangement
	return variance < epsilon*epsilon
}

func (ph *PersistentHomology) convexHullArea(points [][]float64) float64 {
	if len(points) < 3 {
		return 0
	}

	// Simple approximation using bounding box for 2D points
	minX, maxX := points[0][0], points[0][0]
	minY, maxY := points[0][1], points[0][1]

	for _, point := range points {
		if point[0] < minX {
			minX = point[0]
		}
		if point[0] > maxX {
			maxX = point[0]
		}
		if point[1] < minY {
			minY = point[1]
		}
		if point[1] > maxY {
			maxY = point[1]
		}
	}

	width := maxX - minX
	height := maxY - minY

	return width * height
}

func (ph *PersistentHomology) analyzeTopologicalSignatures(diagrams []PersistenceDiagram) float64 {
	totalScore := 0.0
	totalFeatures := 0

	for _, diagram := range diagrams {
		for _, feature := range diagram.Features {
			persistence := feature.Death - feature.Birth
			if persistence > 0 {
				// Weight by persistence and dimension
				weight := persistence * math.Pow(0.5, float64(diagram.Dimension))
				totalScore += weight
				totalFeatures++
			}
		}
	}

	if totalFeatures == 0 {
		return 0
	}

	// Normalize score (0-1 scale)
	return math.Min(1.0, totalScore/float64(totalFeatures*2))
}

func (ph *PersistentHomology) bootstrapTopology(points [][]float64, maxDim int, maxEps float64, numSteps, nBootstrap int) []float64 {
	scores := make([]float64, nBootstrap)
	n := len(points)

	for i := 0; i < nBootstrap; i++ {
		// Bootstrap sample of points
		bootPoints := make([][]float64, n)
		for j := 0; j < n; j++ {
			idx := int(math.Floor(float64(n) * math.Sqrt(float64(i*j%n))))
			if idx >= n {
				idx = n - 1
			}
			bootPoints[j] = make([]float64, len(points[idx]))
			copy(bootPoints[j], points[idx])
		}

		// Compute topological score for bootstrap sample
		diagrams := ph.computePersistenceDiagrams(bootPoints, maxDim, maxEps, numSteps)
		scores[i] = ph.analyzeTopologicalSignatures(diagrams)
	}

	return scores
}

func (ph *PersistentHomology) computeTopologyPValue(bootstrapScores []float64, observedScore float64) float64 {
	count := 0
	for _, score := range bootstrapScores {
		if score >= observedScore {
			count++
		}
	}
	return float64(count) / float64(len(bootstrapScores))
}

// AuditEvidence performs evidence auditing for persistent homology using discovery q-values
func (ph *PersistentHomology) AuditEvidence(discoveryEvidence interface{}, validationData []float64, metadata map[string]interface{}) RefereeResult {
	// Persistent homology is about topological structure - use default audit logic
	// since topological analysis requires manifold embedding that's hard to audit from q-values alone
	return DefaultAuditEvidence("Persistent_Homology", discoveryEvidence, validationData, metadata)
}
