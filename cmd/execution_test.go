package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/command"
)

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
