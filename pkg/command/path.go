package command

import (
	"os"
	"path/filepath"
)

// ExpandPath expands a leading ~ in a file path to the user's home directory.
// This is needed because the shell does not expand ~ when it appears inside
// flag values (e.g., --file=~/path).
func ExpandPath(path string) (string, error) {
	if path == "~" || len(path) >= 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[1:])
	}
	return path, nil
}
