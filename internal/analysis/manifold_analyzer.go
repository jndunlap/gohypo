package analysis

import (
	"fmt"
	"math"
	"math/rand"

	"gohypo/models"
)

// ManifoldAnalyzer computes topological features for visualization
type ManifoldAnalyzer struct {
	// Configuration for manifold analysis
	embeddingDim int
	gridSize     int
}

// NewManifoldAnalyzer creates a new manifold analyzer
func NewManifoldAnalyzer() *ManifoldAnalyzer {
	return &ManifoldAnalyzer{
		embeddingDim: 2, // t-SNE/UMAP target dimension
		gridSize:     50, // Surface grid resolution
	}
}

// TopologicalData represents the computed manifold features for visualization
type TopologicalData struct {
	RelationshipSurface    *RelationshipSurface    `json:"relationship_surface"`
	DimensionalityProjection *DimensionalityProjection `json:"dimensionality_projection"`
	ManifoldTears          *ManifoldTears          `json:"manifold_tears"`
	HomologyAnalysis       *HomologyAnalysis       `json:"homology_analysis"`
	Summary                *TopologicalSummary     `json:"summary"`
}

// TopologicalSummary provides human-readable insights
type TopologicalSummary struct {
	TearCount        int     `json:"tear_count"`
	CollapseRisk     string  `json:"collapse_risk"` // "Low", "Medium", "High"
	HoleCount        int     `json:"hole_count"`
	ManifoldStability float64 `json:"manifold_stability"` // 0-1 score
	TopologicalInsights []string `json:"topological_insights"`
}

// RelationshipSurface represents 3D surface data for variable relationships
type RelationshipSurface struct {
	Points [][]float64 `json:"points"` // [x, y, z] coordinates
	GridSize int       `json:"grid_size"`
	TearRegions []TearRegion `json:"tear_regions"`
}

// TearRegion identifies areas where the manifold tears
type TearRegion struct {
	XMin, XMax, YMin, YMax float64
	Severity float64
	Description string
}

// DimensionalityProjection represents low-dimensional embedding
type DimensionalityProjection struct {
	Points []EmbeddedPoint `json:"points"`
	Clusters []ClusterInfo `json:"clusters"`
}

// EmbeddedPoint represents a point in reduced dimensional space
type EmbeddedPoint struct {
	X, Y float64
	OriginalIndex int
	ClusterID int
}

// ClusterInfo describes identified clusters
type ClusterInfo struct {
	ID int
	Center [2]float64
	Size int
	SeparationScore float64
}

// ManifoldTears identifies relationship discontinuities
type ManifoldTears struct {
	Tears []TearPoint `json:"tears"`
	Curve []CurvePoint `json:"curve"` // Continuous relationship curve
	DiscontinuityScore float64 `json:"discontinuity_score"`
}

// TearPoint represents a detected manifold tear
type TearPoint struct {
	Position float64
	Severity float64
	Description string
	Variable string
}

// CurvePoint represents a point on the relationship curve
type CurvePoint struct {
	X, Y float64
}

// HomologyAnalysis represents persistent homology results
type HomologyAnalysis struct {
	HomologyClasses []HomologyClass `json:"homology_classes"`
	BettiNumbers    []int           `json:"betti_numbers"`
	PersistenceThreshold float64    `json:"persistence_threshold"`
}

// HomologyClass represents a homology class
type HomologyClass struct {
	Dimension int     `json:"dimension"`
	Birth     float64 `json:"birth"`
	Death     float64 `json:"death"`
	Persistence float64 `json:"persistence"`
}

// AnalyzeHypothesis computes topological features for a hypothesis
func (ma *ManifoldAnalyzer) AnalyzeHypothesis(hypothesis *models.HypothesisResult, evidence *EvidenceBrief) *TopologicalData {
	data := &TopologicalData{}

	// Compute relationship surface from associations
	data.RelationshipSurface = ma.computeRelationshipSurface(evidence)

	// Compute dimensionality projection
	data.DimensionalityProjection = ma.computeDimensionalityProjection(evidence)

	// Detect manifold tears from breakpoints
	data.ManifoldTears = ma.detectManifoldTears(evidence)

	// Compute homology from structural breaks and hysteresis
	data.HomologyAnalysis = ma.computeHomology(evidence)

	// Generate summary insights
	data.Summary = ma.generateSummary(data)

	return data
}

