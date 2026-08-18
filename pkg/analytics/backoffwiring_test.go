package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/pkg/command"
)

func TestSendFileRecordsBackoff(t *testing.T) {
	testCases := []struct {
		name        string
		response    *http.Response
		responseErr error
		wantStatus  int
		wantBackoff time.Duration
		wantNone    bool
	}{
		{
			name:        "429 with Retry-After",
			response:    responseWithRetryAfter(http.StatusTooManyRequests, "429 Too Many Requests", "600"),
			wantStatus:  http.StatusTooManyRequests,
			wantBackoff: 10 * time.Minute,
		},
		{
			name:     "202 is a successful send",
			response: &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted"},
			wantNone: true,
		},
		{
			name:     "500 is not the API asking the CLI to stop",
			response: &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error"},
			wantNone: true,
		},
		{
			name:        "neither is a dead network",
			responseErr: errors.New("connection refused"),
			wantNone:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configDir := t.TempDir()
			t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
			path := writeRawEventFile(t, []byte(`{}`))
			apiClient := &fakeTelemetryClient{response: tc.response, err: tc.responseErr}
			before := time.Now()

			_ = newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

			require.Equal(t, 1, apiClient.calls)
			if tc.wantNone {
				requireNoBackoffState(t, configDir)
				return
			}
			got := readBackoffState(t, configDir)
			require.Equal(t, tc.wantStatus, got.StatusCode)
			require.WithinDuration(t, before.Add(tc.wantBackoff), got.Until, time.Minute)
		})
	}
}

func TestSendFileSkipsSendDuringBackoff(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	writeBackoffState(t, configDir, backoff{Until: time.Now().Add(10 * time.Minute), StatusCode: http.StatusServiceUnavailable})
	path := writeRawEventFile(t, []byte(`{}`))
	apiClient := &fakeTelemetryClient{}
	var diagnostics bytes.Buffer

	err := newTestSender(apiClient, true, true).SendFile(context.Background(), path, &diagnostics)

	require.NoError(t, err)
	require.Zero(t, apiClient.calls, "an active backoff must make no request")
	require.FileExists(t, backoffStatePath(configDir), "the backoff outlives the skipped send")
	require.NoFileExists(t, path, "the event file is consumed, not held for a retry")
	require.Contains(t, diagnostics.String(), "analytics: send skipped, backoff until")
}

// TestSendFileStillReportsRejections tests that a caller waiting on the child process learns the send
// failed through the exit code.
func TestSendFileStillReportsRejections(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	path := writeRawEventFile(t, []byte(`{}`))
	apiClient := &fakeTelemetryClient{
		response: &http.Response{StatusCode: http.StatusServiceUnavailable, Status: "503 Service Unavailable"},
	}
	var diagnostics bytes.Buffer

	err := newTestSender(apiClient, true, true).SendFile(context.Background(), path, &diagnostics)

	require.ErrorContains(t, err, "unexpected response 503 Service Unavailable")
	require.Contains(t, diagnostics.String(), "analytics: 503 response, sending paused until")
	require.Contains(t, diagnostics.String(), "analytics error:")
}

// TestSendDuringBackoff validates that we do not start an analytics subprocess while a backoff is in effect.
func TestSendDuringBackoff(t *testing.T) {
	t.Run("active backoff with logging disabled", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
		writeBackoffState(t, configDir, backoff{Until: time.Now().Add(10 * time.Minute), StatusCode: http.StatusTooManyRequests})
		sender, launcher, launcherCalls := senderWithFakeLauncher(t, true, false)
		installationIDCalls := 0
		sender.getInstallationID = func() (string, error) {
			installationIDCalls++
			return testInstallationID, nil
		}
		var stderr bytes.Buffer

		sender.Send(exampleExecutionResult(), &stderr)

		require.Zero(t, *launcherCalls, "an active backoff must not resolve a launcher")
		require.Zero(t, installationIDCalls, "an active backoff must not create analytics state")
		require.Empty(t, launcher.detachedPaths)
		require.Empty(t, launcher.syncPaths)
		require.Empty(t, eventFiles(t, configDir))
		require.Empty(t, stderr.String(), "a backoff is invisible with logging off")
	})

	t.Run("active backoff with logging enabled", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
		writeBackoffState(t, configDir, backoff{Until: time.Now().Add(10 * time.Minute), StatusCode: http.StatusTooManyRequests})
		sender, launcher, launcherCalls := senderWithFakeLauncher(t, true, true)
		var stderr bytes.Buffer

		sender.Send(exampleExecutionResult(), &stderr)

		require.Zero(t, *launcherCalls, "an active backoff must not resolve a launcher")
		require.Empty(t, launcher.detachedPaths)
		require.Empty(t, launcher.syncPaths)
		require.Empty(t, eventFiles(t, configDir))
		testassert.ContainsInOrder(t, stderr.String(),
			`"command":"render services list"`,
			"analytics: send skipped, backoff until",
			"(429)",
		)
	})

	// Reading backoff state is gated on sendingEnabled, so an opted-out run neither
	// consults it nor deletes the expired state it happens to find.
	t.Run("sending disabled with expired backoff", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
		writeBackoffState(t, configDir, backoff{Until: time.Now().Add(-time.Minute), StatusCode: http.StatusTooManyRequests})
		sender, _, _ := senderWithFakeLauncher(t, false, true)
		var stderr bytes.Buffer

		sender.Send(exampleExecutionResult(), &stderr)

		require.FileExists(t, backoffStatePath(configDir))
		require.NotContains(t, stderr.String(), "backoff")
	})
}

func senderWithFakeLauncher(t *testing.T, sendingEnabled, shouldLog bool) (*Sender, *fakeEventFileLauncher, *int) {
	t.Helper()
	sender := newTestSender(&fakeTelemetryClient{}, sendingEnabled, shouldLog)
	launcher := &fakeEventFileLauncher{}
	calls := 0
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		calls++
		return launcher, nil
	}
	return sender, launcher, &calls
}

func exampleExecutionResult() command.ExecutionResult {
	return command.ExecutionResult{
		CommandPath:    "render services list",
		CompletionKind: command.CompletionKindSuccess,
		StartedAt:      exampleStartedAt,
	}
}

func responseWithRetryAfter(statusCode int, status, retryAfter string) *http.Response {
	header := http.Header{}
	header.Set("Retry-After", retryAfter)
	return &http.Response{
		StatusCode: statusCode,
		Status:     status,
		Header:     header,
	}
}

func readBackoffState(t *testing.T, configDir string) backoff {
	t.Helper()
	data, err := os.ReadFile(backoffStatePath(configDir))
	require.NoError(t, err)
	var b backoff
	require.NoError(t, json.Unmarshal(data, &b))
	return b
}

func eventFiles(t *testing.T, configDir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(configDir, "state", "analytics", "events", "*"))
	require.NoError(t, err)
	return matches
}
