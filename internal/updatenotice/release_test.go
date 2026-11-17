package updatenotice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/cfg"
)

func TestReleaseClient_LatestRelease(t *testing.T) {
	testCases := []struct {
		name          string
		status        int
		body          string
		wantVersion   string
		wantPublished time.Time
		wantErr       string
	}{
		{
			name:          "returns the version and publication time",
			status:        http.StatusOK,
			body:          `{"tag_name": "v2.33.5", "published_at": "2026-11-02T15:04:05Z"}`,
			wantVersion:   "2.33.5",
			wantPublished: time.Date(2026, 11, 2, 15, 4, 5, 0, time.UTC),
		},
		{
			name:    "reports a rate limit response",
			status:  http.StatusForbidden,
			body:    `{"message": "API rate limit exceeded"}`,
			wantErr: "status 403",
		},
		{
			name:    "rejects a tag that is not a version",
			status:  http.StatusOK,
			body:    `{"tag_name": "nightly"}`,
			wantErr: "nightly",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath, gotUserAgent string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath, gotUserAgent = r.URL.Path, r.Header.Get("User-Agent")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			release, err := NewReleaseClient(server.URL + "/repos/render-oss/cli/").LatestRelease(context.Background())

			require.Equal(t, "/repos/render-oss/cli/releases/latest", gotPath)
			require.Equal(t, "render-cli/"+cfg.Version, gotUserAgent)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantVersion, release.Version.String())
			require.True(t, tc.wantPublished.Equal(release.PublishedAt))
		})
	}
}

func TestReleaseClient_LatestReleaseStopsWhenCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewReleaseClient(server.URL).LatestRelease(ctx)

	require.ErrorIs(t, err, context.Canceled)
}