// computeRelationshipSurface creates 3D surface from association evidence
func (ma *ManifoldAnalyzer) computeRelationshipSurface(evidence *EvidenceBrief) *RelationshipSurface {
	surface := &RelationshipSurface{
		GridSize: ma.gridSize,
		Points:   make([][]float64, 0),
		TearRegions: make([]TearRegion, 0),
	}

	// Generate grid points
	for i := 0; i < ma.gridSize; i++ {
		for j := 0; j < ma.gridSize; j++ {
			x := float64(i)/float64(ma.gridSize-1)*10 - 5 // Scale to [-5, 5]
			y := float64(j)/float64(ma.gridSize-1)*10 - 5

			// Compute z value based on associations and breakpoints
			z := ma.computeSurfaceHeight(x, y, evidence)

			surface.Points = append(surface.Points, []float64{x, y, z})
		}
	}

	// Identify tear regions from breakpoint evidence
	for _, bp := range evidence.Breakpoints {
		if bp.ConfidenceLevel == ConfidenceStrong || bp.ConfidenceLevel == ConfidenceVeryStrong {
			tear := TearRegion{
				XMin: bp.Threshold - 0.5,
				XMax: bp.Threshold + 0.5,
				YMin: -5,
				YMax: 5,
				Severity: bp.Delta,
				Description: fmt.Sprintf("Structural break at %s = %.2f", bp.Feature, bp.Threshold),
			}
			surface.TearRegions = append(surface.TearRegions, tear)
		}
	}

	return surface
}

// computeSurfaceHeight calculates z-value for surface plotting
func (ma *ManifoldAnalyzer) computeSurfaceHeight(x, y float64, evidence *EvidenceBrief) float64 {
	height := 0.0

	// Base surface from associations
	for _, assoc := range evidence.Associations {
		if assoc.ConfidenceLevel == ConfidenceStrong || assoc.ConfidenceLevel == ConfidenceVeryStrong {
			// Simple relationship modeling
			distance := math.Sqrt(x*x + y*y)
			height += assoc.RawEffect * math.Exp(-distance/2.0) * assoc.ScreeningScore
		}
	}

	// Add discontinuities from breakpoints
	for _, bp := range evidence.Breakpoints {
		if bp.ConfidenceLevel == ConfidenceStrong || bp.ConfidenceLevel == ConfidenceVeryStrong {
			threshold := bp.Threshold
			if x > threshold {
				height += bp.Delta * 0.5
			}
		}
	}

	// Add noise for realism
	height += rand.NormFloat64() * 0.1

	return height
}

// computeDimensionalityProjection creates t-SNE style embedding
func (ma *ManifoldAnalyzer) computeDimensionalityProjection(evidence *EvidenceBrief) *DimensionalityProjection {
	projection := &DimensionalityProjection{
		Points: make([]EmbeddedPoint, 0),
		Clusters: make([]ClusterInfo, 0),
	}

	// Simple clustering based on confidence levels
	clusters := make(map[int][]EmbeddedPoint)

	for i, assoc := range evidence.Associations {
		clusterID := 0
		if assoc.ConfidenceLevel == ConfidenceStrong || assoc.ConfidenceLevel == ConfidenceVeryStrong {
			clusterID = 1
		} else if assoc.ConfidenceLevel == ConfidenceModerate {
			clusterID = 2
		}

		// Simple positioning based on effect size
		x := assoc.RawEffect * 3
		y := float64(i%10) - 5

		point := EmbeddedPoint{
			X: x,
			Y: y,
			OriginalIndex: i,
			ClusterID: clusterID,
		}

		projection.Points = append(projection.Points, point)
		clusters[clusterID] = append(clusters[clusterID], point)
	}

	// Compute cluster info
	for clusterID, points := range clusters {
		if len(points) == 0 {
			continue
		}

		centerX, centerY := 0.0, 0.0
		for _, p := range points {
			centerX += p.X
			centerY += p.Y
		}
		centerX /= float64(len(points))
		centerY /= float64(len(points))

		cluster := ClusterInfo{
			ID: clusterID,
			Center: [2]float64{centerX, centerY},
			Size: len(points),
			SeparationScore: 1.0, // Simplified
		}
		projection.Clusters = append(projection.Clusters, cluster)
	}

	return projection
}

// detectManifoldTears identifies relationship discontinuities
func (ma *ManifoldAnalyzer) detectManifoldTears(evidence *EvidenceBrief) *ManifoldTears {
	tears := &ManifoldTears{
		Tears: make([]TearPoint, 0),
		Curve: make([]CurvePoint, 0),
		DiscontinuityScore: 0.0,
	}

	// Generate relationship curve
	for x := -5.0; x <= 5.0; x += 0.1 {
		y := ma.computeCurveHeight(x, evidence)
		tears.Curve = append(tears.Curve, CurvePoint{X: x, Y: y})
	}

	// Detect tears from breakpoints
	for _, bp := range evidence.Breakpoints {
		if bp.ConfidenceLevel == ConfidenceStrong || bp.ConfidenceLevel == ConfidenceVeryStrong {
			tear := TearPoint{
				Position: bp.Threshold,
				Severity: math.Abs(bp.Delta),
				Description: fmt.Sprintf("Phase transition at %s = %.2f", bp.Feature, bp.Threshold),
				Variable: bp.Feature,
			}
			tears.Tears = append(tears.Tears, tear)
			tears.DiscontinuityScore += tear.Severity
		}
	}

	// Normalize discontinuity score
	if len(tears.Tears) > 0 {
		tears.DiscontinuityScore /= float64(len(tears.Tears))
	}

	return tears
}

