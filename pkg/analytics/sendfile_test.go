package analytics

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
)

func TestSenderSendFileHappyPath(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	payload := client.CreateCliTelemetryEventJSONRequestBody{
		Command:        "render services list",
		CompletionKind: telemetryclient.Success,
		ExitCode:       0,
	}
	path, err := writeEventFile(payload)
	require.NoError(t, err)

	body := &eventPOSTResponseBody{}
	apiClient := &fakeTelemetryClient{
		response: &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted", Body: body},
	}
	apiClient.handler = func(ctx context.Context, got client.CreateCliTelemetryEventJSONRequestBody) (*http.Response, error) {
		_, statErr := os.Lstat(path)
		require.ErrorIs(t, statErr, os.ErrNotExist, "event file must be removed before the request")
		return apiClient.response, nil
	}

	err = newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

	require.NoError(t, err)
	require.Equal(t, 1, apiClient.calls)
	require.Equal(t, payload, apiClient.payload)
	require.True(t, body.closed)
}

// TestSenderSendFileWithRelativeConfigDirOverride pins that a relative
// RENDER_CLI_CONFIG_DIR cannot make the writer and SendFile's path validation
// disagree: writeEventFile must hand back an absolute path that SendFile
// accepts, or every event file would be written and then orphaned.
func TestSenderSendFileWithRelativeConfigDirOverride(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("RENDER_CLI_CONFIG_DIR", ".render-local")
	path, err := writeEventFile(client.CreateCliTelemetryEventJSONRequestBody{Command: "render services list"})
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(path))
	apiClient := &fakeTelemetryClient{
		response: &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted"},
	}

	err = newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

	require.NoError(t, err)
	require.Equal(t, 1, apiClient.calls)
	_, statErr := os.Lstat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestSenderSendFileRequestTimeout(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	path := writeRawEventFile(t, []byte(`{}`))
	var deadlineRemaining time.Duration
	apiClient := &fakeTelemetryClient{}
	apiClient.handler = func(ctx context.Context, _ client.CreateCliTelemetryEventJSONRequestBody) (*http.Response, error) {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		deadlineRemaining = time.Until(deadline)
		return &http.Response{StatusCode: http.StatusAccepted}, nil
	}

	err := newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

	require.NoError(t, err)
	require.LessOrEqual(t, deadlineRemaining, sendFileRequestTimeout)
	require.Greater(t, deadlineRemaining, sendFileRequestTimeout-100*time.Millisecond)
}

func TestSenderSendFileRejectsInvalidPath(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	validPath := writeRawEventFile(t, []byte(`{"command":"render services list"}`))
	eventsDir := filepath.Dir(validPath)
	separator := string(filepath.Separator)

	testCases := []struct {
		name string
		path string
	}{
		{name: "relative", path: filepath.Base(validPath)},
		{name: "directory traversal", path: eventsDir + separator + "nested" + separator + ".." + separator + filepath.Base(validPath)},
		{name: "outside events directory", path: filepath.Join(filepath.Dir(eventsDir), filepath.Base(validPath))},
		{name: "wrong prefix", path: filepath.Join(eventsDir, "other.json")},
		{name: "wrong suffix", path: filepath.Join(eventsDir, "event-other.txt")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			apiClient := &fakeTelemetryClient{}

			err := newTestSender(apiClient, true, false).SendFile(context.Background(), tc.path, io.Discard)

			require.Error(t, err)
			require.Zero(t, apiClient.calls)
		})
	}

	// Rejected paths must not consume another event, including traversal paths
	// that resolve to an existing valid event file.
	_, err := os.Lstat(validPath)
	require.NoError(t, err, "path validation must not consume the valid event file")
}

