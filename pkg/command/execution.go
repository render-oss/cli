package command

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// ExitCoder is an error that specifies the process exit code it represents.
type ExitCoder interface {
	error
	ExitCode() int
}

// ExecutionResult describes one completed CLI execution.
type ExecutionResult struct {
	// Command is the command Cobra selected, or the root command when the
	// execution never reached Cobra command selection.
	Command *cobra.Command
	// Duration is the elapsed wall-clock time for the execution.
	Duration time.Duration
	// ExitCode is the process exit code returned by the top-level executor.
	ExitCode int
}

// exitError carries a process exit code through Cobra's error-return path.
type exitError struct {
	// code is the intended process exit code for the command.
	code int
	// cause is the optional underlying error.
	cause error
}

// Error implements the error interface.
func (e exitError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return fmt.Sprintf("exited with code %d", e.code)
}

// Unwrap returns the underlying error, if any.
func (e exitError) Unwrap() error {
	return e.cause
}

// ExitCode returns the process exit code represented by the error.
func (e exitError) ExitCode() int {
	return e.code
}

// NewExitError returns an error carrying a process exit code. If cause is
// non-nil, the returned error wraps it and uses its message. Commands should return
// nil, rather than an exit error with code zero, to represent success.
func NewExitError(exitCode int, cause error) error {
	return exitError{code: exitCode, cause: cause}
}
