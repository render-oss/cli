package welcome

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/command"
)

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

// setupWelcomeTest configures the CLI to use a temp config directory and returns
// the welcome marker's path inside it, so ShowIfNeeded never touches the
// machine's real ~/.render.
func setupWelcomeTest(t *testing.T) (outMarkerPath string) {
	t.Helper()

	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	outMarkerPath, err := markerPath()
	require.NoError(t, err)

	return outMarkerPath
}

func TestShowIfNeeded(t *testing.T) {
	t.Run("prints the notice and records it as shown", func(t *testing.T) {
		markerPath := setupWelcomeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))

		out := ansi.Strip(buf.String())
		require.Contains(t, out, "Welcome to the Render CLI!")
		require.Contains(t, out, "The Render CLI collects usage data")

		info, err := os.Stat(markerPath)
		require.NoError(t, err)
		require.Zero(t, info.Size())
	})

	t.Run("shows the opted-out confirmation when a mechanism is in effect", func(t *testing.T) {
		setupWelcomeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonDoNotTrack))

		out := ansi.Strip(buf.String())
		require.Contains(t, out, "Telemetry is disabled due to your DO_NOT_TRACK setting.")
		require.NotContains(t, out, "Opt out:")
	})

	t.Run("a second run shows nothing once the marker exists", func(t *testing.T) {
		setupWelcomeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))
		buf.Reset()

		require.False(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())
	})

	t.Run("a failed notice write does not record it as shown", func(t *testing.T) {
		markerPath := setupWelcomeTest(t)

		require.True(t, ShowIfNeeded(command.NewStream(failingWriter{}), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))

		_, err := os.Stat(markerPath)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("CI permits analytics without printing or writing a marker", func(t *testing.T) {
		markerPath := setupWelcomeTest(t)
		var buf bytes.Buffer

		require.False(t, ShowIfNeeded(command.NewStream(&buf), Conditions{CI: true}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())

		_, err := os.Stat(markerPath)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("piped stderr does not print or write a marker", func(t *testing.T) {
		markerPath := setupWelcomeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())

		_, err := os.Stat(markerPath)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("a marker read failure skips analytics", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Windows reports a non-directory path component as os.ErrNotExist")
		}

		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "state"), nil, 0o600), "Must setup an invalid state directory to force marker read failure")
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())
	})
}
