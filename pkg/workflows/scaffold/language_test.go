package scaffold

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupCommand_Python(t *testing.T) {
	cmd := SetupCommand(Python)
	assert.NotEmpty(t, cmd)
	assert.Contains(t, cmd, "venv")
	assert.Contains(t, cmd, "activate")

	if runtime.GOOS == "windows" {
		assert.Contains(t, cmd, "python -m venv")
		assert.Contains(t, cmd, `Scripts`)
	} else {
		assert.Contains(t, cmd, "python3 -m venv")
		assert.Contains(t, cmd, "source")
	}
}

func TestDepsInstallCommand_Python(t *testing.T) {
	python := pythonBinary()
	activate := pythonActivateCommand()
	prefix := python + " -m venv .venv && " + activate + " && "

	t.Run("creates venv, activates, and runs pip", func(t *testing.T) {
		result := DepsInstallCommand(Python, "pip install -r requirements.txt")
		assert.Equal(t, prefix+"pip install -r requirements.txt", result)
	})

	t.Run("normalizes pip3 to pip", func(t *testing.T) {
		result := DepsInstallCommand(Python, "pip3 install -r requirements.txt")
		assert.Equal(t, prefix+"pip install -r requirements.txt", result)
	})

	t.Run("handles chained pip commands", func(t *testing.T) {
		result := DepsInstallCommand(Python, "pip install -r requirements.txt && pip install -e .")
		assert.Equal(t, prefix+"pip install -r requirements.txt && pip install -e .", result)
	})

	t.Run("prepends venv and activate for non-pip commands", func(t *testing.T) {
		result := DepsInstallCommand(Python, "make build")
		assert.Equal(t, prefix+"make build", result)
	})
}

func TestLocalBuildCommand_Python(t *testing.T) {
	venvPip := venvBinPath("pip")
	assert.Equal(t, venvPip+" install -r requirements.txt", LocalBuildCommand(Python, "pip install -r requirements.txt"))
	assert.Equal(t, venvPip+" install -r requirements.txt", LocalBuildCommand(Python, "pip3 install -r requirements.txt"))
}

func TestLocalStartCommand_Python(t *testing.T) {
	venvPython := venvBinPath("python")
	assert.Equal(t, venvPython+" main.py", LocalStartCommand(Python, "python main.py"))
	assert.Equal(t, venvPython+" main.py", LocalStartCommand(Python, "python3 main.py"))
	assert.Equal(t, "render-workflows main:app", LocalStartCommand(Python, "render-workflows main:app"))
}
