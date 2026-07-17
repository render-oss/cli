package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/cfg"
	commandpkg "github.com/render-oss/cli/pkg/command"
)

// onExecutionCompleteFunc runs side effects after a command execution finishes.
type onExecutionCompleteFunc func(result commandpkg.ExecutionResult)

// onExecutionComplete is our hook into the end of an execution
// Will eventually house telemetry emission
var onExecutionComplete onExecutionCompleteFunc = func(result commandpkg.ExecutionResult) {
	logExecutionResult(result)
}

// newExecutionResult constructs a result when an execution finishes, measuring
// its duration from startedAt to now.
func newExecutionResult(command *cobra.Command, exitCode int, startedAt time.Time) commandpkg.ExecutionResult {
	return commandpkg.ExecutionResult{
		Command:  command,
		Duration: time.Since(startedAt),
		ExitCode: exitCode,
	}
}

// logExecutionResult writes the result to stderr when RENDER_LOG_ANALYTICS=1.
func logExecutionResult(result commandpkg.ExecutionResult) {
	if !cfg.ShouldLogAnalytics() {
		return
	}

	out := io.Writer(os.Stderr)
	commandPath := ""
	if result.Command != nil {
		out = result.Command.ErrOrStderr()
		commandPath = result.Command.CommandPath()
	}
	_, _ = fmt.Fprintf(out, "execution: command=%q exit_code=%d duration=%s\n",
		commandPath, result.ExitCode, result.Duration)
}
