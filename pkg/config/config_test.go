package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// tmpConfigPath creates a temp dir, points RENDER_CLI_CONFIG_PATH at path ./cli.yaml inside it,
// and returns the path. This isolates tests from ~/.render/cli.yaml.
// Does __not__ create the cli.yml
func tmpConfigPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.yaml")
	t.Setenv(configPathEnvKey, path)
	return path
}

func TestDeleteConfig_DeletesFile(t *testing.T) {
	path := tmpConfigPath(t)

	// Write a config file with an OAuth key.
	cfg := &Config{Version: 1, APIConfig: APIConfig{Key: "rnd_test"}}
	require.NoError(t, cfg.Persist())
	_, err := os.Stat(path)
	require.NoError(t, err, "config file should exist before clear")

	require.NoError(t, DeleteConfig())

	_, err = os.Stat(path)
	require.True(t, os.IsNotExist(err), "config file should be deleted after clear")
}

func TestDeleteConfig_NoopWhenMissing(t *testing.T) {
	tmpConfigPath(t) // sets env var to a path that doesn't exist
	require.NoError(t, DeleteConfig())
}

func TestLoad_ReturnsFreshConfigWhenMissing(t *testing.T) {
	path := tmpConfigPath(t) // sets env var to a path that doesn't exist

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, currentVersion, cfg.Version)
	require.Empty(t, cfg.Key)
	require.Empty(t, cfg.Workspace)

	_, statErr := os.Stat(path)
	require.True(t, os.IsNotExist(statErr), "Load should not create the config file")
}

func TestHasOAuthConfig_TrueWhenKeySet(t *testing.T) {
	tmpConfigPath(t)

	cfg := &Config{Version: 1, APIConfig: APIConfig{Key: "rnd_test"}}
	require.NoError(t, cfg.Persist())

	has, err := HasOAuthConfig()
	require.NoError(t, err)
	require.True(t, has)
}

func TestHasOAuthConfig_FalseWhenNoFile(t *testing.T) {
	tmpConfigPath(t)

	has, err := HasOAuthConfig()
	require.NoError(t, err)
	require.False(t, has)
}

func TestHasOAuthConfig_FalseWhenFileHasNoKey(t *testing.T) {
	tmpConfigPath(t)

	cfg := &Config{Version: 1}
	require.NoError(t, cfg.Persist())

	has, err := HasOAuthConfig()
	require.NoError(t, err)
	require.False(t, has)
}
