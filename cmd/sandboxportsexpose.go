package cmd

import (
	"github.com/spf13/cobra"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

func newSandboxPortsExposeCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "expose <sandboxId> <port>",
		Short:        "Expose a sandbox port at a public URL",
		Args:         cobra.ExactArgs(2),
		SilenceUsage: true,
		Long: `Expose a port in a sandbox at a public URL and print the URL.

Exposing a port that is already exposed returns its URL. A sandbox can expose
up to 10 ports.`,
		Example: `  # Expose port 8080
  render sandboxes ports expose sbx-abc123 8080

  # JSON output
  render sandboxes ports expose sbx-abc123 8080 --output json`,
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

		_, err = command.NonInteractive(cmd, func() (*sandboxclient.SandboxExposedPort, error) {
			return deps.SandboxService().ExposePort(cmd.Context(), input.SandboxID, port)
		}, text.SandboxPortDetail)
		return err
	}

	return cmd
}
