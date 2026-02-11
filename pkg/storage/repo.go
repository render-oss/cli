package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/render-oss/cli/pkg/client"
	storageclient "github.com/render-oss/cli/pkg/client/storage"
	"github.com/render-oss/cli/pkg/pointers"
)

// Repo handles REST API calls for object storage
type Repo struct {
	client     *client.ClientWithResponses
	httpClient *http.Client
}

// NewRepo creates a new storage Repo
func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{
		client:     c,
		httpClient: &http.Client{},
	}
}

// GetUploadURL requests a presigned URL for uploading an object
func (r *Repo) GetUploadURL(ctx context.Context, ownerId, region, key string, sizeBytes int64) (*storageclient.PutObjectOutput, error) {
	resp, err := r.client.PutObjectWithResponse(ctx, ownerId, client.Region(region), key, storageclient.PutObjectInput{
		SizeBytes: sizeBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, fmt.Errorf("upload URL request failed: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: no data returned")
	}

	return resp.JSON200, nil
}

// GetDownloadURL requests a presigned URL for downloading an object
func (r *Repo) GetDownloadURL(ctx context.Context, ownerId, region, key string) (*storageclient.GetObjectOutput, error) {
	resp, err := r.client.GetObjectWithResponse(ctx, ownerId, client.Region(region), key)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return nil, fmt.Errorf("object not found: %s", key)
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, fmt.Errorf("download URL request failed: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: no data returned")
	}

	return resp.JSON200, nil
}

// Delete deletes an object
func (r *Repo) Delete(ctx context.Context, ownerId, region, key string) error {
	resp, err := r.client.DeleteObjectWithResponse(ctx, ownerId, client.Region(region), key)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}

	if resp.StatusCode() == http.StatusNotFound {
		return fmt.Errorf("object not found: %s", key)
	}

	if resp.StatusCode() != http.StatusNoContent && resp.StatusCode() != http.StatusOK {
		if err := client.ErrorFromResponse(resp); err != nil {
			return fmt.Errorf("delete request failed: %w", err)
		}
		return fmt.Errorf("delete request failed with status %d", resp.StatusCode())
	}

	return nil
}

// List lists objects in object storage with pagination support
func (r *Repo) List(ctx context.Context, ownerId, region, cursor string, limit int) ([]ObjectInfo, string, error) {
	params := &client.ListObjectsParams{
		Limit: pointers.From(limit),
	}
	if cursor != "" {
		params.Cursor = &cursor
	}

	resp, err := r.client.ListObjectsWithResponse(ctx, ownerId, client.Region(region), params)
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute request: %w", err)
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, "", fmt.Errorf("list objects request failed: %w", err)
	}

	if resp.JSON200 == nil {
		return nil, "", nil
	}

	respOK := resp.JSON200
	items := respOK.Items
	objects := make([]ObjectInfo, len(items))
	for i, bwc := range items {
		objects[i] = ObjectInfo{
			Key:          bwc.Object.Key,
			SizeBytes:    bwc.Object.SizeBytes,
			LastModified: bwc.Object.LastModified,
		}
	}

	var nextCursor string
	if respOK.HasNext && respOK.NextCursor != nil {
		nextCursor = *respOK.NextCursor
	}

	return objects, nextCursor, nil
}

// UploadToPresignedURL uploads file content to a presigned URL
func (r *Repo) UploadToPresignedURL(ctx context.Context, presignedURL string, content io.Reader, contentLength int64) error {
	body := content
	if contentLength == 0 {
		// In Go's net/http, a ContentLength of 0 with a non-nil Body is treated as
		// unknown, causing the client to use chunked transfer encoding. Presigned S3
		// URLs do not support chunked encoding, so we use http.NoBody to ensure
		// Content-Length: 0 is sent instead.
		body = http.NoBody
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, presignedURL, body)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.ContentLength = contentLength

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute upload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New(storageErrorMessage(resp.StatusCode))
	}

	return nil
}

// DownloadFromPresignedURL downloads content from a presigned URL
func (r *Repo) DownloadFromPresignedURL(ctx context.Context, presignedURL string, dest io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, presignedURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to execute download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, errors.New(storageErrorMessage(resp.StatusCode))
	}

	written, err := io.Copy(dest, resp.Body)
	if err != nil {
		return written, fmt.Errorf("failed to write downloaded content: %w", err)
	}

	return written, nil
}

func storageErrorMessage(statusCode int) string {
	switch statusCode {
	case 400:
		return "bad request"
	case 401, 403:
		return "access denied"
	case 404:
		return "object not found"
	case 409:
		return "conflict"
	case 413:
		return "object too large"
	case 429:
		return "rate limited, please try again later"
	case 500, 502, 503, 504:
		return "storage service temporarily unavailable"
	default:
		return fmt.Sprintf("unexpected error (HTTP %d)", statusCode)
	}
}
