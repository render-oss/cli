package updatenotice

import (
	"bytes"
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

// fakeClient is a GitHubReleaseClient that counts lookups. With block set, a
// lookup waits until its context is cancelled. Releases are published two days
// ago unless published is set.
type fakeClient struct {
	latest    string
	published time.Time
	err       error
	block     bool
	// during, if set, runs while the lookup is in flight.
	during func()
	calls  atomic.Int32
}

func (f *fakeClient) LatestRelease(ctx context.Context) (Release, error) {
	f.calls.Add(1)
	if f.block {
		<-ctx.Done()
		return Release{}, ctx.Err()
	}
	if f.during != nil {
		f.during()
	}
	if f.err != nil {
		return Release{}, f.err
	}
	published := f.published
	if published.IsZero() {
		published = time.Now().Add(-48 * time.Hour)
	}
	return Release{Version: semver.MustParse(f.latest), PublishedAt: published}, nil
}

var interactive = Conditions{Interactive: true}

func setupStateDir(t *testing.T) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("RENDER_CLI_DISABLE_UPDATE_NOTIFIER", "")
	setExecutable(t, "/usr/local/bin/render")
}

// setExecutable makes DetectInstall see the running binary at path.
func setExecutable(t *testing.T, path string) {
	t.Helper()
	original := executablePath
	executablePath = func() (string, error) { return path, nil }
	t.Cleanup(func() { executablePath = original })
}

func show(c *Check) (string, bool) {
	var buf bytes.Buffer
	shown := c.Show(command.NewStream(&buf))
	return ansi.Strip(buf.String()), shown
}

func savedState(t *testing.T) state {
	t.Helper()
	s, err := readState()
	require.NoError(t, err)
	return *s
}

func TestStart_IneligibleRunsNeverLookUp(t *testing.T) {
	testCases := []struct {
		name       string
		conditions Conditions
		version    string
		setup      func(t *testing.T)
	}{
		{name: "not interactive", conditions: Conditions{}, version: "1.0.0"},
		{
			name: "disabled", conditions: interactive, version: "1.0.0",
			setup: func(t *testing.T) { t.Setenv("RENDER_CLI_DISABLE_UPDATE_NOTIFIER", "1") },
		},
		{name: "built from source", conditions: interactive, version: "dev"},
		{
			name: "notified within the last day", conditions: interactive, version: "1.0.0",
			setup: func(t *testing.T) {
				require.NoError(t, writeState(state{NotifiedAt: time.Now().Add(-23 * time.Hour)}))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupStateDir(t)
			if tc.setup != nil {
				tc.setup(t)
			}
			client := &fakeClient{latest: "2.0.0"}

			check := Start(tc.conditions, client, tc.version)

			require.Nil(t, check)
			out, shown := show(check)
			require.False(t, shown)
			require.Empty(t, out)
			require.Zero(t, client.calls.Load())
		})
	}
}

func TestStart_ShowsNoticeWhenNewerReleaseExists(t *testing.T) {
	setupStateDir(t)
	client := &fakeClient{latest: "1.2.0"}

	out, shown := show(Start(interactive, client, "1.1.5"))

	require.True(t, shown)
	require.Contains(t, out, "Update available: v1.1.5 → v1.2.0\n")
	require.Contains(t, out, "To upgrade, see: https://render.com/docs/cli#1-install-or-upgrade\n")
	require.Contains(t, out, "Release notes: https://github.com/render-oss/cli/releases/tag/v1.2.0")
	require.EqualValues(t, 1, client.calls.Load())

	saved := savedState(t)
	require.Equal(t, "1.2.0", saved.LatestVersion)
	require.WithinDuration(t, time.Now(), saved.NotifiedAt, time.Minute)
}

func TestStart_ShowsAtMostOncePerDay(t *testing.T) {
	setupStateDir(t)
	client := &fakeClient{latest: "1.2.0"}

	_, shown := show(Start(interactive, client, "1.1.5"))
	require.True(t, shown)

	require.Nil(t, Start(interactive, client, "1.1.5"), "a second run the same day shows nothing")
	require.EqualValues(t, 1, client.calls.Load())
}

func TestStart_ShowsAgainAfterADay(t *testing.T) {
	setupStateDir(t)
	require.NoError(t, writeState(state{
		LatestVersion: "1.2.0",
		CheckedAt:     time.Now().Add(-time.Minute),
		NotifiedAt:    time.Now().Add(-25 * time.Hour),
	}))

	out, shown := show(Start(interactive, &fakeClient{latest: "1.2.0"}, "1.1.5"))

	require.True(t, shown)
	require.Contains(t, out, "v1.1.5 → v1.2.0")
}

