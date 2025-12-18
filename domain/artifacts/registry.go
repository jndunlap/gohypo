package artifacts

import (
	"fmt"

	"gohypo/domain/core"
	"gohypo/domain/stats"
)

// Note: ArtifactKind is defined in domain/core

// ArtifactSchema defines the structure of an artifact
type ArtifactSchema struct {
	Kind          core.ArtifactKind
	SchemaVersion string
	KeyFunc       func(core.Artifact) string // Stable identifier function
	ValidateFunc  func(core.Artifact) error  // Validation function
}

// Registry maps artifact kinds to their schemas
var Registry = map[core.ArtifactKind]ArtifactSchema{
	core.ArtifactRelationship: {
		Kind:          core.ArtifactRelationship,
		SchemaVersion: "1.0.0",
		KeyFunc:       relationshipKey,
		ValidateFunc:  validateRelationship,
	},
	core.ArtifactVariableProfile: {
		Kind:          core.ArtifactVariableProfile,
		SchemaVersion: "1.0.0",
		KeyFunc:       variableProfileKey,
		ValidateFunc:  validateVariableProfile,
	},
	core.ArtifactSkippedRelationship: {
		Kind:          core.ArtifactSkippedRelationship,
		SchemaVersion: "1.0.0",
		KeyFunc:       skippedRelationshipKey,
		ValidateFunc:  validateSkippedRelationship,
	},
	core.ArtifactSweepManifest: {
		Kind:          core.ArtifactSweepManifest,
		SchemaVersion: "1.0.0",
		KeyFunc:       sweepManifestKey,
		ValidateFunc:  validateSweepManifest,
	},
	core.ArtifactFDRFamily: {
		Kind:          core.ArtifactFDRFamily,
		SchemaVersion: "1.0.0",
		KeyFunc:       fdrFamilyKey,
		ValidateFunc:  validateFDRFamily,
	},
	core.ArtifactVariableHealth: {
		Kind:          core.ArtifactVariableHealth,
		SchemaVersion: "1.0.0",
		KeyFunc:       variableHealthKey,
		ValidateFunc:  validateVariableHealth,
	},
	core.ArtifactHypothesis: {
		Kind:          core.ArtifactHypothesis,
		SchemaVersion: "1.0.0",
		KeyFunc:       hypothesisKey,
		ValidateFunc:  validateHypothesis,
	},
	core.ArtifactRun: {
		Kind:          core.ArtifactRun,
		SchemaVersion: "1.0.0",
		KeyFunc:       runManifestKey,
		ValidateFunc:  validateRunManifest,
	},
}

// GetSchema returns the schema for an artifact kind
func GetSchema(kind core.ArtifactKind) (ArtifactSchema, error) {
	schema, exists := Registry[kind]
	if !exists {
		return ArtifactSchema{}, fmt.Errorf("unknown artifact kind: %s", kind)
	}
	return schema, nil
}

// ValidateArtifact validates an artifact against its schema
func ValidateArtifact(artifact core.Artifact) error {
	schema, err := GetSchema(artifact.Kind)
	if err != nil {
		return err
	}
	return schema.ValidateFunc(artifact)
}

// GetArtifactKey returns the stable key for an artifact
func GetArtifactKey(artifact core.Artifact) (string, error) {
	schema, err := GetSchema(artifact.Kind)
	if err != nil {
		return "", err
	}
	return schema.KeyFunc(artifact), nil
}

// Key functions for each artifact type
func relationshipKey(artifact core.Artifact) string {
	// For relationships, key is canonical and collision-resistant
	if payload, ok := artifact.Payload.(stats.RelationshipPayload); ok {
		// Use canonical ordering: min(varX,varY) first, then max(varX,varY)
		varX, varY := string(payload.VariableX), string(payload.VariableY)
		if varX > varY {
			varX, varY = varY, varX
		}
		// Format: relationship:{testType}:{familyID}:{varX}:{varY}
		key := fmt.Sprintf("relationship:%s:%s:%s:%s",
			payload.TestType, payload.FamilyID, varX, varY)
		return key
	}

	// Fallback for map payloads (legacy)
	if payload, ok := artifact.Payload.(map[string]interface{}); ok {
		if keyData, exists := payload["key"]; exists {
			if keyMap, ok := keyData.(map[string]interface{}); ok {
				varX := keyMap["variable_x"]
				varY := keyMap["variable_y"]
				testType := keyMap["test_type"]
				return fmt.Sprintf("%s-%s-%s", varX, varY, testType)
			}
		}
	}
	return string(artifact.ID) // fallback to ID
}

func variableHealthKey(artifact core.Artifact) string {
	return string(artifact.ID) // Variable health keyed by ID
}

func variableProfileKey(artifact core.Artifact) string {
	// Prefer stable key by variable_key if present.
	if payload, ok := artifact.Payload.(map[string]interface{}); ok {
		if varKey, ok := payload["variable_key"].(string); ok && varKey != "" {
			return fmt.Sprintf("variable_profile:%s", varKey)
		}
	}
	return string(artifact.ID)
}

