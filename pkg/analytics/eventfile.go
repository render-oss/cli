package analytics

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/render-oss/cli/internal/files"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
)

const (
	analyticsDirectoryName = "analytics"
	eventsDirectoryName    = "events"
	eventFilePrefix        = "event-"
	eventFileSuffix        = ".json"
)

// writeEventFile serializes one analytics event to a unique state file and
// closes it before returning the path. Event files are an implementation
// detail of Sender's subprocess handoff.
func writeEventFile(payload client.CreateCliTelemetryEventJSONRequestBody) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("serialize analytics event: %w", err)
	}

	eventsDir, err := eventsDirectory()
	if err != nil {
		return "", fmt.Errorf("resolve analytics event directory: %w", err)
	}
	path := filepath.Join(eventsDir, eventFilePrefix+uuid.NewString()+eventFileSuffix)
	if err := files.Write(path, data); err != nil {
		return "", fmt.Errorf("write analytics event file: %w", err)
	}
	return path, nil
}

// analyticsDirectory returns the absolute path to the directory that holds all
// analytics state.
func analyticsDirectory() (string, error) {
	stateDir, err := config.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Abs(filepath.Join(stateDir, analyticsDirectoryName))
}

// eventsDirectory returns the absolute directory that holds analytics event
// files. Both the writer and SendFile's path validation must derive the
// directory from here: a relative RENDER_CLI_CONFIG_DIR otherwise yields paths
// that write fine but fail validation, orphaning every event file.
func eventsDirectory() (string, error) {
	analyticsDir, err := analyticsDirectory()
	if err != nil {
		return "", err
	}
	return filepath.Join(analyticsDir, eventsDirectoryName), nil
}
