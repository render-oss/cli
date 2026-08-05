package analytics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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

// newSubprocessLauncher creates a launcher that re-executes the currently
// running Render binary.
func newSubprocessLauncher() (subprocessLauncher, error) {
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
	// leave cmd.Env nil so the child inherits analytics opt-in,
	// logging configuration, and config dir from its parent. Nil streams are
	// connected to the null device by os/exec; a stderr override lets a
	// synchronous caller capture child diagnostics without inheriting the
	// parent's stream.
	cmd.Env = nil
	cmd.Stderr = stderr
	return cmd
}

// startDetached runs "render analytics send <eventFile>" as a background subprocess and then returns without waiting
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
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		// The deadline that killed the child explains the failure better than the
		// child's resulting exit error does.
		return fmt.Errorf("analytics subprocess exceeded %s deadline", l.timeout)
	}
	if err != nil {
		return fmt.Errorf("running synchronous analytics subprocess: %w", err)
	}
	return nil
}
