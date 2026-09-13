package analytics

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	helperModeEnv = "RENDER_CLI_TEST_ANALYTICS_SUBPROCESS_HELPER"

	// helperModeSuccess exits successfully without doing any work.
	helperModeSuccess = "success"
	// helperModeReportEnvironment reports the analytics environment the child
	// inherited from its parent on stderr.
	helperModeReportEnvironment = "environment"
	// helperModeWaitOnGateFile blocks until its event file exists, so the parent
	// decides when the child finishes — or never creates it and leaves the child
	// to be killed.
	helperModeWaitOnGateFile = "gate"
	// helperModeStartDetached starts another helper in detached mode and exits.
	// The child inherits helperModeMarkAfterGate from this intermediate process.
	helperModeStartDetached = "start-detached"
	// helperModeMarkAfterGate waits for its event file, then writes a marker that
	// proves it remained alive after its launcher exited.
	helperModeMarkAfterGate = "mark-after-gate"
)

// helperGateTimeout bounds how long helperModeWaitOnGateFile blocks: long
// enough that a loaded machine does not fail a passing test, short enough that
// a regression fails quickly instead of hanging.
const helperGateTimeout = 2 * time.Second

func TestMain(m *testing.M) {
	// A child launched by newHelperLauncher re-executes this test binary and
	// re-enters here. It impersonates "render analytics send" rather than
	// running the package's tests. This mimics the trick used by the standard
	// library's os/exec tests; see https://github.com/golang/go/issues/79941 for
	// proposed first-party support.
	if mode := os.Getenv(helperModeEnv); mode != "" {
		os.Exit(runHelper(mode))
	}
	// Any launcher created through the test-only opt-in marks its child. Keep
	// that child from falling through to m.Run even if a future test forgets to
	// select a helper mode.
	if IsSendSubprocess() {
		os.Exit(0)
	}

	os.Exit(m.Run())
}

func TestNewSubprocessLauncherUsesTheCurrentExecutable(t *testing.T) {
	t.Setenv(AllowSubprocessInTestsEnv, "1")
	t.Setenv(analyticsSubprocessEnv, "")

	launcher, err := newSubprocessLauncher()
	require.NoError(t, err)
	executable, err := os.Executable()
	require.NoError(t, err)

	require.Equal(t, executable, launcher.executable)
}

func TestOptedInSubprocessExitsThroughTestMain(t *testing.T) {
	t.Setenv(AllowSubprocessInTestsEnv, "1")
	t.Setenv(analyticsSubprocessEnv, "")

	launcher, err := newSubprocessLauncher()
	require.NoError(t, err)

	require.NoError(t, launcher.runSync(t.Context(), "event.json", io.Discard))
}

// TestNewSubprocessLauncherRefusesInTestBinaryWithoutOptIn ensures analytics
// cannot accidentally re-execute an unprepared test binary and start a growing
// chain of test processes. See AllowSubprocessInTestsEnv for the safe opt-in.
func TestNewSubprocessLauncherRefusesInTestBinaryWithoutOptIn(t *testing.T) {
	t.Setenv(AllowSubprocessInTestsEnv, "")
	t.Setenv(analyticsSubprocessEnv, "")

	_, err := newSubprocessLauncher()

	require.ErrorContains(t, err, "test binary")
}

func TestNewSubprocessLauncherRefusesNestedAnalyticsSubprocess(t *testing.T) {
	t.Setenv(AllowSubprocessInTestsEnv, "1")
	t.Setenv(analyticsSubprocessEnv, "1")

	_, err := newSubprocessLauncher()

	require.ErrorContains(t, err, "another analytics subprocess")
}

func TestIsSendSubprocess(t *testing.T) {
	t.Setenv(analyticsSubprocessEnv, "1")
	require.True(t, IsSendSubprocess())

	t.Setenv(analyticsSubprocessEnv, "")
	require.False(t, IsSendSubprocess())
}

func TestNewSubprocess(t *testing.T) {
	launcher := newTestSubprocessLauncher("/path/to/render")

	cmd := launcher.newSubprocess(t.Context(), "/state/event.json", nil)

	require.Equal(t, []string{
		"/path/to/render",
		"analytics",
		"send",
		"/state/event.json",
	}, cmd.Args)
	require.Nil(t, cmd.Stdin, "subprocess should read stdin from the null device")
	require.Nil(t, cmd.Stdout)
	require.Nil(t, cmd.Stderr)
	require.Contains(t, cmd.Env, analyticsSubprocessEnv+"=1")
	require.Empty(t, cmd.Dir,
		"subprocess must run in the parent's cwd: a relative RENDER_CLI_CONFIG_DIR resolves against it, and a child launched elsewhere would orphan its event file")
}

func TestNewSubprocessOverridesStderr(t *testing.T) {
	launcher := newTestSubprocessLauncher("/path/to/render")

	var stderr bytes.Buffer
	cmd := launcher.newSubprocess(t.Context(), "event.json", &stderr)

	require.Nil(t, cmd.Stdin)
	require.Nil(t, cmd.Stdout)
	require.Same(t, &stderr, cmd.Stderr)
}

