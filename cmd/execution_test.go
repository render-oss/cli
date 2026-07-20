package cmd

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/pkg/command"
)

func TestLogExecutionResult(t *testing.T) {
	result := command.ExecutionResult{
		CommandPath:    "render",
		CompletionKind: command.CompletionKindSuccess,
		Duration:       250 * time.Millisecond,
		ExitCode:       0,
	}

	t.Run("logs the result to the writer when enabled", func(t *testing.T) {
		t.Setenv("RENDER_LOG_ANALYTICS", "1")
		var errOut bytes.Buffer

		logExecutionResult(result, &errOut)

		testassert.ContainsInOrder(t, errOut.String(),
			"execution:", `command="render"`, "kind=success", "exit_code=0", "duration=250ms")
	})

	t.Run("logs nothing when disabled", func(t *testing.T) {
		t.Setenv("RENDER_LOG_ANALYTICS", "")
		var errOut bytes.Buffer

		logExecutionResult(result, &errOut)

		require.Empty(t, errOut.String())
	})
}
