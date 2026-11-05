package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxCredentialsListInput struct {
	Group string `cli:"group"`
}

func newSandboxCredentialsListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List sandbox group credentials",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List brokered credentials in a sandbox group. Secret values are never returned.

Uses the active workspace's default sandbox group unless --group is provided.`,
		Example: `  # List credentials in the default sandbox group
  render sandbox-groups credentials list

  # Output as JSON
  render sandbox-groups credentials list --output json`,
	}
	cmd.Flags().String("group", "", "Use this sandbox group (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCredentialsListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() ([]sandboxesclient.SandboxCredential, error) {
			return deps.SandboxCredentialService().List(cmd.Context(), input.Group)
		}, text.SandboxCredentialTable)
		return err
	}
	return cmd
}
