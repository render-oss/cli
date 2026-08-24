package analyticsnotice

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/pkg/config"
)

// markerFileName names the empty marker whose presence records that the
// first-run notice has been shown.
const markerFileName = "welcome-notice-shown"

// markerPath resolves the marker's path under [config.StateDir].
func markerPath() (string, error) {
	dir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, markerFileName), nil
}

// markerExists reports whether the marker exists.
func markerExists() (bool, error) {
	path, err := markerPath()
	if err != nil {
		return false, err
	}

	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// writeMarker atomically creates the empty marker. Callers treat the returned
// error as best-effort and discard it: a failed write only means the notice may
// show again on a later run.
func writeMarker() error {
	path, err := markerPath()
	if err != nil {
		return err
	}
	return files.Write(path, nil)
}
