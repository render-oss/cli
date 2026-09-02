package analyticsnotice

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

// setupNoticeTest configures the CLI to use a temp config directory and returns
// the notice marker's path inside it, so ShowIfNeeded never touches the
// machine's real ~/.render.
func setupNoticeTest(t *testing.T) (outMarkerPath string) {
	t.Helper()

	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	outMarkerPath, err := markerPath()
	require.NoError(t, err)

	return outMarkerPath
}

func setupMarkerReadFailure(t *testing.T) {
	t.Helper()

	configDir := t.TempDir()
	if runtime.GOOS == "windows" {
		// A wildcard makes the marker path invalid instead of merely absent.
		configDir = filepath.Join(configDir, "invalid?")
	} else {
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "state"), nil, 0o600))
	}
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)

	path, err := markerPath()
	require.NoError(t, err)
	_, err = os.Stat(path)
	require.Error(t, err, "the fixture must make the marker unreadable")
	require.NotErrorIs(t, err, os.ErrNotExist, "the fixture must cause a marker read failure, not an absent marker")
}

func TestCanSend(t *testing.T) {
	testCases := []struct {
		name          string
		ci            bool
		markerPresent bool
		want          bool
	}{
		{name: "non-CI without marker"},
		{name: "non-CI with marker", markerPresent: true, want: true},
		{name: "CI without marker", ci: true, want: true},
		{name: "CI with marker", ci: true, markerPresent: true, want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupNoticeTest(t)
			if tc.markerPresent {
				require.NoError(t, writeMarker())
			}

			require.Equal(t, tc.want, CanSend(Conditions{CI: tc.ci}))
		})
	}

	t.Run("marker read failure suppresses sending", func(t *testing.T) {
		setupMarkerReadFailure(t)

		require.False(t, CanSend(Conditions{}))
	})
}

func TestShowIfNeeded(t *testing.T) {
	t.Run("prints the notice and records it as shown", func(t *testing.T) {
		markerPath := setupNoticeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))

		out := ansi.Strip(buf.String())
		require.Contains(t, out, "The Render CLI collects usage data")

		info, err := os.Stat(markerPath)
		require.NoError(t, err)
		require.Zero(t, info.Size())
	})

	t.Run("shows the opted-out confirmation when a mechanism is in effect", func(t *testing.T) {
		setupNoticeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonDoNotTrack))

		out := ansi.Strip(buf.String())
		require.Contains(t, out, "Telemetry is currently disabled")
		require.NotContains(t, out, "To opt out,")
	})

	t.Run("a second run shows nothing once the marker exists", func(t *testing.T) {
		setupNoticeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))
		buf.Reset()

		require.False(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())
	})

	t.Run("a failed notice write does not record it as shown", func(t *testing.T) {
		markerPath := setupNoticeTest(t)

		require.True(t, ShowIfNeeded(command.NewStream(failingWriter{}), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))

		_, err := os.Stat(markerPath)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("CI permits analytics without printing or writing a marker", func(t *testing.T) {
		markerPath := setupNoticeTest(t)
		var buf bytes.Buffer

		require.False(t, ShowIfNeeded(command.NewStream(&buf), Conditions{CI: true}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())

		_, err := os.Stat(markerPath)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("piped stderr does not print or write a marker", func(t *testing.T) {
		markerPath := setupNoticeTest(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())

		_, err := os.Stat(markerPath)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("a marker read failure skips analytics", func(t *testing.T) {
		setupMarkerReadFailure(t)
		var buf bytes.Buffer

		require.True(t, ShowIfNeeded(command.NewStream(&buf), Conditions{StderrTTY: true}, analytics.OptOutReasonNone))
		require.Empty(t, buf.String())
	})
}