func TestSenderSendFileRejectsUnsafeFilesystemEntries(t *testing.T) {
	testCases := []struct {
		name  string
		setup func(t *testing.T, configDir string) string
	}{
		{
			name: "symlinked file",
			setup: func(t *testing.T, configDir string) string {
				t.Helper()
				target := writeRawEventFile(t, []byte(`{}`))
				path := filepath.Join(filepath.Dir(target), "event-link.json")
				if err := os.Symlink(target, path); err != nil {
					if runtime.GOOS == "windows" {
						t.Skipf("creating symlinks is not permitted: %v", err)
					}
					require.NoError(t, err)
				}
				return path
			},
		},
		{
			name: "event file is actually a directory",
			setup: func(t *testing.T, configDir string) string {
				t.Helper()
				eventsDir := makeEventDirectories(t, configDir)
				path := filepath.Join(eventsDir, "event-directory.json")
				require.NoError(t, os.Mkdir(path, 0o755))
				return path
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configDir := t.TempDir()
			t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
			path := tc.setup(t, configDir)
			apiClient := &fakeTelemetryClient{}

			err := newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

			require.Error(t, err)
			require.Zero(t, apiClient.calls)
			_, statErr := os.Lstat(path)
			require.NoError(t, statErr, "unsafe entries must not be removed")
		})
	}
}

