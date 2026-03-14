package scaffold

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SetupCommand returns the platform-appropriate environment setup command
// for the given language. Returns an empty string if no setup is needed.
//
// Templates can use the {{setupCommand}} placeholder in their nextSteps
// to include this in the displayed instructions.
func SetupCommand(lang Language) string {
	switch lang {
	case Python:
		return pythonSetupCommand()
	default:
		return ""
	}
}

// pythonSetupCommand returns the venv creation + activation command
// tailored to the user's OS and shell.
func pythonSetupCommand() string {
	python := pythonBinary()
	activate := pythonActivateCommand()
	return python + " -m venv .venv && " + activate
}

// pythonBinary returns the best available Python binary name.
// Prefers "python3" but falls back to "python" if python3 is not on PATH.
func pythonBinary() string {
	if runtime.GOOS == "windows" {
		// Windows Python installer registers as "python" by default
		if hasCommand("python3") {
			return "python3"
		}
		return "python"
	}
	if hasCommand("python3") {
		return "python3"
	}
	return "python"
}

// hasCommand reports whether the named command is available on PATH.
func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// DepsInstallCommand returns the command to run for installing dependencies.
// For Python, it creates a venv and uses the venv's own pip to avoid
// PEP 668 "externally managed environment" errors and pip/pip3 ambiguity.
// For other languages, it returns the build command unchanged.
func DepsInstallCommand(lang Language, buildCmd string) string {
	if lang != Python {
		return buildCmd
	}

	python := pythonBinary()
	activate := pythonActivateCommand()

	// Normalize pip3 → pip since activation puts the venv's pip on PATH
	cmd := buildCmd
	cmd = replaceBareCommand(cmd, "pip3", "pip")

	return python + " -m venv .venv && " + activate + " && " + cmd
}

// LocalBuildCommand rewrites a template's build command for the local
// environment. For Python, replaces pip/pip3 with the venv's pip binary
// so the command works without manual venv activation.
// For other languages, returns unchanged.
func LocalBuildCommand(lang Language, buildCmd string) string {
	if lang != Python {
		return buildCmd
	}
	venvPip := venvBinPath("pip")
	cmd := replaceBareCommand(buildCmd, "pip3", venvPip)
	cmd = replaceBareCommand(cmd, "pip", venvPip)
	return cmd
}

// LocalStartCommand rewrites a template's start command for the local
// environment. For Python, replaces python/python3 with the venv's python
// binary so the command works without manual venv activation.
// For other languages, returns unchanged.
func LocalStartCommand(lang Language, startCmd string) string {
	if lang != Python {
		return startCmd
	}
	venvPython := venvBinPath("python")
	cmd := replaceBareCommand(startCmd, "python3", venvPython)
	cmd = replaceBareCommand(cmd, "python", venvPython)
	return cmd
}

// venvBinPath returns the path to a binary inside the .venv directory.
func venvBinPath(name string) string {
	if runtime.GOOS == "windows" {
		return `.venv\Scripts\` + name
	}
	return ".venv/bin/" + name
}

// replaceBareCommand replaces a bare command name with a replacement,
// matching at the start of the string and after "&&" in chained commands.
func replaceBareCommand(cmd string, name string, replacement string) string {
	if cmd == name {
		cmd = replacement
	} else if strings.HasPrefix(cmd, name+" ") {
		cmd = replacement + cmd[len(name):]
	}

	old := "&& " + name + " "
	new := "&& " + replacement + " "
	cmd = strings.ReplaceAll(cmd, old, new)

	return cmd
}

// pythonActivateCommand returns the venv activation command for the
// user's current shell and OS.
func pythonActivateCommand() string {
	switch runtime.GOOS {
	case "windows":
		// Check for PowerShell via PSModulePath (set in all PowerShell sessions)
		if os.Getenv("PSModulePath") != "" {
			return `.venv\Scripts\Activate.ps1`
		}
		return `.venv\Scripts\activate.bat`
	default:
		shell := os.Getenv("SHELL")
		if strings.Contains(shell, "fish") {
			return "source .venv/bin/activate.fish"
		}
		// bash, zsh, and most other POSIX shells
		return "source .venv/bin/activate"
	}
}
