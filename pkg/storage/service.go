package storage

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
)

// StorageService defines the interface for object storage operations
type StorageService interface {
	Upload(ctx context.Context, key, filePath string) (*UploadResult, error)
	Download(ctx context.Context, key string, dest io.Writer) (*DownloadResult, error)
	Delete(ctx context.Context, key string) (*DeleteResult, error)
	List(ctx context.Context, cursor string, limit int) (*ListResult, error)
}

// CloudService implements StorageService for Render cloud storage
type CloudService struct {
	repo    *Repo
	ownerId string
	region  string
}

// NewCloudService creates a new CloudService
func NewCloudService(c *client.ClientWithResponses, ownerId, region string) *CloudService {
	return &CloudService{
		repo:    NewRepo(c),
		ownerId: ownerId,
		region:  region,
	}
}

// Upload uploads a file to cloud object storage
func (s *CloudService) Upload(ctx context.Context, key, filePath string) (*UploadResult, error) {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	// Request presigned upload URL
	uploadURL, err := s.repo.GetUploadURL(ctx, s.ownerId, s.region, key, fileInfo.Size())
	if err != nil {
		return nil, fmt.Errorf("failed to get upload URL: %w", err)
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Upload to presigned URL
	if err := s.repo.UploadToPresignedURL(ctx, uploadURL.Url, file, fileInfo.Size()); err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	return &UploadResult{
		Key:       key,
		Region:    s.region,
		SizeBytes: fileInfo.Size(),
		LocalPath: filePath,
	}, nil
}

// Download downloads an object from cloud storage
func (s *CloudService) Download(ctx context.Context, key string, dest io.Writer) (*DownloadResult, error) {
	// Request presigned download URL
	downloadURL, err := s.repo.GetDownloadURL(ctx, s.ownerId, s.region, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get download URL: %w", err)
	}

	// Download from presigned URL
	written, err := s.repo.DownloadFromPresignedURL(ctx, downloadURL.Url, dest)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	return &DownloadResult{
		Key:       key,
		Region:    s.region,
		SizeBytes: written,
	}, nil
}

// Delete deletes an object from cloud storage
func (s *CloudService) Delete(ctx context.Context, key string) (*DeleteResult, error) {
	if err := s.repo.Delete(ctx, s.ownerId, s.region, key); err != nil {
		return nil, err
	}

	return &DeleteResult{
		Key:    key,
		Region: s.region,
	}, nil
}

// List lists objects in cloud storage with pagination support
func (s *CloudService) List(ctx context.Context, cursor string, limit int) (*ListResult, error) {
	objects, nextCursor, err := s.repo.List(ctx, s.ownerId, s.region, cursor, limit)
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Objects: objects,
		Cursor:  nextCursor,
	}, nil
}

// ServiceConfig holds configuration for creating a storage service
type ServiceConfig struct {
	Local    bool
	OwnerId  string
	Region   string
	BasePath string
}

// NewService creates the appropriate StorageService based on configuration
// Priority: --local flag > RENDER_USE_LOCAL_DEV=true > default cloud
func NewService(c *client.ClientWithResponses, cfg ServiceConfig) (StorageService, error) {
	if cfg.Local || IsLocalMode() {
		return newLocalServiceWithConfig(cfg)
	}
	return newCloudServiceWithConfig(c, cfg)
}

func newLocalServiceWithConfig(cfg ServiceConfig) (*LocalService, error) {
	// Get ownerId (bucket name) - default to workspace ID
	bucketName := cfg.OwnerId
	if bucketName == "" {
		var err error
		bucketName, err = config.WorkspaceID()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace ID: %w", err)
		}
	}

	// Region is required for local storage to match cloud structure
	region := cfg.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for object storage operations")
	}

	return NewLocalService(cfg.BasePath, region, bucketName), nil
}

func newCloudServiceWithConfig(c *client.ClientWithResponses, cfg ServiceConfig) (*CloudService, error) {
	ownerId := cfg.OwnerId
	if ownerId == "" {
		var err error
		ownerId, err = config.WorkspaceID()
		if err != nil {
			return nil, fmt.Errorf("failed to get workspace ID: %w", err)
		}
	}

	// Region is required - validation should happen at command level
	region := cfg.Region
	if region == "" {
		return nil, fmt.Errorf("region is required for object storage operations")
	}

	return NewCloudService(c, ownerId, region), nil
}

// NewServiceFromContext creates a StorageService, getting the client from context if needed
// Priority: --local flag > RENDER_USE_LOCAL_DEV=true > default cloud
func NewServiceFromContext(ctx context.Context, cfg ServiceConfig) (StorageService, error) {
	if cfg.Local || IsLocalMode() {
		return newLocalServiceWithConfig(cfg)
	}

	// For cloud mode, create a new default client
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return NewService(c, cfg)
}
