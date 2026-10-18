package cmd

import (
	"errors"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
)

// exitWithProcessCode returns an error that carries the exit code of a process
// whose output the CLI already streamed to the user. A zero exit code returns
// nil. The CLI does not add an error message that the process did not produce.
func exitWithProcessCode(cmd *cobra.Command, exitCode int) error {
	if exitCode == 0 {
		return nil
	}
	cmd.Root().SilenceErrors = true
	return command.NewExitError(exitCode, nil)
}

// processExitCode reads the exit code of a local process from the error that
// exec.Cmd.Run returned. A process that a signal stopped maps to 128 plus the
// signal number, as a POSIX shell reports it. The boolean is false when the
// error does not describe a process that ran to completion.
func processExitCode(err error) (int, bool) {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 0, false
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal()), true
	}
	if exitErr.ExitCode() < 0 {
		return 0, false
	}
	return exitErr.ExitCode(), true
}
