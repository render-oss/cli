package cmd

import (
	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxCredentialsRestoreInput struct {
	CredentialID string `cli:"arg:0"`
	Group        string `cli:"group"`
	// ParseCommand does not support int64 flags, so RunE reads this separately.
	VersionID int64
}

func (i *SandboxCredentialsRestoreInput) Validate(_ bool) error {
	return validateSandboxCredentialID(i.CredentialID)
}

func newSandboxCredentialsRestoreCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "restore <credentialId>",
		Short:        "Restore a sandbox credential version",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Restore the routing, injection, and secret value of a retained version as a new current revision. Requires an admin.

Uses the active workspace's default sandbox group unless --group is provided.`,
		Example: `  # Restore version 2 as a new current revision
  render ea sandbox-groups credentials restore inference-key --version 2`,
	}
	cmd.Flags().String("group", "", "Use this sandbox group (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")
	cmd.Flags().Int64("version", 0, "Restore this retained configuration version")
	_ = cmd.MarkFlagRequired("version")
	setFlagPlaceholder(cmd.Flags(), "version", "VERSION_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		version, err := cmd.Flags().GetInt64("version")
		if err != nil {
			return err
		}
		input := SandboxCredentialsRestoreInput{VersionID: version}
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err = command.NonInteractive(cmd, func() (*sandboxesclient.SandboxCredential, error) {
			return deps.SandboxCredentialService().Restore(cmd.Context(), input.Group, input.CredentialID, input.VersionID)
		}, text.SandboxCredentialDetail)
		return err
	}
	return cmd
}
