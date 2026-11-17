package updatenotice

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/blang/semver/v4"

	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/pkg/config"
)

const stateFileName = "update-check.json"

// state is the most recent release lookup and when the notice was last shown,
// persisted between invocations.
type state struct {
	// LatestVersion is the latest release seen, or empty if no lookup has
	// succeeded yet.
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
	// PublishedAt is when LatestVersion was released, or zero if unknown.
	PublishedAt time.Time `json:"published_at,omitzero"`
	// NotifiedAt is when the notice was last shown, or zero if never.
	NotifiedAt time.Time `json:"notified_at,omitzero"`
}

// latest parses LatestVersion, returning nil when it is empty or invalid.
func (s state) latest() *semver.Version {
	v, err := semver.Parse(s.LatestVersion)
	if err != nil {
		return nil
	}
	return &v
}

// statePath resolves the state file under [config.StateDir].
func statePath() (string, error) {
	dir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, stateFileName), nil
}

// readState returns the persisted lookup, or nil if there is none.
func readState() (*state, error) {
	path, err := statePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// updateState applies change to the saved state and saves the result, which it
// returns. It reads the state again first, so that fields another command saved
// in the meantime survive; fallback is used when nothing readable is saved.
// Saving is best-effort: a failed write only means the next command looks the
// release up again.
//
// Two commands can still interleave between the read and the write. The read
// keeps that window to microseconds, and the cost of losing it is one repeated
// notice or lookup, which does not justify a cross-platform file lock.
func updateState(fallback state, change func(*state)) state {
	next := fallback
	if saved, err := readState(); err == nil {
		next = *saved
	}
	change(&next)
	_ = writeState(next)
	return next
}

// writeState atomically persists s.
func writeState(s state) error {
	path, err := statePath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return files.Write(path, data)
}
