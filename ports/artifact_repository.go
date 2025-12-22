package ports

import (
	"context"
	"time"

	"gohypo/domain/core"
)

// ArtifactRepository defines the interface for hot-tier artifact storage
type ArtifactRepository interface {
	// Standard artifact operations
	SaveArtifact(ctx context.Context, artifact core.Artifact) error
	GetArtifact(ctx context.Context, artifactID core.ID) (*core.Artifact, error)
	ListArtifactsBySession(ctx context.Context, sessionID string) ([]core.Artifact, error)
	DeleteArtifact(ctx context.Context, artifactID core.ID) error

	// Metadata-specific operations for tiered storage
	SaveArtifactMetadata(ctx context.Context, metadata *ArtifactMetadata) error
	GetArtifactMetadata(ctx context.Context, artifactID core.ID) (*ArtifactMetadata, error)
	ListArtifactsOlderThan(ctx context.Context, olderThan time.Time) ([]*ArtifactMetadata, error)
	UpdateArtifactMetadata(ctx context.Context, metadata *ArtifactMetadata) error
}

// BlobStorage defines the interface for cold-tier blob storage
type BlobStorage interface {
	StoreBlob(ctx context.Context, key string, data interface{}) error
	GetBlob(ctx context.Context, key string) (*ArtifactBlob, error)
	DeleteBlob(ctx context.Context, key string) error
	ListBlobsByPrefix(ctx context.Context, prefix string) ([]string, error)
}

// ArtifactMetadata represents the hot-tier artifact data
type ArtifactMetadata struct {
	ID           core.ID           `json:"id"`
	Kind         core.ArtifactKind `json:"kind"`
	Fingerprint  core.Hash         `json:"fingerprint"`
	CreatedAt    core.Timestamp    `json:"created_at"`
	SessionID    string            `json:"session_id"`
	SizeBytes    int64             `json:"size_bytes"`
	BlobKey      string            `json:"blob_key,omitempty"` // For cold tier reference

	// Core validation results (always kept hot)
	ValidationStatus string    `json:"validation_status"`
	ConfidenceScore  float64   `json:"confidence_score"`
	EValue          float64    `json:"e_value"`

	// Minimal variable info
	CauseKey       string  `json:"cause_key"`
	EffectKey      string  `json:"effect_key"`
	EffectSize     float64 `json:"effect_size"`
	PValue         float64 `json:"p_value"`
}

// ArtifactBlob represents large artifact data stored in cold tier
type ArtifactBlob struct {
	ID        core.ID        `json:"id"`
	Kind      core.ArtifactKind `json:"kind"`

	// Full statistical details
	RawPayload      interface{} `json:"raw_payload"`
	SenseResults    interface{} `json:"sense_results,omitempty"`
	EvidenceBlocks  interface{} `json:"evidence_blocks,omitempty"`
	DiscoveryBriefs interface{} `json:"discovery_briefs,omitempty"`

	// Metadata
	CreatedAt time.Time `json:"created_at"`
}
