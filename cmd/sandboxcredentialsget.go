package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxCredentialsGetInput struct {
	CredentialID string `cli:"arg:0"`
	Group        string `cli:"group"`
}

func (i *SandboxCredentialsGetInput) Validate(_ bool) error {
	return validateSandboxCredentialID(i.CredentialID)
}

func newSandboxCredentialsGetCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get <credentialId>",
		Short:        "Get a sandbox group credential",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Get a sandbox credential by ID. Secret values are never returned.

Uses the active workspace's default sandbox group unless --group is provided.`,
		Example: `  # Get a sandbox group credential
  render ea sandbox-groups credentials get inference-key

  # Output as JSON for a specific group
  render ea sandbox-groups credentials get inference-key --group sbg-abc123 --output json`,
	}
	cmd.Flags().String("group", "", "Use this sandbox group (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCredentialsGetInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxCredential, error) {
			return deps.SandboxCredentialService().Get(cmd.Context(), input.Group, input.CredentialID)
		}, text.SandboxCredentialDetail)
		return err
	}
	return cmd
}
