package core

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ID represents a domain identifier
type ID string

// NewID creates a new unique identifier using UUID v7 for time-ordered generation
func NewID() ID {
	// Use UUID v7 for time-ordered, sortable IDs
	// Falls back to v4 if v7 is not available (for compatibility)
	id, err := uuid.NewV7()
	if err != nil {
		// Fallback to v4 if v7 fails
		id = uuid.New()
	}
	return ID(id.String())
}

// String returns the string representation
func (id ID) String() string {
	return string(id)
}

// IsEmpty checks if the ID is empty
func (id ID) IsEmpty() bool {
	return id == ""
}

// Domain-specific ID types
type (
	HypothesisID ID
	RunID        ID
	SnapshotID   ID
	VariableKey  ID
	ArtifactID   ID
)

// String conversions for domain IDs
func (id HypothesisID) String() string { return ID(id).String() }
func (id RunID) String() string        { return ID(id).String() }
func (id SnapshotID) String() string   { return ID(id).String() }
func (id VariableKey) String() string  { return ID(id).String() }
func (id ArtifactID) String() string   { return ID(id).String() }

// ParseHypothesisID parses a string into HypothesisID
func ParseHypothesisID(s string) (HypothesisID, error) {
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("hypothesis ID cannot be empty")
	}
	return HypothesisID(s), nil
}

// ParseRunID parses a string into RunID
func ParseRunID(s string) (RunID, error) {
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("run ID cannot be empty")
	}
	return RunID(s), nil
}

// ParseSnapshotID parses a string into SnapshotID
func ParseSnapshotID(s string) (SnapshotID, error) {
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("snapshot ID cannot be empty")
	}
	return SnapshotID(s), nil
}

// ParseVariableKey parses a string into VariableKey
func ParseVariableKey(s string) (VariableKey, error) {
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("variable key cannot be empty")
	}
	return VariableKey(s), nil
}

// ParseArtifactID parses a string into ArtifactID
func ParseArtifactID(s string) (ArtifactID, error) {
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("artifact ID cannot be empty")
	}
	return ArtifactID(s), nil
}

// Artifact represents any output of the system
type Artifact struct {
	ID        ID           `json:"id"`
	Kind      ArtifactKind `json:"kind"`
	Payload   interface{}  `json:"payload"`
	CreatedAt Timestamp    `json:"created_at"`
}

// ArtifactKind defines types of artifacts
type ArtifactKind string

const (
	ArtifactRelationship ArtifactKind = "relationship"
	// ArtifactVariableProfile is the output of the Profile stage (per-variable stats).
	ArtifactVariableProfile ArtifactKind = "variable_profile"
	// ArtifactSkippedRelationship records why a variable pair was not tested.
	ArtifactSkippedRelationship ArtifactKind = "skipped_relationship"
	// ArtifactSweepManifest captures audit metadata for a sweep (counts, thresholds, fingerprint, etc.).
	ArtifactSweepManifest ArtifactKind = "sweep_manifest"
	// ArtifactFDRFamily captures FDR family definitions produced by stats stages.
	ArtifactFDRFamily      ArtifactKind = "fdr_family"
	ArtifactVariableHealth ArtifactKind = "variable_health"
	ArtifactHypothesis     ArtifactKind = "hypothesis"
	ArtifactRun            ArtifactKind = "run"
	// NEW: Greenfield Research Flow artifacts
	ArtifactResearchDirective  ArtifactKind = "research_directive"
	ArtifactEngineeringBacklog ArtifactKind = "engineering_backlog"
)