// computeCurveHeight calculates y-value for relationship curves
func (ma *ManifoldAnalyzer) computeCurveHeight(x float64, evidence *EvidenceBrief) float64 {
	height := math.Sin(x*0.5) + x*0.1 // Base relationship

	// Apply breakpoint effects
	for _, bp := range evidence.Breakpoints {
		if x > bp.Threshold {
			height += bp.Delta * 0.3
		}
	}

	return height
}

// computeHomology performs simplified persistent homology analysis
func (ma *ManifoldAnalyzer) computeHomology(evidence *EvidenceBrief) *HomologyAnalysis {
	homology := &HomologyAnalysis{
		HomologyClasses: make([]HomologyClass, 0),
		BettiNumbers:    []int{1, 0, 0}, // Default: 1 component, 0 loops, 0 voids
		PersistenceThreshold: 0.1,
	}

	// Dimension 0: Connected components (always at least 1)
	homology.HomologyClasses = append(homology.HomologyClasses, HomologyClass{
		Dimension: 0,
		Birth: 0,
		Death: 5,
		Persistence: 5,
	})

	// Dimension 1: Loops from hysteresis effects
	for _, hyst := range evidence.HysteresisEffects {
		if hyst.ConfidenceLevel == ConfidenceStrong || hyst.ConfidenceLevel == ConfidenceVeryStrong {
			homologyClass := HomologyClass{
				Dimension: 1,
				Birth: 0.5, // Simplified birth time
				Death: hyst.HysteresisStrength,
				Persistence: hyst.HysteresisStrength,
			}
			homology.HomologyClasses = append(homology.HomologyClasses, homologyClass)
			homology.BettiNumbers[1]++
		}
	}

	// Dimension 2: Voids from structural breaks
	for _, sb := range evidence.StructuralBreaks {
		if sb.ConfidenceLevel == ConfidenceStrong || sb.ConfidenceLevel == ConfidenceVeryStrong {
			homologyClass := HomologyClass{
				Dimension: 2,
				Birth: sb.BreakPoint,
				Death: sb.BreakPoint + 0.5, // Shorter persistence for voids
				Persistence: 0.5,
			}
			homology.HomologyClasses = append(homology.HomologyClasses, homologyClass)
			homology.BettiNumbers[2]++
		}
	}

	return homology
}

// generateSummary creates human-readable topological insights
func (ma *ManifoldAnalyzer) generateSummary(data *TopologicalData) *TopologicalSummary {
	summary := &TopologicalSummary{
		TearCount: len(data.ManifoldTears.Tears),
		HoleCount: data.HomologyAnalysis.BettiNumbers[2],
		TopologicalInsights: make([]string, 0),
	}

	// Assess collapse risk
	if summary.TearCount > 2 {
		summary.CollapseRisk = "High"
	} else if summary.TearCount > 0 {
		summary.CollapseRisk = "Medium"
	} else {
		summary.CollapseRisk = "Low"
	}

	// Compute stability score
	summary.ManifoldStability = 1.0 - data.ManifoldTears.DiscontinuityScore
	if summary.ManifoldStability < 0 {
		summary.ManifoldStability = 0
	}

	// Generate insights
	if summary.TearCount > 0 {
		summary.TopologicalInsights = append(summary.TopologicalInsights,
			fmt.Sprintf("Detected %d manifold tears indicating phase transitions", summary.TearCount))
	}

	if summary.CollapseRisk == "High" {
		summary.TopologicalInsights = append(summary.TopologicalInsights,
			"High dimensionality collapse risk - variables may be dangerously aligned")
	}

	if summary.HoleCount > 0 {
		summary.TopologicalInsights = append(summary.TopologicalInsights,
			fmt.Sprintf("Found %d topological holes representing missing relationships", summary.HoleCount))
	}

	if summary.ManifoldStability > 0.8 {
		summary.TopologicalInsights = append(summary.TopologicalInsights,
			"High manifold stability suggests robust, predictable relationships")
	}

	return summary
}
