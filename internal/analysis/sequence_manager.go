package analysis

import (
	"sync/atomic"
)

// SequenceManager provides thread-safe, globally ordered sequence IDs
// for the Scientific Ledger system. Ensures deterministic event ordering
// across concurrent statistical analysis operations.
type SequenceManager struct {
	currentSID int64
}

// NewSequenceManager creates a new sequence manager starting from 1
func NewSequenceManager() *SequenceManager {
	return &SequenceManager{currentSID: 0}
}

// Next returns a new, unique Sequence ID atomically.
// Thread-safe for concurrent use across multiple goroutines.
func (s *SequenceManager) Next() int64 {
	return atomic.AddInt64(&s.currentSID, 1)
}

// GetCurrent returns the last issued SID without incrementing.
// Useful for monitoring and debugging.
func (s *SequenceManager) GetCurrent() int64 {
	return atomic.LoadInt64(&s.currentSID)
}
