package storage

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLocalService_Upload(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		content string
		wantErr bool
	}{
		{
			name:    "simple upload",
			key:     "test/file.txt",
			content: "hello world",
		},
		{
			name:    "key with special characters",
			key:     "uploads/images/photo (1).jpg",
			content: "image content",
		},
		{
			name:    "key with spaces",
			key:     "my files/document.txt",
			content: "document content",
		},
		{
			name:    "key with unicode",
			key:     "uploads/测试/文件.txt",
			content: "unicode content",
		},
		{
			name:    "deep path",
			key:     "a/b/c/d/e/f/file.txt",
			content: "deep content",
		},
		{
			name:    "key with URL special chars",
			key:     "files/file%20with%20encoding.txt",
			content: "encoded content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			basePath := t.TempDir()
			svc := NewLocalService(basePath, "oregon", "usr-123")

			// Create source file
			srcFile := filepath.Join(t.TempDir(), "source.txt")
			require.NoError(t, os.WriteFile(srcFile, []byte(tt.content), 0644))

			// Upload
			result, err := svc.Upload(context.Background(), tt.key, srcFile)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.key, result.Key)
			require.Equal(t, "oregon", result.Region)
			require.Equal(t, int64(len(tt.content)), result.SizeBytes)

			// Verify file was created at correct path
			expectedPath := filepath.Join(basePath, "oregon", "usr-123", url.PathEscape(tt.key))
			require.FileExists(t, expectedPath)

			// Verify content
			content, err := os.ReadFile(expectedPath)
			require.NoError(t, err)
			require.Equal(t, tt.content, string(content))
		})
	}
}

func TestLocalService_Upload_FileNotFound(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	_, err := svc.Upload(context.Background(), "test/key", "/nonexistent/file.txt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to stat file")
}

func TestLocalService_Download(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	// Upload a file first
	key := "test/download.txt"
	content := "download test content"
	srcFile := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte(content), 0644))

	_, err := svc.Upload(context.Background(), key, srcFile)
	require.NoError(t, err)

	// Download to buffer
	var buf []byte
	dest := &bufferWriter{buf: &buf}

	result, err := svc.Download(context.Background(), key, dest)
	require.NoError(t, err)
	require.Equal(t, key, result.Key)
	require.Equal(t, "oregon", result.Region)
	require.Equal(t, int64(len(content)), result.SizeBytes)
	require.Equal(t, content, string(buf))
}

func TestLocalService_Download_NotFound(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	var buf []byte
	dest := &bufferWriter{buf: &buf}

	_, err := svc.Download(context.Background(), "nonexistent/key", dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "object not found")
}

func TestLocalService_Delete(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	// Upload a file first
	key := "test/delete.txt"
	content := "delete test"
	srcFile := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte(content), 0644))

	_, err := svc.Upload(context.Background(), key, srcFile)
	require.NoError(t, err)

	// Verify file exists
	objectPath := filepath.Join(basePath, "oregon", "usr-123", url.PathEscape(key))
	require.FileExists(t, objectPath)

	// Delete
	result, err := svc.Delete(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, key, result.Key)
	require.Equal(t, "oregon", result.Region)

	// Verify file is deleted
	require.NoFileExists(t, objectPath)
}

func TestLocalService_Delete_NotFound(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	_, err := svc.Delete(context.Background(), "nonexistent/key")
	require.Error(t, err)
	require.Contains(t, err.Error(), "object not found")
}

func TestLocalService_Exists(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	// Upload a file
	key := "test/exists.txt"
	srcFile := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0644))

	_, err := svc.Upload(context.Background(), key, srcFile)
	require.NoError(t, err)

	// Check exists
	exists, err := svc.Exists(context.Background(), key)
	require.NoError(t, err)
	require.True(t, exists)

	// Check non-existent
	exists, err = svc.Exists(context.Background(), "nonexistent/key")
	require.NoError(t, err)
	require.False(t, exists)
}

