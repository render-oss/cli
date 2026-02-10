package storage

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStorageErrorMessage(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       string
	}{
		{
			name:       "400 bad request",
			statusCode: 400,
			want:       "bad request",
		},
		{
			name:       "401 unauthorized",
			statusCode: 401,
			want:       "access denied",
		},
		{
			name:       "403 forbidden",
			statusCode: 403,
			want:       "access denied",
		},
		{
			name:       "404 not found",
			statusCode: 404,
			want:       "object not found",
		},
		{
			name:       "409 conflict",
			statusCode: 409,
			want:       "conflict",
		},
		{
			name:       "413 payload too large",
			statusCode: 413,
			want:       "object too large",
		},
		{
			name:       "429 too many requests",
			statusCode: 429,
			want:       "rate limited, please try again later",
		},
		{
			name:       "500 internal server error",
			statusCode: 500,
			want:       "storage service temporarily unavailable",
		},
		{
			name:       "502 bad gateway",
			statusCode: 502,
			want:       "storage service temporarily unavailable",
		},
		{
			name:       "503 service unavailable",
			statusCode: 503,
			want:       "storage service temporarily unavailable",
		},
		{
			name:       "504 gateway timeout",
			statusCode: 504,
			want:       "storage service temporarily unavailable",
		},
		{
			name:       "unknown status code",
			statusCode: 418,
			want:       "unexpected error (HTTP 418)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storageErrorMessage(tt.statusCode)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestDownloadFromPresignedURL_StorageErrors(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		wantErrContain []string
		wantErrExclude []string
	}{
		{
			name:       "404 with S3 XML error",
			statusCode: 404,
			responseBody: `<?xml version="1.0" encoding="UTF-8"?>
<Error><Code>NoSuchKey</Code><Message>The specified key does not exist.</Message><Key>tea-d3tc3fuuk2gs73d0paug/foo/bar/test.txt</Key><RequestId>95N55HR7H0QBF3X9</RequestId><HostId>QfLXA55SGkqZ6VEKV97lgMjiFNWRFhpTj29FAylq2SOh2LJFMyvHuRdUjDu1IaZ/NmQR0znt4/0=</HostId></Error>`,
			wantErrContain: []string{"object not found"},
			wantErrExclude: []string{"NoSuchKey", "tea-d3tc3fuuk2gs73d0paug", "RequestId", "HostId"},
		},
		{
			name:       "403 with GCS error",
			statusCode: 403,
			responseBody: `<?xml version='1.0' encoding='UTF-8'?>
<Error><Code>AccessDenied</Code><Message>Access denied.</Message><Details>render-objects-bucket/some/path</Details></Error>`,
			wantErrContain: []string{"access denied"},
			wantErrExclude: []string{"render-objects-bucket", "AccessDenied", "Details"},
		},
		{
			name:           "500 server error",
			statusCode:     500,
			responseBody:   `Internal Server Error: connection to storage backend failed`,
			wantErrContain: []string{"storage service temporarily unavailable"},
			wantErrExclude: []string{"Internal Server Error", "storage backend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that returns the specified status and body
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			repo := &Repo{
				httpClient: server.Client(),
			}

			var buf bytes.Buffer
			_, err := repo.DownloadFromPresignedURL(context.Background(), server.URL, &buf)

			require.Error(t, err)
			for _, expected := range tt.wantErrContain {
				require.Contains(t, err.Error(), expected)
			}

			// Verify sensitive information is NOT exposed
			for _, excluded := range tt.wantErrExclude {
				require.NotContains(t, err.Error(), excluded,
					"error message should not contain sensitive info: %s", excluded)
			}
		})
	}
}

func TestUploadToPresignedURL_StorageErrors(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		wantErrContain []string
		wantErrExclude []string
	}{
		{
			name:       "403 with S3 access denied",
			statusCode: 403,
			responseBody: `<?xml version="1.0" encoding="UTF-8"?>
<Error><Code>AccessDenied</Code><Message>Access Denied</Message><RequestId>ABC123</RequestId><HostId>xyz789</HostId></Error>`,
			wantErrContain: []string{"access denied"},
			wantErrExclude: []string{"AccessDenied", "RequestId", "HostId", "ABC123", "xyz789"},
		},
		{
			name:       "413 payload too large",
			statusCode: 413,
			responseBody: `<?xml version="1.0" encoding="UTF-8"?>
<Error><Code>EntityTooLarge</Code><Message>Your proposed upload exceeds the maximum allowed size</Message><MaxSizeAllowed>5368709120</MaxSizeAllowed></Error>`,
			wantErrContain: []string{"object too large"},
			wantErrExclude: []string{"EntityTooLarge", "MaxSizeAllowed", "5368709120"},
		},
		{
			name:           "502 bad gateway",
			statusCode:     502,
			responseBody:   `Bad Gateway: upstream storage unavailable`,
			wantErrContain: []string{"storage service temporarily unavailable"},
			wantErrExclude: []string{"Bad Gateway", "upstream"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that returns the specified status and body
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			repo := &Repo{
				httpClient: server.Client(),
			}

			content := strings.NewReader("test content")
			err := repo.UploadToPresignedURL(context.Background(), server.URL, content, 12)

			require.Error(t, err)
			for _, expected := range tt.wantErrContain {
				require.Contains(t, err.Error(), expected)
			}

			// Verify sensitive information is NOT exposed
			for _, excluded := range tt.wantErrExclude {
				require.NotContains(t, err.Error(), excluded,
					"error message should not contain sensitive info: %s", excluded)
			}
		})
	}
}

func TestDownloadFromPresignedURL_Success(t *testing.T) {
	expectedContent := "file content here"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedContent))
	}))
	defer server.Close()

	repo := &Repo{
		httpClient: server.Client(),
	}

	var buf bytes.Buffer
	written, err := repo.DownloadFromPresignedURL(context.Background(), server.URL, &buf)

	require.NoError(t, err)
	require.Equal(t, int64(len(expectedContent)), written)
	require.Equal(t, expectedContent, buf.String())
}

func TestUploadToPresignedURL_Success(t *testing.T) {
	var receivedContent bytes.Buffer

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContent.ReadFrom(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := &Repo{
		httpClient: server.Client(),
	}

	content := "test upload content"
	err := repo.UploadToPresignedURL(context.Background(), server.URL, strings.NewReader(content), int64(len(content)))

	require.NoError(t, err)
	require.Equal(t, content, receivedContent.String())
}
