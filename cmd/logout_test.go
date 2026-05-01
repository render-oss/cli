package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

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

// runLogout executes the logout command and collects output into a buffer that is returned to the caller
func runLogout(t *testing.T) (string, error) {
	t.Helper()
	cmd := newLogoutCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	err := cmd.RunE(cmd, nil)
	return buf.String(), err
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
	require.NoError(t, config.SetAPIConfig(config.APIConfig{Key: "rnd_test"}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "Successfully logged out")
	require.Contains(t, out, "render login")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
}

func TestLogoutBothEnvAndOAuth(t *testing.T) {
	configPath := setupLogoutTest(t)
	t.Setenv("RENDER_API_KEY", "rnd_env_token")
	require.NoError(t, config.SetAPIConfig(config.APIConfig{Key: "rnd_oauth"}))

	out, err := runLogout(t)
	require.NoError(t, err)
	require.Contains(t, out, "OAuth credentials cleared")
	require.Contains(t, out, "RENDER_API_KEY")

	_, statErr := os.Stat(configPath)
	require.True(t, os.IsNotExist(statErr), "config file should be deleted after logout")
	require.Equal(t, "rnd_env_token", os.Getenv("RENDER_API_KEY"), "logout should not modify RENDER_API_KEY")
}