func TestLocalService_objectPath(t *testing.T) {
	svc := NewLocalService("/base", "oregon", "usr-123")

	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "simple key",
			key:  "file.txt",
			want: "/base/oregon/usr-123/file.txt",
		},
		{
			name: "key with path",
			key:  "path/to/file.txt",
			want: "/base/oregon/usr-123/path%2Fto%2Ffile.txt",
		},
		{
			name: "key with spaces",
			key:  "file with spaces.txt",
			want: "/base/oregon/usr-123/file%20with%20spaces.txt",
		},
		{
			name: "key with special chars",
			key:  "file(1).txt",
			want: "/base/oregon/usr-123/file%281%29.txt",
		},
		{
			name: "key with unicode",
			key:  "测试/文件.txt",
			want: "/base/oregon/usr-123/%E6%B5%8B%E8%AF%95%2F%E6%96%87%E4%BB%B6.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.objectPath(tt.key)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestLocalService_objectPath_RegionAndBucket(t *testing.T) {
	svc := NewLocalService("/base", "virginia", "tea-456")

	key := "test/file.txt"
	got := svc.objectPath(key)

	// Should include region and bucket in path
	require.Contains(t, got, "virginia")
	require.Contains(t, got, "tea-456")
	require.Contains(t, got, url.PathEscape(key))
}

func TestLocalService_cleanupEmptyParents(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	// Upload a file in a nested path
	key := "a/b/c/file.txt"
	srcFile := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0644))

	_, err := svc.Upload(context.Background(), key, srcFile)
	require.NoError(t, err)

	// Delete the file
	_, err = svc.Delete(context.Background(), key)
	require.NoError(t, err)

	// Verify empty parent directories are cleaned up
	// But region directory should remain
	regionPath := filepath.Join(basePath, "oregon")
	require.DirExists(t, regionPath)

	// Bucket directory should be cleaned up (it's empty)
	bucketPath := filepath.Join(regionPath, "usr-123")
	_, err = os.Stat(bucketPath)
	require.True(t, os.IsNotExist(err))
}

func TestLocalService_cleanupEmptyParents_PreservesRegion(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	// Upload and delete a file
	key := "test/file.txt"
	srcFile := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte("content"), 0644))

	_, err := svc.Upload(context.Background(), key, srcFile)
	require.NoError(t, err)

	_, err = svc.Delete(context.Background(), key)
	require.NoError(t, err)

	// Region directory should still exist
	regionPath := filepath.Join(basePath, "oregon")
	require.DirExists(t, regionPath)
}

func TestNewLocalService_DefaultBasePath(t *testing.T) {
	svc := NewLocalService("", "oregon", "usr-123")
	require.Equal(t, DefaultLocalBasePath, svc.basePath)
	require.Equal(t, "oregon", svc.region)
	require.Equal(t, "usr-123", svc.bucketName)
}

func TestLocalService_UploadDownloadRoundTrip(t *testing.T) {
	basePath := t.TempDir()
	svc := NewLocalService(basePath, "oregon", "usr-123")

	// Upload
	key := "roundtrip/test.txt"
	originalContent := "roundtrip test content"
	srcFile := filepath.Join(t.TempDir(), "source.txt")
	require.NoError(t, os.WriteFile(srcFile, []byte(originalContent), 0644))

	uploadResult, err := svc.Upload(context.Background(), key, srcFile)
	require.NoError(t, err)
	require.Equal(t, int64(len(originalContent)), uploadResult.SizeBytes)

	// Download
	var buf []byte
	dest := &bufferWriter{buf: &buf}

	downloadResult, err := svc.Download(context.Background(), key, dest)
	require.NoError(t, err)
	require.Equal(t, uploadResult.SizeBytes, downloadResult.SizeBytes)
	require.Equal(t, originalContent, string(buf))
}

// bufferWriter is a helper for testing Download operations
type bufferWriter struct {
	buf *[]byte
}

func (b *bufferWriter) Write(p []byte) (n int, err error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}
