package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
)

// TestTelemetryBackoffAcrossInvocations is an integration test that exercises
// the backoff mechanism. This test runs real analytics subprocesses. The fake
// API server is programmed to issue a retry-after, and the test ensures it is
// honored on a subsequent invocation.
func TestTelemetryBackoffAcrossInvocations(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: analyticsWorkspaceID, Name: "Analytics Workspace"}))
	t.Setenv("RENDER_WORKSPACE", analyticsWorkspaceID)
	configDir := t.TempDir()

	retryAfter := 10 * time.Minute
	server.CliTelemetry.RespondWithRetryAfter(http.StatusTooManyRequests, retryAfterSeconds(retryAfter))
	before := time.Now()
	first := executeWithAnalytics(t, server, configDir, true, "postgres", "list", "--output", "json")

	require.Equal(t, 0, first.Result.ExitCode)
	require.Empty(t, server.CliTelemetry.Instances, "a rejected event is never collected")
	require.Equal(t, 1, telemetryRequestCount(server))

	state := readBackoffState(t, configDir)
	require.Equal(t, http.StatusTooManyRequests, state.StatusCode)
	require.WithinDuration(t, before.Add(retryAfter), state.Until, time.Minute,
		"Retry-After outranks the 5m default")

	second := executeWithAnalytics(t, server, configDir, true, "postgres", "list", "--output", "json")

	require.Equal(t, 1, telemetryRequestCount(server), "an active backoff must make no request")
	require.Empty(t, analyticsEventFiles(t, configDir), "an active backoff must not write an event file")
	require.Equal(t, first.Result.ExitCode, second.Result.ExitCode, "an active backoff must not change the exit code")
	require.Equal(t, first.Stdout, second.Stdout, "an active backoff must not change command output")
	require.Equal(t, first.Stderr, second.Stderr, "an active backoff must not change command output")
}

func TestTelemetryResumesAfterBackoffExpires(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: analyticsWorkspaceID, Name: "Analytics Workspace"}))
	t.Setenv("RENDER_WORKSPACE", analyticsWorkspaceID)
	configDir := t.TempDir()
	writeExpiredBackoffState(t, configDir)

	run := executeWithAnalytics(t, server, configDir, true, "postgres", "list", "--output", "json")

	require.Equal(t, 0, run.Result.ExitCode)
	require.Len(t, server.CliTelemetry.Instances, 1, "an expired backoff must not suppress the send")
	require.FileExists(t, backoffStatePath(configDir), "expired backoff state remains until it is overwritten")
}

// retryAfterSeconds formats a duration as a Retry-After header value, which
// RFC 9110 specifies in whole seconds.
func retryAfterSeconds(d time.Duration) string {
	return strconv.Itoa(int(d.Seconds()))
}

func telemetryRequestCount(server *renderapi.Server) int {
	count := 0
	for _, request := range server.Requests {
		if strings.Contains(request.URI, "cli-telemetry-events") {
			count++
		}
	}
	return count
}

// backoffState mirrors the on-disk shape of pkg/analytics' backoff data. It is
// spelled out here rather than imported so the file format is regression-tested
// from outside the package that writes it.
type backoffState struct {
	Until      time.Time `json:"until"`
	StatusCode int       `json:"status_code"`
}

func backoffStatePath(configDir string) string {
	return filepath.Join(configDir, "state", "analytics", "backoff.json")
}

func readBackoffState(t *testing.T, configDir string) backoffState {
	t.Helper()
	data, err := os.ReadFile(backoffStatePath(configDir))
	require.NoError(t, err)
	var state backoffState
	require.NoError(t, json.Unmarshal(data, &state))
	return state
}

func writeExpiredBackoffState(t *testing.T, configDir string) {
	t.Helper()
	data, err := json.Marshal(backoffState{
		Until:      time.Now().Add(-time.Minute),
		StatusCode: http.StatusServiceUnavailable,
	})
	require.NoError(t, err)
	path := backoffStatePath(configDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

func analyticsEventFiles(t *testing.T, configDir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(configDir, "state", "analytics", "events", "*"))
	require.NoError(t, err)
	return matches
}
