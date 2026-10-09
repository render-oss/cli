package cmd

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
)

const configPathEnvKey = "RENDER_CLI_CONFIG_PATH"

// setupLogoutTest points the CLI at a fresh temp config file and clears
// RENDER_API_KEY. Both are restored automatically by t.Setenv on cleanup.
// Returns the config path so callers can pre-populate it if needed.
func setupLogoutTest(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "cli.yaml")
	t.Setenv("RENDER_CLI_CONFIG_DIR", dir)
	t.Setenv(configPathEnvKey, configPath)
	t.Setenv("RENDER_API_KEY", "")
	return configPath
}

// runLogout executes the logout command and collects output into a buffer that is returned to the caller
func runLogout(t *testing.T) (string, error) {
	t.Helper()
	stdout, _, err := runLogoutWithContext(t, nil)
	return stdout, err
}

func runLogoutWithContext(t *testing.T, ctx context.Context) (string, string, error) {
	t.Helper()
	cmd := newLogoutCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if ctx != nil {
		cmd.SetContext(ctx)
	}
	err := cmd.RunE(cmd, nil)
	return stdout.String(), stderr.String(), err
}

func TestLogoutNotLoggedIn(t *testing.T) {
	setupLogoutTest(t)

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "not currently logged in")
	require.Contains(t, out, "render login")
}

func TestLogoutWithEnvVarOnly(t *testing.T) {
	setupLogoutTest(t)
	t.Setenv("RENDER_API_KEY", "rnd_env_token")

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "RENDER_API_KEY environment variable")
	require.Contains(t, out, "Render Dashboard")
}

func TestLogoutSuccess(t *testing.T) {
	configPath := setupLogoutTest(t)
	installIDPath := filepath.Join(filepath.Dir(configPath), "state", "installation-id.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(installIDPath), 0o755))
	require.NoError(t, os.WriteFile(installIDPath, []byte("persistent-state"), 0o600))
	server := renderapi.NewServer(t)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: server.URL(),
	}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "Successfully logged out")
	require.Contains(t, out, "render login")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	state, err := os.ReadFile(installIDPath)
	require.NoError(t, err)
	require.Equal(t, "persistent-state", string(state))
	require.Equal(t, "rnd_test_revoke", server.OAuth.Revokes.Only(t).AccessToken)
}

func TestLogoutWarnsWithoutSuccessWhenTokenRevocationFails(t *testing.T) {
	configPath := setupLogoutTest(t)
	server := renderapi.NewServer(t)
	server.OAuth.RespondWith(http.StatusInternalServerError)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: server.URL(),
	}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "Warning: something went wrong revoking your CLI token")
	require.Contains(t, out, "revoke token failed with status 500")
	require.Contains(t, out, "https://dashboard.render.com/settings#cli-tokens")
	require.NotContains(t, out, "Successfully logged out")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.Len(t, server.OAuth.Revokes.Instances, 1, "logout should still call the revoke endpoint")
}

func TestLogoutDoesNotPrintTokenWhenRevocationResponseEchoesIt(t *testing.T) {
	setupLogoutTest(t)
	const token = "rnd_secret_revoke_token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer "+token, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(r.Header.Get("Authorization")))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  token,
		Host: server.URL,
	}))

	output := command.TEXT
	ctx := command.SetFormatInContext(context.Background(), &output)
	stdout, stderr, err := runLogoutWithContext(t, ctx)
	require.NoError(t, err)
	require.Contains(t, stdout, "revoke token failed with status 500")
	require.NotContains(t, stdout, token)
	require.NotContains(t, stderr, token)
}

func TestLogoutInteractiveShowsSpinner(t *testing.T) {
	configPath := setupLogoutTest(t)
	server := renderapi.NewServer(t)
	server.OAuth.RespondWith(http.StatusNoContent, 75*time.Millisecond)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: server.URL(),
	}))

	out, stderr, err := runLogoutWithContext(t, nil)
	require.NoError(t, err)
	require.Contains(t, stderr, "Logging out")
	require.Contains(t, out, "Successfully logged out")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
}

func TestLogoutNonInteractiveDoesNotShowSpinner(t *testing.T) {
	configPath := setupLogoutTest(t)
	server := renderapi.NewServer(t)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: server.URL(),
	}))

	output := command.TEXT
	ctx := command.SetFormatInContext(context.Background(), &output)
	out, stderr, err := runLogoutWithContext(t, ctx)
	require.NoError(t, err)
	require.NotContains(t, stderr, "Logging out")
	require.Contains(t, out, "Successfully logged out")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.Len(t, server.OAuth.Revokes.Instances, 1, "logout should still call the revoke endpoint")
}

func TestLogoutBothEnvAndOAuth(t *testing.T) {
	configPath := setupLogoutTest(t)
	t.Setenv("RENDER_API_KEY", "rnd_env_token")
	server := renderapi.NewServer(t)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_oauth",
		Host: server.URL(),
	}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "OAuth credentials cleared")
	require.Contains(t, out, "RENDER_API_KEY")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.Len(t, server.OAuth.Revokes.Instances, 1, "logout should call the revoke endpoint")
	require.Equal(t, "rnd_env_token", os.Getenv("RENDER_API_KEY"), "logout should not modify RENDER_API_KEY")
}

func TestLogoutWarnsWithEnvKeyNoteWhenTokenRevocationFails(t *testing.T) {
	configPath := setupLogoutTest(t)
	t.Setenv("RENDER_API_KEY", "rnd_env_token")
	server := renderapi.NewServer(t)
	server.OAuth.RespondWith(http.StatusInternalServerError)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_oauth",
		Host: server.URL(),
	}))
	require.NoError(t, config.SetDashboardURL("https://dashboard.staging.render.com/verify"))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "Warning: something went wrong revoking your CLI token")
	require.Contains(t, out, "https://dashboard.staging.render.com/settings#cli-tokens")
	require.Contains(t, out, "Note: RENDER_API_KEY is still set in your environment.")
	require.NotContains(t, out, "OAuth credentials cleared")
	require.NotContains(t, out, "Successfully logged out")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.Len(t, server.OAuth.Revokes.Instances, 1, "logout should still call the revoke endpoint")
	require.Equal(t, "rnd_env_token", os.Getenv("RENDER_API_KEY"), "logout should not modify RENDER_API_KEY")
}

func TestLogoutDoesNotEmitAnalytics(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{
		noticeMarkerPresent: true,
		allowSubprocess:     true,
	})
	configPath := filepath.Join(harness.configDir, "cli.yaml")
	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: harness.server.URL(),
	}))
	require.FileExists(t, configPath)

	result := harness.execute("logout")

	require.Equal(t, 0, result.ExitCode)
	require.False(t, result.AnalyticsEligible)
	require.True(t, result.AnalyticsNoticeEligible)
	require.Len(t, harness.server.OAuth.Revokes.Instances, 1, "logout should exercise the credential revocation path")
	require.NoFileExists(t, configPath, "logout should delete the OAuth config")
	require.Equal(t, "test-api-key", os.Getenv("RENDER_API_KEY"), "analytics should remain able to authenticate")
	require.Empty(t, harness.server.CliTelemetry.Instances)
}
