package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigDir(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Run("defaults to ~/.render", func(t *testing.T) {
		t.Setenv("RENDER_CLI_CONFIG_DIR", "")

		dir, err := ConfigDir()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(home, ".render"), dir)
	})

	t.Run("honors RENDER_CLI_CONFIG_DIR", func(t *testing.T) {
		override := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", override)

		dir, err := ConfigDir()
		require.NoError(t, err)
		require.Equal(t, override, dir)
	})

	t.Run("expands tilde", func(t *testing.T) {
		t.Setenv("RENDER_CLI_CONFIG_DIR", "~/custom-render")

		dir, err := ConfigDir()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(home, "custom-render"), dir)
	})
}

func TestStateDirIsInsideConfigDir(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)

	dir, err := StateDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(configDir, "state"), dir)

	_, err = os.Stat(dir)
	require.True(t, os.IsNotExist(err), "StateDir should not create the directory")
}

func TestStateDirIgnoresLegacyConfigPathOverride(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", "")
	t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))

	home, err := os.UserHomeDir()
	require.NoError(t, err)

	dir, err := StateDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".render", "state"), dir)
}

func TestConfigPathUsesConfigDir(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	t.Setenv("RENDER_CLI_CONFIG_PATH", "")

	cfg := &Config{Version: 1, APIConfig: APIConfig{Key: "rnd_test"}}
	require.NoError(t, cfg.Persist())

	require.FileExists(t, filepath.Join(configDir, "cli.yaml"))
}

func TestLegacyConfigPathWinsOverConfigDir(t *testing.T) {
	configDir := t.TempDir()
	legacyPath := filepath.Join(t.TempDir(), "legacy.yaml")
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	t.Setenv("RENDER_CLI_CONFIG_PATH", legacyPath)

	cfg := &Config{Version: 1, APIConfig: APIConfig{Key: "rnd_test"}}
	require.NoError(t, cfg.Persist())

	require.FileExists(t, legacyPath)
	require.NoFileExists(t, filepath.Join(configDir, "cli.yaml"))
}
