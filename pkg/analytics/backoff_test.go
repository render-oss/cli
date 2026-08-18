package analytics

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackoffFor(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	testCases := []struct {
		name        string
		statusCode  int
		retryAfter  string
		wantBackoff time.Duration
		wantNone    bool
	}{
		{
			name:        "a 429 without Retry-After uses the 429 fallback",
			statusCode:  http.StatusTooManyRequests,
			wantBackoff: 5 * time.Minute,
		},
		{
			name:        "a Retry-After above the floor is honored",
			statusCode:  http.StatusTooManyRequests,
			retryAfter:  "60",
			wantBackoff: time.Minute,
		},
		{
			name:        "a Retry-After below the floor is raised to the floor",
			statusCode:  http.StatusTooManyRequests,
			retryAfter:  "5",
			wantBackoff: 30 * time.Second,
		},
		{
			name:        "Retry-After may be an HTTP-date",
			statusCode:  http.StatusTooManyRequests,
			retryAfter:  now.Add(2 * time.Hour).Format(http.TimeFormat),
			wantBackoff: 2 * time.Hour,
		},
		{
			name:        "an HTTP-date already past is ignored and a fallback value is used",
			statusCode:  http.StatusTooManyRequests,
			retryAfter:  now.Add(-time.Hour).Format(http.TimeFormat),
			wantBackoff: 5 * time.Minute,
		},
		{
			name:        "an unparseable Retry-After is ignored and a fallback value is used",
			statusCode:  http.StatusTooManyRequests,
			retryAfter:  "soon",
			wantBackoff: 5 * time.Minute,
		},
		{
			name:        "a Retry-After above the cap is not honored",
			statusCode:  http.StatusTooManyRequests,
			retryAfter:  "999999999999",
			wantBackoff: 24 * time.Hour,
		},
		{
			name:        "a 503 without Retry-After uses the 503 fallback",
			statusCode:  http.StatusServiceUnavailable,
			wantBackoff: 30 * time.Minute,
		},
		{
			name:        "a 503 honors Retry-After",
			statusCode:  http.StatusServiceUnavailable,
			retryAfter:  "7200",
			wantBackoff: 2 * time.Hour,
		},
		{
			name:        "a 404 honors Retry-After",
			statusCode:  http.StatusNotFound,
			retryAfter:  "7200",
			wantBackoff: 2 * time.Hour,
		},
		{name: "a 202 calls for no backoff", statusCode: http.StatusAccepted, wantNone: true},
		{
			name:       "a Retry-After on an ineligible status calls for no backoff",
			statusCode: http.StatusInternalServerError,
			retryAfter: "7200",
			wantNone:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := backoffFor(postResponse{StatusCode: tc.statusCode, RetryAfter: tc.retryAfter}, now)

			if tc.wantNone {
				require.Equal(t, backoff{}, got)
				require.False(t, got.inEffect(now))
				return
			}
			require.True(t, got.inEffect(now))
			require.Equal(t, tc.statusCode, got.StatusCode)
			require.Equal(t, now.Add(tc.wantBackoff), got.Until)
		})
	}
}

func TestBackoffInEffect(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	require.False(t, backoff{}.inEffect(now), "the zero backoff is never in effect")
	require.False(t, backoff{Until: now.Add(-time.Second)}.inEffect(now))
	require.False(t, backoff{Until: now}.inEffect(now), "a backoff expiring exactly now is over")
	require.True(t, backoff{Until: now.Add(time.Second)}.inEffect(now))
	require.False(t, backoff{Until: now.Add(maxBackoff + time.Second)}.inEffect(now), "an overlong backoff is never in effect")
}