func TestStart_ShowsNothingWhenUpToDate(t *testing.T) {
	setupStateDir(t)

	out, shown := show(Start(interactive, &fakeClient{latest: "1.1.5"}, "1.1.5"))

	require.False(t, shown)
	require.Empty(t, out)
	require.True(t, savedState(t).NotifiedAt.IsZero())
}

func TestStart_FreshStateSkipsLookup(t *testing.T) {
	setupStateDir(t)
	require.NoError(t, writeState(state{LatestVersion: "3.0.0", CheckedAt: time.Now().Add(-3 * time.Hour)}))
	client := &fakeClient{latest: "9.9.9"}

	out, shown := show(Start(interactive, client, "2.0.0"))

	require.True(t, shown)
	require.Contains(t, out, "v2.0.0 → v3.0.0")
	require.Zero(t, client.calls.Load())
}

func TestStart_StaleStateLooksUpAgain(t *testing.T) {
	setupStateDir(t)
	require.NoError(t, writeState(state{LatestVersion: "3.0.0", CheckedAt: time.Now().Add(-checkInterval - time.Minute)}))
	client := &fakeClient{latest: "3.1.0"}

	out, shown := show(Start(interactive, client, "2.0.0"))

	require.True(t, shown)
	require.Contains(t, out, "v2.0.0 → v3.1.0")
	require.EqualValues(t, 1, client.calls.Load())
}

func TestStart_FailedLookupKeepsPreviousResultAndBacksOff(t *testing.T) {
	setupStateDir(t)
	require.NoError(t, writeState(state{LatestVersion: "3.0.0", CheckedAt: time.Now().Add(-checkInterval - time.Minute)}))
	client := &fakeClient{err: errors.New("offline")}

	check := Start(interactive, client, "2.0.0")
	<-check.done
	require.Equal(t, "3.0.0", savedState(t).LatestVersion, "the last known release is kept")
	require.WithinDuration(t, time.Now(), savedState(t).CheckedAt, time.Minute, "the failed attempt is recorded")

	_, _ = show(Start(interactive, client, "2.0.0"))
	require.EqualValues(t, 1, client.calls.Load(), "a failed lookup is not retried until the interval passes")
}

func TestStart_FirstFailedLookupShowsNothing(t *testing.T) {
	setupStateDir(t)

	out, shown := show(Start(interactive, &fakeClient{err: errors.New("offline")}, "2.0.0"))

	require.False(t, shown)
	require.Empty(t, out)
}

func TestShow_DoesNotWaitLongForASlowLookup(t *testing.T) {
	setupStateDir(t)
	client := &fakeClient{block: true}

	startedAt := time.Now()
	out, shown := show(Start(interactive, client, "2.0.0"))

	require.False(t, shown)
	require.Empty(t, out)
	require.Less(t, time.Since(startedAt), exitGracePeriod+time.Second)

	_, err := readState()
	require.Error(t, err, "a cancelled lookup is not recorded, so the next run tries again")

	_, _ = show(Start(interactive, client, "2.0.0"))
	require.EqualValues(t, 2, client.calls.Load())
}

func TestHighlightChanged(t *testing.T) {
	mark := func(parts ...string) string { return "[" + parts[0] + "]" }

	testCases := []struct {
		current, latest, want string
	}{
		{current: "1.4.2", latest: "1.4.3", want: "1.4.[3]"},
		{current: "2.30.5", latest: "2.33.5", want: "2.[33].5"},
		{current: "2.31.0", latest: "2.33.0", want: "2.[33].0"},
		{current: "1.4.2", latest: "1.5.0", want: "1.[5].[0]"},
		{current: "1.4.2", latest: "2.0.0", want: "[2].[0].[0]"},
		{current: "1.4.2", latest: "1.4.2", want: "1.4.2"},
	}

	for _, tc := range testCases {
		t.Run(tc.current+"->"+tc.latest, func(t *testing.T) {
			require.Equal(t, tc.want, highlightChanged(tc.current, tc.latest, mark))
		})
	}
}

func TestStart_AgentsGetOneLine(t *testing.T) {
	setupStateDir(t)

	out, shown := show(Start(Conditions{Agent: true}, &fakeClient{latest: "1.2.0"}, "1.1.5"))

	require.True(t, shown)
	require.Equal(t, "Update available: v1.1.5 → v1.2.0. To upgrade, see: https://render.com/docs/cli#1-install-or-upgrade. "+
		"Release notes: https://github.com/render-oss/cli/releases/tag/v1.2.0\n", out)
}

