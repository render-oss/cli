package cmd

import (
	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxPortsListInput struct {
	SandboxID string `cli:"arg:0"`
}

func newSandboxPortsListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list <sandboxId>",
		Short:        "List a sandbox's exposed ports",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long:         `List the ports a sandbox exposes and their public URLs.`,
		Example: `  # List exposed ports
  render sandboxes ports list sbx-abc123

  # JSON output
  render sandboxes ports list sbx-abc123 --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxPortsListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (sandboxclient.SandboxExposedPorts, error) {
			return deps.SandboxService().ListPorts(cmd.Context(), input.SandboxID)
		}, text.SandboxPortTable)
		return err
	}

	return cmd
}
