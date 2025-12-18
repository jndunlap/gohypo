package core

import (
	"testing"
)

// TestNewIDUniqueness tests that NewID generates unique identifiers
func TestNewIDUniqueness(t *testing.T) {
	const numIDs = 10000

	// Generate many IDs
	ids := make(map[ID]bool, numIDs)
	for i := 0; i < numIDs; i++ {
		id := NewID()
		if id.IsEmpty() {
			t.Errorf("Generated empty ID at iteration %d", i)
		}
		if ids[id] {
			t.Errorf("Generated duplicate ID: %s", id)
		}
		ids[id] = true
	}

	if len(ids) != numIDs {
		t.Errorf("Expected %d unique IDs, got %d", numIDs, len(ids))
	}
}

// TestIDString tests ID string conversion
func TestIDString(t *testing.T) {
	id := ID("test-123")
	if id.String() != "test-123" {
		t.Errorf("Expected String() to return 'test-123', got '%s'", id.String())
	}
}

// TestIDIsEmpty tests ID emptiness check
func TestIDIsEmpty(t *testing.T) {
	emptyID := ID("")
	if !emptyID.IsEmpty() {
		t.Error("Expected empty ID to be empty")
	}

	nonEmptyID := ID("not-empty")
	if nonEmptyID.IsEmpty() {
		t.Error("Expected non-empty ID to not be empty")
	}
}

// TestParseHypothesisID tests hypothesis ID parsing
func TestParseHypothesisID(t *testing.T) {
	tests := []struct {
		input    string
		expected HypothesisID
		hasError bool
	}{
		{"valid-id", HypothesisID("valid-id"), false},
		{"", "", true},
		{"   ", "", true},
	}

	for _, test := range tests {
		result, err := ParseHypothesisID(test.input)
		if test.hasError && err == nil {
			t.Errorf("Expected error for input '%s', but got none", test.input)
		}
		if !test.hasError && err != nil {
			t.Errorf("Unexpected error for input '%s': %v", test.input, err)
		}
		if result != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, result)
		}
	}
}

// TestParseRunID tests run ID parsing
func TestParseRunID(t *testing.T) {
	tests := []struct {
		input    string
		expected RunID
		hasError bool
	}{
		{"run-123", RunID("run-123"), false},
		{"", "", true},
	}

	for _, test := range tests {
		result, err := ParseRunID(test.input)
		if test.hasError && err == nil {
			t.Errorf("Expected error for input '%s', but got none", test.input)
		}
		if !test.hasError && err != nil {
			t.Errorf("Unexpected error for input '%s': %v", test.input, err)
		}
		if result != test.expected {
			t.Errorf("Expected %s, got %s", test.expected, result)
		}
	}
}
