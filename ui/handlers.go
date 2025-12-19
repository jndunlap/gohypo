package ui

import (
	domainBrief "gohypo/domain/stats/brief"
)

// FieldStats represents statistics for a single field/variable
type FieldStats struct {
	Name              string
	MissingRate       float64
	MissingRatePct    string
	UniqueCount       int
	Variance          float64
	Cardinality       int
	Type              string
	SampleSize        int
	InRelationships   int
	StrongestCorr     float64
	AvgEffectSize     float64
	SignificantRels   int
	TotalRelsAnalyzed int

	// Enhanced statistical information from StatisticalBrief
	Mean             float64
	StdDev           float64
	Min              float64
	Max              float64
	Median           float64
	CV               float64 // Coefficient of variation (StdDev / Mean)
	Skewness         float64
	Kurtosis         float64
	IsNormal         bool
	SparsityRatio    float64
	NoiseCoefficient float64
	OutlierCount     int
	Entropy          float64
	Mode             string
	ModeFrequency    int

	// Store the original StatisticalBrief for full access
	StatisticalBrief *domainBrief.StatisticalBrief
}