func TestStart_HomebrewWaitsAfterARelease(t *testing.T) {
	setupStateDir(t)
	setExecutable(t, "/opt/homebrew/Cellar/render/2.30.5/bin/render")
	client := &fakeClient{latest: "1.2.0", published: time.Now().Add(-time.Hour)}

	out, shown := show(Start(interactive, client, "1.1.5"))
	require.False(t, shown, "homebrew-core may not have the release yet")
	require.Empty(t, out)
	require.True(t, savedState(t).NotifiedAt.IsZero(), "the notice is still due")

	require.NoError(t, writeState(state{LatestVersion: "1.2.0", CheckedAt: time.Now(), PublishedAt: time.Now().Add(-homebrewGracePeriod - time.Minute)}))
	out, shown = show(Start(interactive, client, "1.1.5"))
	require.True(t, shown)
	require.Contains(t, out, "To upgrade, run: brew upgrade render\n")
}

func TestDetectInstall(t *testing.T) {
	testCases := []struct {
		path string
		want Install
	}{
		{path: "/opt/homebrew/Cellar/render/2.34.0/bin/render", want: InstallHomebrew},
		{path: "/home/linuxbrew/.linuxbrew/Cellar/render/2.34.0/bin/render", want: InstallHomebrew},
		{path: `C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\Render.CLI_Microsoft.Winget.Source_8wekyb3d8bbwe\cli_v2.34.0.exe`, want: InstallWinGet},
		{path: "/Users/me/.local/bin/render", want: InstallUnknown},
		{path: "/usr/local/bin/render", want: InstallUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			setExecutable(t, tc.path)
			require.Equal(t, tc.want, DetectInstall())
		})
	}
}

func TestRender_UpgradeLineFollowsInstall(t *testing.T) {
	s := command.NewStream(&bytes.Buffer{})

	require.Contains(t, ansi.Strip(Render(s, "2.0.0", "2.1.0", InstallHomebrew)), "To upgrade, run: brew upgrade render\n")
	require.Contains(t, ansi.Strip(Render(s, "2.0.0", "2.1.0", InstallWinGet)), "To upgrade, run: winget upgrade Render.CLI\n")
	require.Contains(t, ansi.Strip(Render(s, "2.0.0", "2.1.0", InstallUnknown)), "To upgrade, see: https://render.com/docs/cli#1-install-or-upgrade\n")
}

func TestRenderStatus(t *testing.T) {
	s := command.NewStream(&bytes.Buffer{})

	require.Equal(t, "You are using the latest version", ansi.Strip(RenderStatus(s, "2.0.0", "", InstallUnknown, false)))
	require.Equal(t, "Update available: v2.0.0 → v2.1.0\nTo upgrade, run: brew upgrade render\n"+
		"Release notes: https://github.com/render-oss/cli/releases/tag/v2.1.0",
		ansi.Strip(RenderStatus(s, "2.0.0", "2.1.0", InstallHomebrew, false)))
	require.Equal(t, "Update available: v2.0.0 → v2.1.0. To upgrade, run: brew upgrade render. "+
		"Release notes: https://github.com/render-oss/cli/releases/tag/v2.1.0",
		RenderStatus(s, "2.0.0", "2.1.0", InstallHomebrew, true))
}

func TestStart_LookupKeepsANoticeAnotherCommandShowedMeanwhile(t *testing.T) {
	setupStateDir(t)
	client := &fakeClient{latest: "1.2.0", during: func() {
		require.NoError(t, writeState(state{NotifiedAt: time.Now()}))
	}}

	out, shown := show(Start(interactive, client, "1.1.5"))

	require.False(t, shown, "the other command already showed the notice")
	require.Empty(t, out)
	saved := savedState(t)
	require.Equal(t, "1.2.0", saved.LatestVersion)
	require.WithinDuration(t, time.Now(), saved.NotifiedAt, time.Minute)
}

func TestShow_KeepsALookupAnotherCommandSavedMeanwhile(t *testing.T) {
	setupStateDir(t)
	require.NoError(t, writeState(state{LatestVersion: "1.2.0", CheckedAt: time.Now().Add(-3 * time.Hour)}))
	check := Start(interactive, &fakeClient{}, "1.1.5")
	newer := state{LatestVersion: "1.3.0", CheckedAt: time.Now().Truncate(time.Second)}
	require.NoError(t, writeState(newer))

	_, shown := show(check)

	require.True(t, shown)
	saved := savedState(t)
	require.Equal(t, "1.3.0", saved.LatestVersion)
	require.True(t, newer.CheckedAt.Equal(saved.CheckedAt))
	require.WithinDuration(t, time.Now(), saved.NotifiedAt, time.Minute)
}
