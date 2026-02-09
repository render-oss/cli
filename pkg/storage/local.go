package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

const (
	// DefaultLocalBasePath is the default directory for local object storage
	DefaultLocalBasePath = ".render/objects"
)

// LocalService implements StorageService for local filesystem storage
type LocalService struct {
	basePath   string
	region     string
	bucketName string
}

// NewLocalService creates a new LocalService with the specified base path, region, and bucket name
func NewLocalService(basePath, region, bucketName string) *LocalService {
	if basePath == "" {
		basePath = DefaultLocalBasePath
	}
	return &LocalService{
		basePath:   basePath,
		region:     region,
		bucketName: bucketName,
	}
}

// Upload copies a local file to the local object storage directory
func (s *LocalService) Upload(ctx context.Context, key, filePath string) (*UploadResult, error) {
	// Verify file exists
	if _, err := os.Stat(filePath); err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Get the full object file path (key includes filename)
	destPath := s.objectPath(key)

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	// Open source file
	src, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Create destination file
	dest, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dest.Close()

	// Copy the file
	written, err := io.Copy(dest, src)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	return &UploadResult{
		Key:       key,
		Region:    s.region,
		SizeBytes: written,
		LocalPath: destPath,
	}, nil
}

// Download reads an object from local storage and writes it to the provided writer
func (s *LocalService) Download(ctx context.Context, key string, dest io.Writer) (*DownloadResult, error) {
	objectPath := s.objectPath(key)

	if _, err := os.Stat(objectPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	// Open the object file
	src, err := os.Open(objectPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open object: %w", err)
	}
	defer src.Close()

	// Copy to destination
	written, err := io.Copy(dest, src)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return &DownloadResult{
		Key:       key,
		Region:    s.region,
		SizeBytes: written,
		LocalPath: objectPath,
	}, nil
}

// Delete removes an object from local storage
func (s *LocalService) Delete(ctx context.Context, key string) (*DeleteResult, error) {
	objectPath := s.objectPath(key)

	// Check if object exists
	if _, err := os.Stat(objectPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, fmt.Errorf("failed to stat object: %w", err)
	}

	// Remove the object file
	if err := os.Remove(objectPath); err != nil {
		return nil, fmt.Errorf("failed to delete object: %w", err)
	}

	// Clean up empty parent directories
	s.cleanupEmptyParents(objectPath)

	return &DeleteResult{
		Key:    key,
		Region: s.region,
	}, nil
}

// Exists checks if an object exists in local storage
func (s *LocalService) Exists(ctx context.Context, key string) (bool, error) {
	objectPath := s.objectPath(key)
	_, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// objectPath returns the full file path for a given object key
// Path structure: {basePath}/{region}/{bucketName}/{encodedKey}
// The key includes the filename (e.g., "uploads/images/photo.jpg")
// Keys are URL-encoded to handle special characters safely, which creates
// a flat structure on disk (slashes are encoded as %2F)
func (s *LocalService) objectPath(key string) string {
	// URL-encode the key to handle special characters safely
	// This creates a flat structure on disk (e.g., "uploads%2Fimages%2Fphoto.jpg")
	encodedKey := url.PathEscape(key)
	return filepath.Join(s.basePath, s.region, s.bucketName, encodedKey)
}

// cleanupEmptyParents removes empty parent directories up to the region directory
// Stops at {basePath}/{region} to preserve region structure
func (s *LocalService) cleanupEmptyParents(path string) {
	// Stop at the region level: {basePath}/{region}
	regionPath := filepath.Join(s.basePath, s.region)

	parent := filepath.Dir(path)
	for parent != regionPath && parent != s.basePath && parent != "." && parent != "/" {
		entries, err := os.ReadDir(parent)
		if err != nil || len(entries) > 0 {
			break
		}
		os.Remove(parent)
		parent = filepath.Dir(parent)
	}
}

// List lists objects in local storage with pagination support
func (s *LocalService) List(ctx context.Context, cursor string, limit int) (*ListResult, error) {
	bucketPath := filepath.Join(s.basePath, s.region, s.bucketName)

	if _, err := os.Stat(bucketPath); os.IsNotExist(err) {
		return &ListResult{Objects: []ObjectInfo{}}, nil
	}

	entries, err := os.ReadDir(bucketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var objects []ObjectInfo
	startIndex := 0

	if cursor != "" {
		encodedCursor := url.PathEscape(cursor)
		for i, entry := range entries {
			if entry.Name() == encodedCursor {
				startIndex = i + 1
				break
			}
		}
	}

	lastIndex := startIndex
	for i := startIndex; i < len(entries) && len(objects) < limit; i++ {
		entry := entries[i]
		// Skip directories - users may have manually created them in the local storage path
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		decodedKey, err := url.PathUnescape(entry.Name())
		if err != nil {
			decodedKey = entry.Name()
		}

		objects = append(objects, ObjectInfo{
			Key:          decodedKey,
			SizeBytes:    info.Size(),
			LastModified: info.ModTime(),
		})
		lastIndex = i + 1
	}

	// Determine next cursor - only set if there are more entries to process
	nextCursor := ""
	if len(objects) > 0 && lastIndex < len(entries) {
		nextCursor = objects[len(objects)-1].Key
	}

	return &ListResult{
		Objects: objects,
		Cursor:  nextCursor,
	}, nil
}

// IsLocalMode checks if local development mode is enabled
func IsLocalMode() bool {
	return os.Getenv("RENDER_USE_LOCAL_DEV") == "true"
}
