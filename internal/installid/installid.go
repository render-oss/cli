// Package installid manages the stable identifier for a Render CLI installation.
// i.e., the identifier should be unique to the machine (not the user)
// The file format is a simple text file containing only a v4 UUID
// This implementation is tolerant to whitespace around the UUID and empty lines,
// but does not tolerate any other unexpected symbols
package installid

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/pkg/config"
)

const fileName = "installation-id.txt"

// Resolve returns the stable installation UUID, creating and persisting one
// when it is missing or invalid. It returns ("", err) when it cannot obtain a
// persisted, reusable UUID. Callers must treat failures as best-effort and
// never allow them to affect command output or exit status.
func Resolve() (string, error) {
	path, err := path()
	if err != nil {
		return "", err
	}

	return ensureID(path)
}

// ensureID returns the installation ID - a UUID string
//
// If a valid installation ID file already exists, the contents are simply returned
// If the file does not exist, an ID is generated, written to disk, and returned
// Finally, if the file does not contain a valid UUID, an ID is re-generated and re-written to disk
func ensureID(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err == nil {
		id, parseErr := uuid.Parse(strings.TrimSpace(string(contents)))
		if parseErr == nil && id.Version() == uuid.Version(4) && id.Variant() == uuid.RFC4122 {
			return id.String(), nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	id := uuid.NewString()
	if err := files.Write(path, []byte(id)); err != nil {
		return "", err
	}

	return id, nil
}

// path returns the path to the file containing the installation ID
func path() (string, error) {
	stateDir, err := config.StateDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(stateDir, fileName), nil
}