func TestStartDetachedDoesNotWait(t *testing.T) {
	launcher := newHelperLauncher(t, helperModeWaitOnGateFile)

	gate := filepath.Join(t.TempDir(), "continue")
	done := make(chan error, 1)
	go func() {
		done <- launcher.startDetached(gate)
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		// Unblock the helper before failing so a regression does not leave it
		// running after the test.
		require.NoError(t, os.WriteFile(gate, nil, 0o600))
		<-done
		t.Fatal("detached launcher waited for the child")
	}

	require.NoError(t, os.WriteFile(gate, nil, 0o600))
}

// testDetachedChildOutlivesLauncherProcess exercises the platform-specific
// process attributes through a three-process chain: the test waits for an
// intermediate launcher to exit before allowing its detached child to write a
// marker. Each supported OS registers this helper from its build-tagged test
// file so CI runs the lifecycle assertion against that OS's implementation.
func testDetachedChildOutlivesLauncherProcess(t *testing.T) {
	t.Helper()

	testDir := t.TempDir()
	gate := filepath.Join(testDir, "continue")
	marker := gate + ".completed"

	launcher := exec.Command(os.Args[0], gate)
	launcher.Env = append(os.Environ(), helperModeEnv+"="+helperModeStartDetached)
	require.NoError(t, launcher.Run(), "intermediate launcher must start the detached child and exit")

	require.NoError(t, os.WriteFile(gate, nil, 0o600))
	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, helperGateTimeout, 10*time.Millisecond,
		"detached child must keep running after its launcher exits")
}

func TestRunSync(t *testing.T) {
	launcher := newHelperLauncher(t, helperModeSuccess)

	require.NoError(t, launcher.runSync(t.Context(), "event.json", nil))
}

func TestRunSyncInheritsEnvironmentAndOverridesStderr(t *testing.T) {
	launcher := newHelperLauncher(t, helperModeReportEnvironment)
	t.Setenv("DO_NOT_TRACK", "1")
	t.Setenv("RENDER_LOG_ANALYTICS", "logged")
	t.Setenv("RENDER_CLI_CONFIG_DIR", "/custom/config")

	var stderr bytes.Buffer
	require.NoError(t, launcher.runSync(t.Context(), "event.json", &stderr))
	require.Equal(t, "1|logged|/custom/config", stderr.String())
}

func TestRunSyncKillsChildAfterDeadline(t *testing.T) {
	launcher := newHelperLauncher(t, helperModeWaitOnGateFile)
	launcher.timeout = 100 * time.Millisecond

	// The gate is never created, so only the deadline can end the child.
	gate := filepath.Join(t.TempDir(), "never-created")
	started := time.Now()
	err := launcher.runSync(t.Context(), gate, nil)

	require.ErrorContains(t, err, "analytics subprocess exceeded 100ms deadline")
	require.Less(t, time.Since(started), helperGateTimeout)
}

// newHelperLauncher returns a launcher whose "render" is this test binary
// re-executed in the given helper mode. See [runHelper] for what each mode does.
func newHelperLauncher(t *testing.T, mode string) subprocessLauncher {
	t.Setenv(helperModeEnv, mode)
	return newTestSubprocessLauncher(os.Args[0])
}

// runHelper impersonates "render analytics send" in a child process launched by
// [newHelperLauncher] and returns the exit code that child should use.
func runHelper(mode string) int {
	switch mode {
	case helperModeSuccess:
		return 0
	case helperModeReportEnvironment:
		_, _ = fmt.Fprintf(
			os.Stderr,
			"%s|%s|%s",
			os.Getenv("DO_NOT_TRACK"),
			os.Getenv("RENDER_LOG_ANALYTICS"),
			os.Getenv("RENDER_CLI_CONFIG_DIR"),
		)
		return 0
	case helperModeWaitOnGateFile:
		// The launcher always passes the event file as the child's final argument,
		// so the helper uses that argument as the gate path.
		gate := os.Args[len(os.Args)-1]
		deadline := time.Now().Add(helperGateTimeout)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(gate); err == nil {
				return 0
			}
			time.Sleep(10 * time.Millisecond)
		}
		return 1
	case helperModeStartDetached:
		// Become an intermediate parent that exits immediately after detaching a
		// second helper. The second helper inherits its new mode from the
		// environment, just as the real analytics child inherits its settings.
		if err := os.Setenv(helperModeEnv, helperModeMarkAfterGate); err != nil {
			return 1
		}
		launcher := newTestSubprocessLauncher(os.Args[0])
		if err := launcher.startDetached(os.Args[len(os.Args)-1]); err != nil {
			return 1
		}
		return 0
	case helperModeMarkAfterGate:
		gate := os.Args[len(os.Args)-1]
		deadline := time.Now().Add(helperGateTimeout)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(gate); err == nil {
				if err := os.WriteFile(gate+".completed", nil, 0o600); err != nil {
					return 1
				}
				return 0
			}
			time.Sleep(10 * time.Millisecond)
		}
		return 1
	default:
		_, _ = fmt.Fprintf(os.Stderr, "unknown analytics subprocess helper mode %q", mode)
		return 2
	}
}

func newTestSubprocessLauncher(executable string) subprocessLauncher {
	return subprocessLauncher{
		executable: executable,
		timeout:    subprocessTimeout,
	}
}
