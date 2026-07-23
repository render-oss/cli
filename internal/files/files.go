// Package files provides shared filesystem operations.
package files

import (
	"fmt"
	"os"
	"path/filepath"
)

type (
	directoryMode = os.FileMode
	fileMode      = os.FileMode
)

const (
	// ownerOnlyFile allows only the owner to read and write the file.
	ownerOnlyFile fileMode = 0o600
	// ownerWritableDirectory allows everyone to read and traverse the directory,
	// while only the owner can modify it.
	ownerWritableDirectory directoryMode = 0o755
)

// Write replaces path with a newly created file containing data rather than
// modifying an existing file in place. Concurrent writes cannot interleave,
// and the last successful replacement wins. Missing parent directories are
// created, and the resulting file uses owner-only read/write permissions where
// supported.
//
// To keep partially written data out of path, Write writes to a temporary file
// in the same directory, closes it, then renames it over path. Temporary files
// are removed on a best-effort basis if any step fails.
func Write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), ownerWritableDirectory); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}

	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()

	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Chmod(ownerOnlyFile); err != nil {
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}

	return nil
}
