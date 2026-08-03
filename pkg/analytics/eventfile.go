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
// detail of Sender's eventual subprocess handoff.
func writeEventFile(payload client.CreateCliTelemetryEventJSONRequestBody) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("serialize analytics event: %w", err)
	}

	stateDir, err := config.StateDir()
	if err != nil {
		return "", fmt.Errorf("resolve analytics state directory: %w", err)
	}
	path := filepath.Join(
		stateDir,
		analyticsDirectoryName,
		eventsDirectoryName,
		eventFilePrefix+uuid.NewString()+eventFileSuffix,
	)
	if err := files.Write(path, data); err != nil {
		return "", fmt.Errorf("write analytics event file: %w", err)
	}
	return path, nil
}
