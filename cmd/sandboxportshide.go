package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandbox"
	"github.com/render-oss/cli/pkg/text"
)

func newSandboxPortsHideCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "hide <sandboxId> <port>",
		Short:        "Stop exposing a sandbox port",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		Long: `Remove a port's public URL and close its open connections.

Hiding a port that is not exposed succeeds.`,
		Example: `  # Stop exposing port 8080
  render sandboxes ports hide sbx-abc123 8080

  # JSON output
  render sandboxes ports hide sbx-abc123 8080 --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxPortInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		port, err := input.port()
		if err != nil {
			return err
		}

		_, err = command.NonInteractive(cmd, func() (*sandbox.HiddenPort, error) {
			return deps.SandboxService().HidePort(cmd.Context(), input.SandboxID, port)
		}, func(p *sandbox.HiddenPort) string {
			return text.FormatStringF("Port %d on %s is not exposed", p.Port, p.SandboxID)
		})
		return err
	}

	return cmd
}
