package command_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Run("expands ~/path", func(t *testing.T) {
		result, err := command.ExpandPath("~/Downloads/file.gif")
		require.NoError(t, err)
		require.Equal(t, filepath.Join(home, "Downloads/file.gif"), result)
	})

	t.Run("expands bare ~", func(t *testing.T) {
		result, err := command.ExpandPath("~")
		require.NoError(t, err)
		require.Equal(t, home, result)
	})

	t.Run("leaves absolute path unchanged", func(t *testing.T) {
		result, err := command.ExpandPath("/usr/local/bin/file")
		require.NoError(t, err)
		require.Equal(t, "/usr/local/bin/file", result)
	})

	t.Run("leaves relative path unchanged", func(t *testing.T) {
		result, err := command.ExpandPath("relative/path/file.txt")
		require.NoError(t, err)
		require.Equal(t, "relative/path/file.txt", result)
	})

	t.Run("leaves empty string unchanged", func(t *testing.T) {
		result, err := command.ExpandPath("")
		require.NoError(t, err)
		require.Equal(t, "", result)
	})

	t.Run("leaves tilde in middle of path unchanged", func(t *testing.T) {
		result, err := command.ExpandPath("/path/~/file")
		require.NoError(t, err)
		require.Equal(t, "/path/~/file", result)
	})
}
