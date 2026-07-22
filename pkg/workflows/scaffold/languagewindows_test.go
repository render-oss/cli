//go:build windows

package scaffold

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupCommand_Python(t *testing.T) {
	t.Run("PowerShell with python3", func(t *testing.T) {
		setPython3Available(t)
		t.Setenv("PSModulePath", `C:\Program Files\PowerShell\Modules`)

		assert.Equal(t, `python3 -m venv .venv && .venv\Scripts\Activate.ps1`, SetupCommand(Python))
	})

	t.Run("Command Prompt with python fallback", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		t.Setenv("PSModulePath", "")

		assert.Equal(t, `python -m venv .venv && .venv\Scripts\activate.bat`, SetupCommand(Python))
	})
}

func setPython3Available(t *testing.T) {
	t.Helper()

	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "python3.exe"), nil, 0o755))
	t.Setenv("PATH", binDir)
	t.Setenv("PATHEXT", ".EXE")
}