func skippedRelationshipKey(artifact core.Artifact) string {
	// Prefer stable key from typed payload.
	if payload, ok := artifact.Payload.(stats.SkippedRelationshipArtifact); ok {
		varX, varY := string(payload.Key.VariableX), string(payload.Key.VariableY)
		if varX > varY {
			varX, varY = varY, varX
		}
		return fmt.Sprintf("skipped_relationship:%s:%s:%s", payload.Key.TestType, varX, varY)
	}
	// Fallback for map payloads.
	if payload, ok := artifact.Payload.(map[string]interface{}); ok {
		if keyData, exists := payload["key"]; exists {
			if keyMap, ok := keyData.(map[string]interface{}); ok {
				varX := keyMap["variable_x"]
				varY := keyMap["variable_y"]
				testType := keyMap["test_type"]
				return fmt.Sprintf("skipped_relationship:%v:%v:%v", testType, varX, varY)
			}
		}
	}
	return string(artifact.ID)
}

func sweepManifestKey(artifact core.Artifact) string {
	// Prefer sweep_id if present, otherwise fall back to ID.
	if payload, ok := artifact.Payload.(map[string]interface{}); ok {
		if sweepID, ok := payload["sweep_id"].(string); ok && sweepID != "" {
			return fmt.Sprintf("sweep_manifest:%s", sweepID)
		}
	}
	return string(artifact.ID)
}

func fdrFamilyKey(artifact core.Artifact) string {
	if payload, ok := artifact.Payload.(stats.FDRFamilyArtifact); ok {
		return fmt.Sprintf("fdr_family:%s", payload.FamilyID)
	}
	if payload, ok := artifact.Payload.(map[string]interface{}); ok {
		if familyID, ok := payload["family_id"].(string); ok && familyID != "" {
			return fmt.Sprintf("fdr_family:%s", familyID)
		}
	}
	return string(artifact.ID)
}

func hypothesisKey(artifact core.Artifact) string {
	return string(artifact.ID) // Hypotheses keyed by ID
}

func runManifestKey(artifact core.Artifact) string {
	// Run manifests are keyed by runID for uniqueness
	if payload, ok := artifact.Payload.(map[string]interface{}); ok {
		if runID, exists := payload["run_id"]; exists {
			if runIDStr, ok := runID.(string); ok {
				return fmt.Sprintf("run_manifest:%s", runIDStr)
			}
		}
	}
	return string(artifact.ID) // fallback to ID
}

// Validation functions for each artifact type
func validateRelationship(artifact core.Artifact) error {
	// Basic validation - could be enhanced
	if artifact.Kind != core.ArtifactRelationship {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactRelationship, artifact.Kind)
	}
	if artifact.ID.IsEmpty() {
		return fmt.Errorf("relationship artifact missing ID")
	}
	return nil
}

func validateVariableHealth(artifact core.Artifact) error {
	if artifact.Kind != core.ArtifactVariableHealth {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactVariableHealth, artifact.Kind)
	}
	return nil
}

func validateVariableProfile(artifact core.Artifact) error {
	if artifact.Kind != core.ArtifactVariableProfile {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactVariableProfile, artifact.Kind)
	}
	if artifact.ID.IsEmpty() {
		return fmt.Errorf("variable profile artifact missing ID")
	}
	return nil
}

func validateSkippedRelationship(artifact core.Artifact) error {
	if artifact.Kind != core.ArtifactSkippedRelationship {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactSkippedRelationship, artifact.Kind)
	}
	if artifact.ID.IsEmpty() {
		return fmt.Errorf("skipped relationship artifact missing ID")
	}
	return nil
}

func validateSweepManifest(artifact core.Artifact) error {
	if artifact.Kind != core.ArtifactSweepManifest {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactSweepManifest, artifact.Kind)
	}
	if artifact.ID.IsEmpty() {
		return fmt.Errorf("sweep manifest artifact missing ID")
	}
	return nil
}

func validateFDRFamily(artifact core.Artifact) error {
	if artifact.Kind != core.ArtifactFDRFamily {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactFDRFamily, artifact.Kind)
	}
	if artifact.ID.IsEmpty() {
		return fmt.Errorf("fdr family artifact missing ID")
	}
	return nil
}

func validateHypothesis(artifact core.Artifact) error {
	if artifact.Kind != core.ArtifactHypothesis {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactHypothesis, artifact.Kind)
	}
	return nil
}

func validateRunManifest(artifact core.Artifact) error {
	if artifact.Kind != core.ArtifactRun {
		return fmt.Errorf("expected kind %s, got %s", core.ArtifactRun, artifact.Kind)
	}
	if artifact.ID.IsEmpty() {
		return fmt.Errorf("run manifest artifact missing ID")
	}
	// Additional validation could check required fields are present
	return nil
}
