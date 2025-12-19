package profiling

import (
	"gohypo/domain/stats/brief"
	analysisbrief "gohypo/internal/analysis/brief"
)

// DataProfiler orchestrates comprehensive statistical profiling of datasets
// DEPRECATED: Use internal/analysis/brief.StatisticalBriefComputer directly
type DataProfiler struct {
	computer *analysisbrief.StatisticalBriefComputer
}

// NewDataProfiler creates a new data profiler
func NewDataProfiler() *DataProfiler {
	return &DataProfiler{
		computer: analysisbrief.NewComputer(),
	}
}

// ProfileColumn performs comprehensive statistical analysis on a single column
func (dp *DataProfiler) ProfileColumn(data []float64, name string) TopologyMarkers {
	// Use unified computation with validation context
	request := brief.ComputationRequest{
		ForValidation: true,
		ForHypothesis: true,
	}

	statBrief, err := dp.computer.ComputeBrief(data, name, "", request)
	if err != nil {
		// Return minimal brief on error
		return *brief.NewBrief(name, "", len(data), request)
	}

	return *statBrief
}

// ProfileDataset analyzes all columns in a dataset
func (dp *DataProfiler) ProfileDataset(dataset map[string][]float64) map[string]TopologyMarkers {
	results := make(map[string]TopologyMarkers)

	for columnName, data := range dataset {
		results[columnName] = dp.ProfileColumn(data, columnName)
	}

	return results
}


