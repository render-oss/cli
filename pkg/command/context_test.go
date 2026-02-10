// pattern: Imperative Shell
package command_test

import (
	"context"
	"testing"

	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestDefaultFormatNonInteractive(t *testing.T) {
	testCases := []struct {
		name       string
		setup      func() context.Context
		wantOutput command.Output
	}{
		{
			name: "interactive output becomes text",
			setup: func() context.Context {
				output := command.Interactive
				return command.SetFormatInContext(context.Background(), &output)
			},
			wantOutput: command.TEXT,
		},
		{
			name: "json output remains json",
			setup: func() context.Context {
				output := command.JSON
				return command.SetFormatInContext(context.Background(), &output)
			},
			wantOutput: command.JSON,
		},
		{
			name: "yaml output remains yaml",
			setup: func() context.Context {
				output := command.YAML
				return command.SetFormatInContext(context.Background(), &output)
			},
			wantOutput: command.YAML,
		},
		{
			name: "text output remains text",
			setup: func() context.Context {
				output := command.TEXT
				return command.SetFormatInContext(context.Background(), &output)
			},
			wantOutput: command.TEXT,
		},
		{
			name: "nil output context becomes text",
			setup: func() context.Context {
				return context.Background()
			},
			wantOutput: command.TEXT,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{Use: "compatibility"}
			cmd.SetContext(tc.setup())

			command.DefaultFormatNonInteractive(cmd)

			output := command.GetFormatFromContext(cmd.Context())
			require.NotNil(t, output)
			require.Equal(t, tc.wantOutput, *output)
		})
	}
}

func TestDefaultFormatNonInteractive_CommandFlowCompatibility(t *testing.T) {
	cmd := &cobra.Command{Use: "synthetic-command-flow"}
	output := command.Interactive
	cmd.SetContext(command.SetFormatInContext(context.Background(), &output))

	command.DefaultFormatNonInteractive(cmd)

	resolved := command.GetFormatFromContext(cmd.Context())
	require.NotNil(t, resolved)
	require.Equal(t, command.TEXT, *resolved)
}
