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

// CompletionKind describes how a CLI execution finished.
type CompletionKind string

const (
	// CompletionKindSuccess indicates that the selected command ran successfully.
	CompletionKindSuccess CompletionKind = "success"
	// CompletionKindHelp indicates that the user explicitly requested help, via
	// the --help / -h flag or the builtin help command. The result's Command is
	// the command whose help was displayed: the root command for `render --help`,
	// and the services command for both `render services --help` and
	// `render help services`.
	//
	// Help that Cobra displays on its own initiative — such as when a parent
	// command with no run function is invoked without a subcommand — is
	// classified as success, not help.
	CompletionKindHelp CompletionKind = "help"
	// CompletionKindVersion indicates that the CLI displayed its root version output.
	CompletionKindVersion CompletionKind = "version"
	// CompletionKindDiscoveryError indicates that the user named a command that
	// does not exist, as in `render bogus` or `render help bogus`. It reflects
	// user input on a working CLI, not a broken binary: Cobra could not find a
	// command to select. Unknown children of command groups land here however
	// the group rejects them: Cobra silently rendering group help
	// (`render postgres bogus`, exit 0) and arg validation rejecting the token
	// (`render ea bogus`, exit 1) are the same user mistake.
	CompletionKindDiscoveryError CompletionKind = "discovery_error"
	// CompletionKindValidationError indicates that a selected command rejected its flags or arguments.
	CompletionKindValidationError CompletionKind = "validation_error"
	// CompletionKindSetupError indicates that the CLI failed while preparing to
	// run the selected command, before the command itself began. Usually the
	// user can correct it: an invalid --output value, a removed legacy flag, or
	// a failure to build the API client all land here.
	CompletionKindSetupError CompletionKind = "setup_error"
	// CompletionKindExecutionError indicates that the selected command ran and
	// returned an error — the ordinary failure mode of a working CLI, such as a
	// failed API request or a nonexistent resource ID. It does not distinguish
	// user mistakes from Render-side or CLI defects. Errors from post-run hooks
	// also land here.
	CompletionKindExecutionError CompletionKind = "execution_error"
	// CompletionKindExplicitExit indicates that the command deliberately chose
	// the process exit code by returning an ExitCoder, typically to report an
	// outcome external to the CLI rather than a CLI failure: `render sandbox exec`
	// propagates the remote process's exit code, and `render deploys create --wait`
	// exits nonzero when the deploy fails.
	CompletionKindExplicitExit CompletionKind = "explicit_exit"
)

// ExecutionResult describes one completed CLI execution.
type ExecutionResult struct {
	// Command is the command Cobra selected, or the root command when discovery failed.
	Command *cobra.Command
	// CompletionKind classifies the phase and manner in which execution finished.
	CompletionKind CompletionKind
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