func TestLoadBackoff(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)

	t.Run("no backoff state", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)

		got := loadBackoff()

		require.Equal(t, backoff{}, got)
		require.False(t, got.inEffect(now))
		requireNoBackoffState(t, configDir)
	})

	t.Run("unexpired backoff", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
		writeBackoffState(t, configDir, backoff{Until: now.Add(10 * time.Minute), StatusCode: http.StatusServiceUnavailable})

		got := loadBackoff()

		require.True(t, got.inEffect(now))
		require.Equal(t, now.Add(10*time.Minute), got.Until.UTC())
		require.Equal(t, http.StatusServiceUnavailable, got.StatusCode)
		require.FileExists(t, backoffStatePath(configDir))
	})

	// Leaving unusable state in place is what keeps a read from racing another
	// invocation's write: deleting the file could erase a fresh backoff that
	// invocation recorded over the same path moments after the API asked for it.
	// [TestRecordBackoffReplacesStaleState] covers how the file gets cleared.
	t.Run("unusable backoff.json is ignored and left in place", func(t *testing.T) {
		testCases := []struct {
			name string
			seed func(t *testing.T, configDir string)
		}{
			{
				name: "expired",
				seed: func(t *testing.T, configDir string) {
					writeBackoffState(t, configDir, backoff{Until: now.Add(-time.Minute), StatusCode: http.StatusTooManyRequests})
				},
			},
			{
				name: "a backoff longer than the max",
				seed: func(t *testing.T, configDir string) {
					writeBackoffState(t, configDir, backoff{Until: now.Add(maxBackoff + time.Hour), StatusCode: http.StatusTooManyRequests})
				},
			},
			{
				name: "unparseable",
				seed: func(t *testing.T, configDir string) {
					writeRawBackoffState(t, configDir, []byte("not json"))
				},
			},
			{
				name: "a timestamp without a timezone",
				seed: func(t *testing.T, configDir string) {
					writeRawBackoffState(t, configDir, []byte(`{"until":"2026-08-18T12:00:00","status_code":429}`))
				},
			},
			{
				name: "larger than the max file size",
				seed: func(t *testing.T, configDir string) {
					writeRawBackoffState(t, configDir, bytes.Repeat([]byte("a"), maxBackoffFileSizeBytes+1))
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				configDir := t.TempDir()
				t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
				tc.seed(t, configDir)
				before, err := os.ReadFile(backoffStatePath(configDir))
				require.NoError(t, err)

				got := loadBackoff()

				require.False(t, got.inEffect(now))
				after, err := os.ReadFile(backoffStatePath(configDir))
				require.NoError(t, err, "reading must not delete state it cannot use")
				require.Equal(t, before, after, "reading must not rewrite it either")
			})
		}
	})
}

// TestRecordBackoffReplacesStaleState covers a new backoff overwriting an
// existing one, which is how state the read path leaves in place gets cleared.
func TestRecordBackoffReplacesStaleState(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	writeRawBackoffState(t, configDir, []byte("not json"))

	recordBackoff(backoff{Until: now.Add(10 * time.Minute), StatusCode: http.StatusServiceUnavailable})

	got := loadBackoff()
	require.True(t, got.inEffect(now))
	require.Equal(t, now.Add(10*time.Minute), got.Until.UTC())
	require.Equal(t, http.StatusServiceUnavailable, got.StatusCode)
}

func TestRecordBackoffIgnoresTheZeroBackoff(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)

	recordBackoff(backoff{})

	requireNoBackoffState(t, configDir)
}

// TestBackoffFileShape pins the on-disk format.
// Any changes to the file shape need to be backward compatible, as a file
// written by one version of the CLI must be readable by a future version.
func TestBackoffFileShape(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)

	recordBackoff(backoff{
		Until:      time.Now().Add(10 * time.Minute),
		StatusCode: http.StatusTooManyRequests,
	})

	data, err := os.ReadFile(backoffStatePath(configDir))
	require.NoError(t, err)
	var raw struct {
		Until      string `json:"until"`
		StatusCode int    `json:"status_code"`
	}
	require.NoError(t, json.Unmarshal(data, &raw))
	require.Equal(t, http.StatusTooManyRequests, raw.StatusCode)
	_, parseErr := time.Parse(time.RFC3339, raw.Until)
	require.NoError(t, parseErr, "the expiry is an RFC 3339 timestamp")
}

// backoffStatePath constructs the file's location.
// [backoffPath] is not used so that tests break if we change the file location,
// giving us a chance to make sure the change was intentional.
func backoffStatePath(configDir string) string {
	return filepath.Join(configDir, "state", "analytics", backoffFileName)
}

// writeBackoffState seeds state through [recordBackoff], so every read-path test
// also exercises the writer it has to agree with.
func writeBackoffState(t *testing.T, configDir string, b backoff) {
	t.Helper()
	recordBackoff(b)
	require.FileExists(t, backoffStatePath(configDir))
}

func writeRawBackoffState(t *testing.T, configDir string, contents []byte) {
	t.Helper()
	path := backoffStatePath(configDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, contents, 0o600))
}

func requireNoBackoffState(t *testing.T, configDir string) {
	t.Helper()
	require.NoFileExists(t, backoffStatePath(configDir))
}
