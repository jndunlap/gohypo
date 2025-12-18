package core

import (
	"time"
)

// Timestamp represents a point in time with timezone awareness
type Timestamp time.Time

// NewTimestamp creates a new timestamp from time.Time
func NewTimestamp(t time.Time) Timestamp {
	return Timestamp(t)
}

// Now returns the current timestamp
func Now() Timestamp {
	return Timestamp(time.Now())
}

// Time returns the underlying time.Time
func (t Timestamp) Time() time.Time {
	return time.Time(t)
}

// IsZero checks if the timestamp is zero
func (t Timestamp) IsZero() bool {
	return time.Time(t).IsZero()
}

// Before returns true if t is before u
func (t Timestamp) Before(u Timestamp) bool {
	return time.Time(t).Before(time.Time(u))
}

// After returns true if t is after u
func (t Timestamp) After(u Timestamp) bool {
	return time.Time(t).After(time.Time(u))
}

// Domain-specific time types
type (
	SnapshotAt Timestamp
	CutoffAt   Timestamp
	Lag        time.Duration
)

// Constructors for domain time types
func NewSnapshotAt(t time.Time) SnapshotAt { return SnapshotAt(NewTimestamp(t)) }
func NewCutoffAt(t time.Time) CutoffAt     { return CutoffAt(NewTimestamp(t)) }
func NewLag(d time.Duration) Lag           { return Lag(d) }

// Time conversions
func (t SnapshotAt) Time() time.Time  { return Timestamp(t).Time() }
func (t CutoffAt) Time() time.Time    { return Timestamp(t).Time() }
func (l Lag) Duration() time.Duration { return time.Duration(l) }

// ApplyLag applies lag to a timestamp (typically subtracting)
func (t SnapshotAt) ApplyLag(lag Lag) CutoffAt {
	return NewCutoffAt(t.Time().Add(-lag.Duration()))
}

// JSON marshaling for Timestamp
func (t Timestamp) MarshalJSON() ([]byte, error) {
	return time.Time(t).MarshalJSON()
}

func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var tm time.Time
	if err := tm.UnmarshalJSON(data); err != nil {
		return err
	}
	*t = Timestamp(tm)
	return nil
}

// String representations
func (t SnapshotAt) String() string { return t.Time().Format(time.RFC3339) }
func (t CutoffAt) String() string   { return t.Time().Format(time.RFC3339) }
func (l Lag) String() string        { return l.Duration().String() }
