package app

import (
	"gohypo/ports"
)

// StageRunner handles execution of statistical analysis stages
type StageRunner struct {
	ledgerPort ports.LedgerPort
	rngPort    ports.RNGPort
}

// NewStageRunner creates a new stage runner
func NewStageRunner(ledgerPort ports.LedgerPort, rngPort ports.RNGPort) *StageRunner {
	return &StageRunner{
		ledgerPort: ledgerPort,
		rngPort:    rngPort,
	}
}
