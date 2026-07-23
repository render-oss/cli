package config

import (
	"os"
	"path/filepath"
)

// ConfigDir returns the directory where the CLI keeps its files: the config
// file (cli.yaml) and persistent state (see StateDir). It defaults to
// ~/.render and can be overridden with RENDER_CLI_CONFIG_DIR. ConfigDir does
// not create the directory.
func ConfigDir() (string, error) {
	if dir := os.Getenv(configDirEnvKey); dir != "" {
		return expandPath(dir)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".render"), nil
}

// StateDir returns the directory for persistent, non-portable state: data
// that should survive CLI invocations and logout, but that a user would not
// copy to another machine. It is always <ConfigDir>/state.
//
// The legacy RENDER_CLI_CONFIG_PATH override does not move
// state: it points at the config file itself, and the CLI must not write
// siblings next to an arbitrary user-chosen file. Use RENDER_CLI_CONFIG_DIR
// to relocate state along with the rest of the CLI's files.
//
// StateDir does not create the directory.
func StateDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state"), nil
}
