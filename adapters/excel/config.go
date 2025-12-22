package excel

import (
	"gohypo/adapters/datareadiness/coercer"
	"gohypo/adapters/datareadiness/synthesizer"
	"gohypo/domain/datareadiness/profiling"
)

// ExcelConfig holds configuration for Excel data source
type ExcelConfig struct {
	FilePath        string                      `json:"file_path"`
	CoercionConfig  coercer.CoercionConfig      `json:"coercion_config"`
	ProfilingConfig profiling.ProfilingConfig   `json:"profiling_config"`
	SynthesisConfig synthesizer.SynthesisConfig `json:"synthesis_config"`
	Enabled         bool                        `json:"enabled"`
}

// DefaultExcelConfig returns sensible defaults for Excel processing
func DefaultExcelConfig() ExcelConfig {
	return ExcelConfig{
		CoercionConfig:  coercer.DefaultCoercionConfig(),
		ProfilingConfig: profiling.DefaultProfilingConfig(),
		SynthesisConfig: synthesizer.DefaultSynthesisConfig(),
		Enabled:         false,
	}
}



