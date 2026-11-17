package updatenotice

import (
	"os"
	"path/filepath"
	"strings"
)

// Install is how the running binary was installed, as far as its location
// tells. Only package managers that put the binary in a recognizable place
// are detected; the install script and direct downloads leave a binary in an
// ordinary directory and stay [InstallUnknown].
type Install int

const (
	InstallUnknown Install = iota
	InstallHomebrew
	InstallWinGet
)

// executablePath is replaced in tests.
var executablePath = os.Executable

// DetectInstall reports how the running binary was installed.
func DetectInstall() Install {
	exe, err := executablePath()
	if err != nil {
		return InstallUnknown
	}
	// Homebrew and WinGet link a command name to the binary they manage.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return installFromPath(exe)
}

// installFromPath classifies a resolved binary path.
func installFromPath(path string) Install {
	p := strings.ToLower(strings.ReplaceAll(path, `\`, "/"))
	switch {
	case strings.Contains(p, "/cellar/render/"):
		return InstallHomebrew
	case strings.Contains(p, "/microsoft/winget/packages/render.cli"):
		return InstallWinGet
	default:
		return InstallUnknown
	}
}

// upgradeCommand returns the command that upgrades this install, or "" when
// the install method is unknown.
func (i Install) upgradeCommand() string {
	switch i {
	case InstallHomebrew:
		return "brew upgrade render"
	case InstallWinGet:
		return "winget upgrade Render.CLI"
	default:
		return ""
	}
}
