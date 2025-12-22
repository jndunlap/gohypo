package session

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"

	"gohypo/domain/core"
	"gohypo/ports"
)

// TieredArtifactManager manages hot/cold artifact storage
type TieredArtifactManager struct {
	hotStorage  ports.ArtifactRepository // PostgreSQL for metadata
	coldStorage BlobStore                // Abstracted blob storage (local/S3)
	maxHotSize  int64                    // Maximum size for hot storage (bytes)
}

func NewTieredArtifactManager(
	hotStorage ports.ArtifactRepository,
	coldStorage BlobStore,
	maxHotSize int64,
) *TieredArtifactManager {
	if maxHotSize == 0 {
		maxHotSize = 1024 * 1024 // 1MB default
	}

	return &TieredArtifactManager{
		hotStorage:  hotStorage,
		coldStorage: coldStorage,
		maxHotSize:  maxHotSize,
	}
}

// StoreArtifact intelligently stores artifacts across tiers
func (tam *TieredArtifactManager) StoreArtifact(
	ctx context.Context,
	artifact core.Artifact,
	sessionID string,
) error {

	// Serialize the full artifact to check size
	fullData, err := json.Marshal(artifact)
	if err != nil {
		return fmt.Errorf("failed to serialize artifact: %w", err)
	}

	artifactSize := int64(len(fullData))

	if artifactSize <= tam.maxHotSize {
		// Store entirely in hot storage
		return tam.hotStorage.SaveArtifact(ctx, artifact)
	}

	// Split storage: metadata hot, payload cold
	metadata, blob := tam.splitArtifact(artifact, sessionID, artifactSize)

	// Store blob in cold storage
	blobKey := fmt.Sprintf("artifacts/%s/%s.json", sessionID, artifact.ID.String())
	if err := tam.coldStorage.StoreBlob(ctx, blobKey, blob); err != nil {
		return fmt.Errorf("failed to store blob: %w", err)
	}

	// Store metadata with blob reference in hot storage
	metadata.BlobKey = blobKey
	return tam.hotStorage.SaveArtifactMetadata(ctx, metadata)
}

// splitArtifact separates artifact into metadata and blob components
func (tam *TieredArtifactManager) splitArtifact(
	artifact core.Artifact,
	sessionID string,
	totalSize int64,
) (*ports.ArtifactMetadata, *ports.ArtifactBlob) {

	metadata := &ports.ArtifactMetadata{
		ID:          artifact.ID,
		Kind:        artifact.Kind,
		Fingerprint: core.Hash(fmt.Sprintf("artifact_%s", artifact.ID.String())),
		CreatedAt:   artifact.CreatedAt,
		SessionID:   sessionID,
		SizeBytes:   totalSize,
	}

	blob := &ports.ArtifactBlob{
		ID:         artifact.ID,
		Kind:       artifact.Kind,
		RawPayload: artifact.Payload,
		CreatedAt:  time.Now(),
	}

	// Extract core validation results for hot storage
	if payload, ok := artifact.Payload.(map[string]interface{}); ok {
		if status, ok := payload["validation_status"].(string); ok {
			metadata.ValidationStatus = status
		}
		if confidence, ok := payload["confidence"].(float64); ok {
			metadata.ConfidenceScore = confidence
		}
		if evalue, ok := payload["current_e_value"].(float64); ok {
			metadata.EValue = evalue
		}

		// Extract variable information
		if causeKey, ok := payload["cause_key"].(string); ok {
			metadata.CauseKey = causeKey
		}
		if effectKey, ok := payload["effect_key"].(string); ok {
			metadata.EffectKey = effectKey
		}
		if effectSize, ok := payload["effect_size"].(float64); ok {
			metadata.EffectSize = effectSize
		}
		if pValue, ok := payload["p_value"].(float64); ok {
			metadata.PValue = pValue
		}

		// Move large data to blob
		blob.SenseResults = payload["sense_results"]
		blob.EvidenceBlocks = payload["evidence"]
		blob.DiscoveryBriefs = payload["discovery_briefs"]

		// Remove large data from hot storage
		delete(payload, "sense_results")
		delete(payload, "evidence")
		delete(payload, "discovery_briefs")
	}

	return metadata, blob
}

