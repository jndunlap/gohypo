package session

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gohypo/ports"
)

// StorageProvider represents different storage backends
type StorageProvider string

const (
	StorageLocal StorageProvider = "local"
	StorageS3    StorageProvider = "s3"
)

// BlobStore defines the interface for blob storage operations
// This abstraction allows seamless switching between local disk and S3
type BlobStore interface {
	// Core blob operations
	StoreBlob(ctx context.Context, key string, data interface{}) error
	GetBlob(ctx context.Context, key string) (io.ReadCloser, error)
	DeleteBlob(ctx context.Context, key string) error
	BlobExists(ctx context.Context, key string) (bool, error)

	// Batch operations
	ListBlobs(ctx context.Context, prefix string) ([]string, error)
	DeleteBlobs(ctx context.Context, keys []string) error

	// Metadata operations
	GetBlobMetadata(ctx context.Context, key string) (*BlobMetadata, error)

	// Lifecycle management
	CleanupExpired(ctx context.Context, olderThan time.Duration) error

	// Provider information
	Provider() StorageProvider
}

// BlobMetadata represents metadata for stored blobs
type BlobMetadata struct {
	Key          string          `json:"key"`
	Size         int64           `json:"size"`
	ContentType  string          `json:"content_type"`
	ETag         string          `json:"etag"`
	LastModified time.Time       `json:"last_modified"`
	Provider     StorageProvider `json:"provider"`
}

// LocalBlobStore implements BlobStore using local filesystem
// This provides S3-compatible interface for development
type LocalBlobStore struct {
	basePath string
}

// NewLocalBlobStore creates a new local blob store
func NewLocalBlobStore(basePath string) (*LocalBlobStore, error) {
	// Ensure base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalBlobStore{
		basePath: basePath,
	}, nil
}

// Provider returns the storage provider type
func (lbs *LocalBlobStore) Provider() StorageProvider {
	return StorageLocal
}

// StoreBlob stores data to local filesystem with S3-like key structure
func (lbs *LocalBlobStore) StoreBlob(ctx context.Context, key string, data interface{}) error {
	// Convert key to filesystem path
	filePath := lbs.keyToPath(key)

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Serialize data to JSON (mimicking S3 object storage)
	var content []byte
	switch v := data.(type) {
	case []byte:
		content = v
	case string:
		content = []byte(v)
	case io.Reader:
		var err error
		content, err = io.ReadAll(v)
		if err != nil {
			return fmt.Errorf("failed to read data: %w", err)
		}
	default:
		// Assume it's a struct that can be JSON marshaled
		if jsonData, ok := data.(ports.ArtifactBlob); ok {
			content = []byte(fmt.Sprintf("%+v", jsonData)) // Simplified for demo
		} else {
			content = []byte(fmt.Sprintf("%+v", data))
		}
	}

	// Write to file
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// GetBlob retrieves data from local filesystem
func (lbs *LocalBlobStore) GetBlob(ctx context.Context, key string) (io.ReadCloser, error) {
	filePath := lbs.keyToPath(key)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blob not found: %s", key)
		}
		return nil, fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	return file, nil
}

// DeleteBlob removes a blob from local storage
func (lbs *LocalBlobStore) DeleteBlob(ctx context.Context, key string) error {
	filePath := lbs.keyToPath(key)

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	return nil
}

// BlobExists checks if a blob exists
func (lbs *LocalBlobStore) BlobExists(ctx context.Context, key string) (bool, error) {
	filePath := lbs.keyToPath(key)

	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check file existence: %w", err)
}

// ListBlobs lists blobs with a given prefix
func (lbs *LocalBlobStore) ListBlobs(ctx context.Context, prefix string) ([]string, error) {
	_ = lbs.keyToPath(prefix) // Convert prefix to path (for future prefix filtering)

	var keys []string
	err := filepath.Walk(lbs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Convert path back to key
		relPath, err := filepath.Rel(lbs.basePath, path)
		if err != nil {
			return err
		}

		key := strings.ReplaceAll(relPath, string(filepath.Separator), "/")

		// Check if it matches prefix
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list blobs: %w", err)
	}

	return keys, nil
}

// DeleteBlobs removes multiple blobs
func (lbs *LocalBlobStore) DeleteBlobs(ctx context.Context, keys []string) error {
	for _, key := range keys {
		if err := lbs.DeleteBlob(ctx, key); err != nil {
			return fmt.Errorf("failed to delete blob %s: %w", key, err)
		}
	}
	return nil
}

// GetBlobMetadata returns metadata for a blob
func (lbs *LocalBlobStore) GetBlobMetadata(ctx context.Context, key string) (*BlobMetadata, error) {
	filePath := lbs.keyToPath(key)

	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blob not found: %s", key)
		}
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &BlobMetadata{
		Key:          key,
		Size:         stat.Size(),
		ContentType:  "application/json", // Assume JSON for now
		LastModified: stat.ModTime(),
		Provider:     StorageLocal,
	}, nil
}

// CleanupExpired removes expired blobs (placeholder implementation)
func (lbs *LocalBlobStore) CleanupExpired(ctx context.Context, olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	return filepath.Walk(lbs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove expired file %s: %w", path, err)
			}
		}

		return nil
	})
}

// keyToPath converts an S3-style key to a filesystem path
func (lbs *LocalBlobStore) keyToPath(key string) string {
	// Replace S3 key structure with filesystem path
	// e.g., "artifacts/session123/hypothesis456.json" -> "/data/artifacts/session123/hypothesis456.json"
	return filepath.Join(lbs.basePath, filepath.FromSlash(key))
}

// S3BlobStore implements BlobStore using AWS S3 (placeholder for future implementation)
type S3BlobStore struct {
	bucketName string
	region     string
}

// NewS3BlobStore creates a new S3 blob store (placeholder)
func NewS3BlobStore(bucketName, region string) (*S3BlobStore, error) {
	return &S3BlobStore{
		bucketName: bucketName,
		region:     region,
	}, nil
}

// Provider returns S3 provider type
func (s3 *S3BlobStore) Provider() StorageProvider {
	return StorageS3
}

// TODO: Implement S3 methods when ready for production
func (s3 *S3BlobStore) StoreBlob(ctx context.Context, key string, data interface{}) error {
	return fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}

// GetBlob retrieves from S3
func (s3 *S3BlobStore) GetBlob(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}

// DeleteBlob removes from S3
func (s3 *S3BlobStore) DeleteBlob(ctx context.Context, key string) error {
	return fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}

// BlobExists checks S3
func (s3 *S3BlobStore) BlobExists(ctx context.Context, key string) (bool, error) {
	return false, fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}

// ListBlobs lists S3 objects
func (s3 *S3BlobStore) ListBlobs(ctx context.Context, prefix string) ([]string, error) {
	return nil, fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}

// DeleteBlobs removes multiple from S3
func (s3 *S3BlobStore) DeleteBlobs(ctx context.Context, keys []string) error {
	return fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}

// GetBlobMetadata gets S3 metadata
func (s3 *S3BlobStore) GetBlobMetadata(ctx context.Context, key string) (*BlobMetadata, error) {
	return nil, fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}

// CleanupExpired cleans up S3
func (s3 *S3BlobStore) CleanupExpired(ctx context.Context, olderThan time.Duration) error {
	return fmt.Errorf("S3 implementation not yet available - use LocalBlobStore for development")
}
