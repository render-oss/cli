package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// ExpandHome expands a leading "~" or "~/..." to the current user's home
// directory. "~user/..." is returned unchanged because we don't resolve
// arbitrary users. Empty input is returned unchanged.
func ExpandHome(path string) (string, error) {
	if path == "" || path[0] != '~' {
		return path, nil
	}
	if path != "~" && len(path) > 1 && path[1] != '/' && path[1] != '\\' {
		return path, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