// RetrieveArtifact reconstructs artifact from hot/cold storage
func (tam *TieredArtifactManager) RetrieveArtifact(
	ctx context.Context,
	artifactID core.ID,
) (*core.Artifact, error) {

	// Get metadata from hot storage
	metadata, err := tam.hotStorage.GetArtifactMetadata(ctx, artifactID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}

	artifact := &core.Artifact{
		ID:        metadata.ID,
		Kind:      metadata.Kind,
		CreatedAt: metadata.CreatedAt,
	}

	if metadata.BlobKey == "" {
		// Full artifact is in hot storage - get the complete artifact
		fullArtifact, err := tam.hotStorage.GetArtifact(ctx, artifactID)
		if err != nil {
			return nil, fmt.Errorf("failed to get full artifact: %w", err)
		}
		return fullArtifact, nil
	}

	// Reconstruct from hot + cold storage
	blobReader, err := tam.coldStorage.GetBlob(ctx, metadata.BlobKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get blob: %w", err)
	}
	defer blobReader.Close()

	// Read blob data
	blobData, err := io.ReadAll(blobReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read blob data: %w", err)
	}

	// Unmarshal into ArtifactBlob struct
	var blob ports.ArtifactBlob
	if err := json.Unmarshal(blobData, &blob); err != nil {
		return nil, fmt.Errorf("failed to unmarshal blob data: %w", err)
	}

	// Merge metadata with blob data
	artifact.Payload = tam.mergePayload(metadata, &blob)

	return artifact, nil
}

// mergePayload combines hot metadata with cold blob data
func (tam *TieredArtifactManager) mergePayload(
	metadata *ports.ArtifactMetadata,
	blob *ports.ArtifactBlob,
) map[string]interface{} {

	payload := make(map[string]interface{})

	// Add metadata fields
	payload["validation_status"] = metadata.ValidationStatus
	payload["confidence"] = metadata.ConfidenceScore
	payload["current_e_value"] = metadata.EValue
	payload["cause_key"] = metadata.CauseKey
	payload["effect_key"] = metadata.EffectKey
	payload["effect_size"] = metadata.EffectSize
	payload["p_value"] = metadata.PValue

	// Add blob data
	if blob.RawPayload != nil {
		payload["raw_payload"] = blob.RawPayload
	}
	if blob.SenseResults != nil {
		payload["sense_results"] = blob.SenseResults
	}
	if blob.EvidenceBlocks != nil {
		payload["evidence"] = blob.EvidenceBlocks
	}
	if blob.DiscoveryBriefs != nil {
		payload["discovery_briefs"] = blob.DiscoveryBriefs
	}

	return payload
}

// CleanupArtifacts removes old artifacts from both tiers
func (tam *TieredArtifactManager) CleanupArtifacts(
	ctx context.Context,
	olderThan time.Time,
) error {

	// Get artifacts to clean up from hot storage
	artifactsToClean, err := tam.hotStorage.ListArtifactsOlderThan(ctx, olderThan)
	if err != nil {
		return fmt.Errorf("failed to list old artifacts: %w", err)
	}

	for _, artifact := range artifactsToClean {
		// Delete from cold storage if blob exists
		if artifact.BlobKey != "" {
			if err := tam.coldStorage.DeleteBlob(ctx, artifact.BlobKey); err != nil {
				log.Printf("Warning: failed to delete blob %s: %v", artifact.BlobKey, err)
			}
		}

		// Delete from hot storage
		if err := tam.hotStorage.DeleteArtifact(ctx, artifact.ID); err != nil {
			log.Printf("Warning: failed to delete artifact metadata %s: %v", artifact.ID, err)
		}
	}

	return nil
}

// GetArtifactsBySession retrieves all artifacts for a session (with option to fetch cold data)
func (tam *TieredArtifactManager) GetArtifactsBySession(
	ctx context.Context,
	sessionID string,
	includeColdData bool,
) ([]*core.Artifact, error) {

	artifacts, err := tam.hotStorage.ListArtifactsBySession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	if !includeColdData {
		// Convert []core.Artifact to []*core.Artifact
		result := make([]*core.Artifact, len(artifacts))
		for i := range artifacts {
			result[i] = &artifacts[i]
		}
		return result, nil
	}

	// Reconstruct full artifacts including cold data
	fullArtifacts := make([]*core.Artifact, len(artifacts))
	for i, artifact := range artifacts {
		if artifact.ID.IsEmpty() {
			continue
		}

		fullArtifact, err := tam.RetrieveArtifact(ctx, artifact.ID)
		if err != nil {
			log.Printf("Warning: failed to retrieve full artifact %s: %v", artifact.ID, err)
			fullArtifacts[i] = &artifact // Use hot-only version as fallback
		} else {
			fullArtifacts[i] = fullArtifact
		}
	}

	return fullArtifacts, nil
}
