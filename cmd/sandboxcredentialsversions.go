package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxCredentialsVersionsInput struct {
	CredentialID string `cli:"arg:0"`
	Group        string `cli:"group"`
}

func (i *SandboxCredentialsVersionsInput) Validate(_ bool) error {
	return validateSandboxCredentialID(i.CredentialID)
}

func newSandboxCredentialsVersionsCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "versions <credentialId>",
		Short:        "List a sandbox credential's versions",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `List retained configuration versions, newest first. Secret values are never returned.

Uses the active workspace's default sandbox group unless --group is provided.`,
		Example: `  # List a sandbox credential's versions
  render sandbox-groups credentials versions inference-key

  # Output as JSON for a specific group
  render sandbox-groups credentials versions inference-key --group sbg-abc123 --output json`,
	}
	cmd.Flags().String("group", "", "Use this sandbox group (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCredentialsVersionsInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() ([]sandboxesclient.SandboxCredentialVersion, error) {
			return deps.SandboxCredentialService().Versions(cmd.Context(), input.Group, input.CredentialID)
		}, text.SandboxCredentialVersionTable)
		return err
	}
	return cmd
}
