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
	t.Setenv(configPathEnvKey, configPath)
	t.Setenv("RENDER_API_KEY", "")
	return configPath
}

// setupLogoutEndpoint starts a test server that handles token revocation.
// It returns the API host to store in config.APIConfig.Host and a revokeCalled
// function that reports whether the logout command called /oauth/revoke.
func setupLogoutEndpoint(t *testing.T, statusCode int, delay ...time.Duration) (string, func() bool) {
	t.Helper()

	responseDelay := time.Duration(0)
	if len(delay) > 0 {
		responseDelay = delay[0]
	}

	revokeCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/oauth/revoke" {
			revokeCalled = true
			time.Sleep(responseDelay)
			w.WriteHeader(statusCode)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	return srv.URL + "/", func() bool {
		return revokeCalled
	}
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
	host, revokeCalled := setupLogoutEndpoint(t, http.StatusNoContent)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: host,
	}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "Successfully logged out")
	require.Contains(t, out, "render login")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.True(t, revokeCalled(), "logout should call the revoke endpoint")
}

func TestLogoutWarnsWithoutSuccessWhenTokenRevocationFails(t *testing.T) {
	configPath := setupLogoutTest(t)
	host, revokeCalled := setupLogoutEndpoint(t, http.StatusInternalServerError)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: host,
	}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "Warning: something went wrong revoking your CLI token")
	require.NotContains(t, out, "Successfully logged out")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.True(t, revokeCalled(), "logout should still call the revoke endpoint")
}

func TestLogoutInteractiveShowsSpinner(t *testing.T) {
	configPath := setupLogoutTest(t)
	host, _ := setupLogoutEndpoint(t, http.StatusNoContent, 75*time.Millisecond)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: host,
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
	host, revokeCalled := setupLogoutEndpoint(t, http.StatusNoContent)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_test_revoke",
		Host: host,
	}))

	output := command.TEXT
	ctx := command.SetFormatInContext(context.Background(), &output)
	out, stderr, err := runLogoutWithContext(t, ctx)
	require.NoError(t, err)
	require.NotContains(t, stderr, "Logging out")
	require.Contains(t, out, "Successfully logged out")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.True(t, revokeCalled(), "logout should still call the revoke endpoint")
}

func TestLogoutBothEnvAndOAuth(t *testing.T) {
	configPath := setupLogoutTest(t)
	t.Setenv("RENDER_API_KEY", "rnd_env_token")
	host, revokeCalled := setupLogoutEndpoint(t, http.StatusNoContent)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_oauth",
		Host: host,
	}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "OAuth credentials cleared")
	require.Contains(t, out, "RENDER_API_KEY")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.True(t, revokeCalled(), "logout should call the revoke endpoint")
	require.Equal(t, "rnd_env_token", os.Getenv("RENDER_API_KEY"), "logout should not modify RENDER_API_KEY")
}

func TestLogoutWarnsWithEnvKeyNoteWhenTokenRevocationFails(t *testing.T) {
	configPath := setupLogoutTest(t)
	t.Setenv("RENDER_API_KEY", "rnd_env_token")
	host, revokeCalled := setupLogoutEndpoint(t, http.StatusInternalServerError)

	require.NoError(t, config.SetAPIConfig(config.APIConfig{
		Key:  "rnd_oauth",
		Host: host,
	}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "Warning: something went wrong revoking your CLI token")
	require.Contains(t, out, "Note: RENDER_API_KEY is still set in your environment.")
	require.NotContains(t, out, "OAuth credentials cleared")
	require.NotContains(t, out, "Successfully logged out")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.True(t, revokeCalled(), "logout should still call the revoke endpoint")
	require.Equal(t, "rnd_env_token", os.Getenv("RENDER_API_KEY"), "logout should not modify RENDER_API_KEY")
}
