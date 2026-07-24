package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

func TestNewExecutionResult(t *testing.T) {
	startedAt := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
	durationBeforeConstruction := time.Since(startedAt)
	result := newExecutionResult(&cobra.Command{Use: "test"}, command.CompletionKindExplicitExit, 17, startedAt)
	durationAfterConstruction := time.Since(startedAt)

	require.Equal(t, "test", result.CommandPath)
	require.Equal(t, command.CompletionKindExplicitExit, result.CompletionKind)
	require.Equal(t, 17, result.ExitCode)
	require.Equal(t, startedAt, result.StartedAt)
	require.GreaterOrEqual(t, result.Duration, durationBeforeConstruction)
	require.LessOrEqual(t, result.Duration, durationAfterConstruction)
}

func TestOutputFormatFromExecution(t *testing.T) {
	testCases := []struct {
		name   string
		output command.Output
	}{
		{name: "interactive", output: command.Interactive},
		{name: "json", output: command.JSON},
		{name: "yaml", output: command.YAML},
		{name: "text", output: command.TEXT},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "test"}
			cmd.SetContext(command.SetFormatInContext(context.Background(), &tc.output))
			require.Equal(t, &tc.output, outputFormatFromExecution(cmd))
		})
	}

	t.Run("unset", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.SetContext(context.Background())
		require.Nil(t, outputFormatFromExecution(cmd))
	})
}
