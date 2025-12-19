package profiling

import (
	"gohypo/domain/stats/brief"
)

// TopologyMarkers is now an alias for StatisticalBrief for backward compatibility
// New code should use domain/stats/brief.StatisticalBrief directly
type TopologyMarkers = brief.StatisticalBrief

// NewTopologyMarkers creates topology markers with validation context
func NewTopologyMarkers(fieldKey string, sampleSize int) *TopologyMarkers {
	return brief.NewBrief(fieldKey, "", sampleSize, brief.ComputationRequest{ForValidation: true})
}


