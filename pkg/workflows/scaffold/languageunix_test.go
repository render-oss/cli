//go:build !windows

package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupCommand_Python(t *testing.T) {
	t.Run("POSIX shell with python3", func(t *testing.T) {
		setPython3Available(t)
		t.Setenv("SHELL", "/bin/bash")

		assert.Equal(t, "python3 -m venv .venv && source .venv/bin/activate", SetupCommand(Python))
	})

	t.Run("fish shell with python fallback", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		t.Setenv("SHELL", "/usr/bin/fish")

		assert.Equal(t, "python -m venv .venv && source .venv/bin/activate.fish", SetupCommand(Python))
	})
}

func setPython3Available(t *testing.T) {
	t.Helper()

	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "python3"), nil, 0o755))
	t.Setenv("PATH", binDir)
}