func TestSenderSendFileRejectsAndRemovesInvalidContent(t *testing.T) {
	testCases := []struct {
		name    string
		content []byte
		wantErr string
	}{
		{name: "empty", content: nil, wantErr: "must contain a JSON object"},
		{name: "not an object", content: []byte(`[]`), wantErr: "must contain a JSON object"},
		{name: "null", content: []byte(`null`), wantErr: "must contain a JSON object"},
		{name: "multiple objects", content: []byte(`{}{}`), wantErr: "decode analytics event file"},
		{name: "trailing value", content: []byte(`{"command":"a"}trailing`), wantErr: "decode analytics event file"},
		{
			name:    "over size limit",
			content: []byte(`{"command":"` + strings.Repeat("x", 64*1024) + `"}`),
			wantErr: "exceeds 65536 bytes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
			path := writeRawEventFile(t, tc.content)
			apiClient := &fakeTelemetryClient{}

			err := newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

			require.ErrorContains(t, err, tc.wantErr)
			require.Zero(t, apiClient.calls)
			_, statErr := os.Lstat(path)
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

// TestSenderSendFileConsumesEventWithoutSendingWhenDisabled covers an
// unexpected parent/child gate mismatch. The parent normally writes a file
// only when sending is enabled, but a disabled child still consumes it so the
// child's current opt-out wins and the event cannot be sent later. Nothing is
// sent, so nothing reaches diagnostics.
func TestSenderSendFileConsumesEventWithoutSendingWhenDisabled(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	path := writeRawEventFile(t, []byte(`{}`))
	apiClient := &fakeTelemetryClient{}
	var diagnostics bytes.Buffer

	err := newTestSender(apiClient, false, true).SendFile(context.Background(), path, &diagnostics)

	require.NoError(t, err)
	require.Zero(t, apiClient.calls)
	require.Empty(t, diagnostics.String())
	_, statErr := os.Lstat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestSenderSendFileRemovesEventAfterRequestFailure(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	path := writeRawEventFile(t, []byte(`{}`))
	requestErr := errors.New("request failed")
	apiClient := &fakeTelemetryClient{err: requestErr}

	err := newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

	require.ErrorIs(t, err, requestErr)
	require.Equal(t, 1, apiClient.calls)
	_, statErr := os.Lstat(path)
	require.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestSenderSendFileSendsAfterRemoveFailure(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	path := writeRawEventFile(t, []byte(`{}`))
	requireUndeletableEventFile(t, path)
	apiClient := &fakeTelemetryClient{
		response: &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted"},
	}

	err := newTestSender(apiClient, true, false).SendFile(context.Background(), path, io.Discard)

	require.ErrorContains(t, err, "remove analytics event file")
	require.Equal(t, 1, apiClient.calls)
	_, statErr := os.Lstat(path)
	require.NoError(t, statErr)
}

func TestSenderSendFileReturnsHTTPFailure(t *testing.T) {
	testCases := []struct {
		name            string
		shouldLog       bool
		wantDiagnostics string
	}{
		{name: "logging disabled", shouldLog: false, wantDiagnostics: ""},
		{
			name:            "logging enabled",
			shouldLog:       true,
			wantDiagnostics: "analytics error: send analytics event: unexpected response 500 Internal Server Error\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
			path := writeRawEventFile(t, []byte(`{}`))
			body := &eventPOSTResponseBody{}
			apiClient := &fakeTelemetryClient{
				response: &http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error", Body: body},
			}
			var diagnostics bytes.Buffer

			err := newTestSender(apiClient, true, tc.shouldLog).SendFile(context.Background(), path, &diagnostics)

			require.ErrorContains(t, err, "unexpected response 500 Internal Server Error")
			require.Equal(t, 1, apiClient.calls)
			require.True(t, body.closed)
			require.Equal(t, tc.wantDiagnostics, diagnostics.String())
			_, statErr := os.Lstat(path)
			require.ErrorIs(t, statErr, os.ErrNotExist)
		})
	}
}

// TestSenderSendFileDiagnostics pins the diagnostics contract: a successful
// send writes one outcome line to the caller's writer only when analytics
// logging is enabled, mirroring Send's internal gate. Failure narration under
// the same gate is covered by TestSenderSendFileReturnsHTTPFailure.
func TestSenderSendFileDiagnostics(t *testing.T) {
	testCases := []struct {
		name      string
		shouldLog bool
		want      string
	}{
		{name: "logging enabled", shouldLog: true, want: "analytics response: 202 Accepted\n"},
		{name: "logging disabled", shouldLog: false, want: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
			path := writeRawEventFile(t, []byte(`{}`))
			apiClient := &fakeTelemetryClient{
				response: &http.Response{StatusCode: http.StatusAccepted, Status: "202 Accepted"},
			}
			var diagnostics bytes.Buffer

			err := newTestSender(apiClient, true, tc.shouldLog).SendFile(context.Background(), path, &diagnostics)

			require.NoError(t, err)
			require.Equal(t, tc.want, diagnostics.String())
		})
	}
}

func writeRawEventFile(t *testing.T, content []byte) string {
	t.Helper()
	configDir := os.Getenv("RENDER_CLI_CONFIG_DIR")
	require.NotEmpty(t, configDir)
	path := filepath.Join(makeEventDirectories(t, configDir), "event-test.json")
	require.NoError(t, os.WriteFile(path, content, 0o600))
	return path
}

func makeEventDirectories(t *testing.T, configDir string) string {
	t.Helper()
	eventsDir := filepath.Join(configDir, "state", "analytics", "events")
	require.NoError(t, os.MkdirAll(eventsDir, 0o755))
	return eventsDir
}

// requireUndeletableEventFile makes removal of path fail for the rest of the
// test, restoring it afterward. On Windows, os.Open omits FILE_SHARE_DELETE, so
// an extra open handle prevents deleting this exact file. Elsewhere, the helper
// uses directory permissions and proves the precondition with a probe file.
func requireUndeletableEventFile(t *testing.T, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		file, err := os.Open(path)
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, file.Close())
			require.NoError(t, os.Remove(path))
		})
		require.Error(t, os.Remove(path),
			"the test needs the open file to reject deletion")
		return
	}

	currentUser, err := user.Current()
	require.NoError(t, err)
	require.NotEqual(t, "0", currentUser.Uid,
		"the test requires a non-root user because root can delete from read-only directories")

	dir := filepath.Dir(path)
	probe := filepath.Join(dir, "probe")
	require.NoError(t, os.WriteFile(probe, nil, 0o600))
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() {
		require.NoError(t, os.Chmod(dir, 0o755))
		_ = os.Remove(probe)
	})

	require.Error(t, os.Remove(probe),
		"the test needs the read-only directory to reject deletion")
}
