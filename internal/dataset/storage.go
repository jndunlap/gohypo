package dataset

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// LocalFileStorage implements FileStorage using local filesystem
type LocalFileStorage struct {
	config *StorageConfig
}

// NewLocalFileStorage creates a new local file storage instance
func NewLocalFileStorage(config *StorageConfig) *LocalFileStorage {
	if config == nil {
		config = DefaultStorageConfig()
	}
	return &LocalFileStorage{config: config}
}

// NewLocalFileStorageWithPath creates a new local file storage with a simple path
func NewLocalFileStorageWithPath(basePath string) *LocalFileStorage {
	config := DefaultStorageConfig()
	config.BasePath = basePath
	return NewLocalFileStorage(config)
}

// Store saves a file to the local filesystem with a unique name
func (s *LocalFileStorage) Store(ctx context.Context, file multipart.File, filename string) (string, error) {
	// Create uploads directory if it doesn't exist
	if err := os.MkdirAll(s.config.BasePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Generate unique filename to prevent conflicts
	ext := filepath.Ext(filename)
	baseName := filename[:len(filename)-len(ext)]
	timestamp := time.Now().Format("20060102_150405")
	uniqueName := fmt.Sprintf("%s_%s_%s%s", baseName, timestamp, uuid.New().String()[:8], ext)

	filePath := filepath.Join(s.config.BasePath, uniqueName)

	// Create the destination file
	destFile, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy file contents with chunking for large files
	buf := make([]byte, s.config.ChunkSize)
	_, err = io.CopyBuffer(destFile, file, buf)
	if err != nil {
		os.Remove(filePath) // Clean up on failure
		return "", fmt.Errorf("failed to copy file contents: %w", err)
	}

	return filePath, nil
}

// GetReader returns a reader for the stored file
func (s *LocalFileStorage) GetReader(ctx context.Context, filePath string) (io.ReadCloser, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

// Delete removes a file from storage
func (s *LocalFileStorage) Delete(ctx context.Context, filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// Exists checks if a file exists in storage
func (s *LocalFileStorage) Exists(ctx context.Context, filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check file existence: %w", err)
	}
	return true, nil
}

// GetFileSize returns the size of a stored file
func (s *LocalFileStorage) GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}
	return info.Size(), nil
}

// GetSignedURL returns a signed URL for temporary access (not applicable for local storage)
func (s *LocalFileStorage) GetSignedURL(ctx context.Context, filePath string, expiry time.Duration) (string, error) {
	// For local storage, we can't generate signed URLs
	// This would be implemented differently for cloud storage
	return "", fmt.Errorf("signed URLs not supported for local storage")
}

// GetPublicURL returns a public URL for the file (not applicable for local storage)
func (s *LocalFileStorage) GetPublicURL(filePath string) string {
	// For local storage, return the file path
	// In a web application, this might be converted to a public URL
	return filePath
}
