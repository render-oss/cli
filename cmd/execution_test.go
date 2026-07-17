package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/pkg/command"
)

func TestLogExecutionResult(t *testing.T) {
	newResult := func(errOut *bytes.Buffer) command.ExecutionResult {
		cmd := &cobra.Command{Use: "render"}
		cmd.SetErr(errOut)
		return command.ExecutionResult{
			Command:  cmd,
			Duration: 250 * time.Millisecond,
			ExitCode: 0,
		}
	}

	t.Run("logs the result to stderr when enabled", func(t *testing.T) {
		t.Setenv("RENDER_LOG_ANALYTICS", "1")
		var errOut bytes.Buffer

		logExecutionResult(newResult(&errOut))

		testassert.ContainsInOrder(t, errOut.String(),
			"execution:", `command="render"`, "exit_code=0", "duration=250ms")
	})

	t.Run("logs nothing when disabled", func(t *testing.T) {
		t.Setenv("RENDER_LOG_ANALYTICS", "")
		var errOut bytes.Buffer

		logExecutionResult(newResult(&errOut))

		require.Empty(t, errOut.String())
	})
}
