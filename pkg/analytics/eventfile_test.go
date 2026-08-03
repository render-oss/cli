package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
)

func TestWriteEventFile(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	payload := client.CreateCliTelemetryEventJSONRequestBody{
		Command:        "render services list",
		CompletionKind: telemetryclient.Success,
		ExitCode:       0,
	}

	first, err := writeEventFile(payload)
	require.NoError(t, err)
	second, err := writeEventFile(payload)
	require.NoError(t, err)

	wantDir := filepath.Join(configDir, "state", "analytics", "events")
	require.Equal(t, wantDir, filepath.Dir(first))
	require.Equal(t, wantDir, filepath.Dir(second))
	require.NotEqual(t, first, second)

	// Windows synthesizes Unix permission bits rather than preserving the
	// modes passed to MkdirAll and Chmod. The remaining assertions still cover
	// the writer's behavior on Windows.
	assertFileModes := runtime.GOOS != "windows"
	for _, dir := range []string{
		filepath.Join(configDir, "state"),
		filepath.Join(configDir, "state", "analytics"),
		wantDir,
	} {
		info, statErr := os.Stat(dir)
		require.NoError(t, statErr)
		if assertFileModes {
			require.Equal(t, os.FileMode(0o755), info.Mode().Perm())
		}
	}

	for _, path := range []string{first, second} {
		info, statErr := os.Stat(path)
		require.NoError(t, statErr)
		require.True(t, info.Mode().IsRegular())
		if assertFileModes {
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}

		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		var got client.CreateCliTelemetryEventJSONRequestBody
		require.NoError(t, json.Unmarshal(data, &got))
		require.Equal(t, payload, got)
	}
}
