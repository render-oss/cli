package analytics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"
)

// subprocessTimeout bounds the full lifetime of a synchronous analytics
// subprocess. It leaves about 500ms around the child's bounded 2500ms API
// request for process startup and teardown.
const subprocessTimeout = 3 * time.Second

// subprocessWaitDelay bounds how long a synchronous send waits for a killed
// child's output pipes to drain.
const subprocessWaitDelay = time.Second

type subprocessLauncher struct {
	executable string
	timeout    time.Duration
}

// analyticsSubprocessEnv marks a process started by subprocessLauncher. Besides
// helping test binaries route the child invocation, it prevents an analytics
// subprocess from starting another analytics subprocess.
const analyticsSubprocessEnv = "RENDER_CLI_ANALYTICS_SUBPROCESS"

// AllowSubprocessInTestsEnv lets a test binary launch a real
// "render analytics send" subprocess. Set it to "1" and give the test binary a
// TestMain that routes marked analytics children to the CLI entrypoint:
//
//	func TestMain(m *testing.M) {
//		if analytics.IsSendSubprocess() {
//			os.Exit(Execute())
//		}
//		os.Exit(m.Run())
//	}
//
// Sending re-executes the running program, which under "go test" is the test
// binary. A test binary handed
// "analytics send <file>" ignores those arguments and runs its whole test suite
// instead, and each of those runs sends its own events and starts more children.
// The environment variable permits the launch. The TestMain routes the marked
// child to the send command. Without both pieces, launcher construction fails
// in a test binary. Outside "go test" the variable is ignored.
const AllowSubprocessInTestsEnv = "RENDER_CLI_ALLOW_ANALYTICS_SUBPROCESS_IN_TESTS"

// IsSendSubprocess reports whether the current process was launched to run
// "render analytics send <event-file>". TestMain functions can use it to route
// a re-executed test binary to the CLI entrypoint instead of the test suite.
func IsSendSubprocess() bool {
	return os.Getenv(analyticsSubprocessEnv) == "1"
}

// subprocessAllowedInTests reports whether a test binary has opted in
// to launching real analytics send subprocesses.
func subprocessAllowedInTests() bool {
	return os.Getenv(AllowSubprocessInTestsEnv) == "1"
}

// newSubprocessLauncher creates a launcher that re-executes the currently
// running Render binary.
//
// It refuses nested analytics subprocesses in every binary. It also refuses a
// launch from a test binary unless the test follows the opt-in contract
// documented by [AllowSubprocessInTestsEnv].
//
// Callers can ignore a refusal: [Sender.Send] does not write an event file until
// it has a launcher, so no event file or subprocess is left behind.
func newSubprocessLauncher() (subprocessLauncher, error) {
	if IsSendSubprocess() {
		return subprocessLauncher{}, errors.New("refusing to launch an analytics subprocess from another analytics subprocess")
	}

	if testing.Testing() && !subprocessAllowedInTests() {
		return subprocessLauncher{}, errors.New(
			"refusing to launch an analytics subprocess from a test binary: re-executing it would re-run the whole test suite; set " +
				AllowSubprocessInTestsEnv + "=1 and add a TestMain that runs the send command")
	}

	executable, err := os.Executable()
	if err != nil {
		return subprocessLauncher{}, fmt.Errorf("resolving analytics subprocess executable: %w", err)
	}

	return subprocessLauncher{
		executable: executable,
		timeout:    subprocessTimeout,
	}, nil
}

// newSubprocess configures an [exec.Cmd] to run "render analytics send <eventFile>".
// Cancelling ctx kills the child.
func (l subprocessLauncher) newSubprocess(ctx context.Context, eventFile string, stderr io.Writer) *exec.Cmd {
	cmd := exec.CommandContext(ctx, l.executable, "analytics", "send", eventFile)
	// cmd.Dir must stay unset: a relative RENDER_CLI_CONFIG_DIR resolves the
	// events directory against the working directory, so a child running
	// anywhere but the parent's cwd would reject and orphan its event file.
	//
	// Preserve the parent's environment and mark the child so it cannot start
	// another analytics subprocess. Nil streams are connected to the null device
	// by os/exec; a stderr override lets a synchronous caller capture child
	// diagnostics without inheriting the parent's stream.
	cmd.Env = append(os.Environ(), analyticsSubprocessEnv+"=1")
	cmd.Stderr = stderr
	return cmd
}

// startDetached runs "render analytics send <eventFile>" as a background
// subprocess and returns without waiting. Process.Release relinquishes this
// process's tracking resources without terminating the child. Callers must only
// invoke this immediately before the CLI exits: Unix reparents the child to a
// system reaper, while Windows has no corresponding parent-side zombie-reaping
// requirement.
func (l subprocessLauncher) startDetached(eventFile string) error {
	// Deliberately unhooked from any caller context: the detached child must
	// outlive the parent process that launched it.
	cmd := l.newSubprocess(context.Background(), eventFile, nil)
	setDetachedProcessAttributes(cmd)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting detached analytics subprocess: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("releasing detached analytics subprocess: %w", err)
	}
	return nil
}

// runSync runs "render analytics send <eventFile>" as a sub-process in the foreground, blocking until done
// That means in sync mode, users do not get control of their terminal back until this is finished
// The timeout covers launching the child as well as running it.
func (l subprocessLauncher) runSync(ctx context.Context, eventFile string, stderr io.Writer) error {
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()

	cmd := l.newSubprocess(ctx, eventFile, stderr)
	// Only a caller that waits needs this: it bounds how long Wait blocks
	// draining a killed child's output pipes, which a grandchild holding the
	// write end would otherwise hold open forever.
	cmd.WaitDelay = subprocessWaitDelay

	err := cmd.Run()
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// The deadline that killed the child explains the failure better than the
		// child's resulting exit error does. The context may expire just after the
		// child exits successfully, so only treat the deadline as the cause when
		// cmd.Run also reports an error.
		return fmt.Errorf("analytics subprocess exceeded %s deadline", l.timeout)
	}
	if err != nil {
		return fmt.Errorf("running synchronous analytics subprocess: %w", err)
	}
	return nil
}
