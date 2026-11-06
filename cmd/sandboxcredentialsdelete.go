package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxcredential"
)

type SandboxCredentialsDeleteInput struct {
	CredentialID string `cli:"arg:0"`
	Group        string `cli:"group"`
}

func (i *SandboxCredentialsDeleteInput) Validate(_ bool) error {
	return validateSandboxCredentialID(i.CredentialID)
}

func newSandboxCredentialsDeleteCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete <credentialId>",
		Short:        "Delete a sandbox group credential",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Delete a brokered credential. Requires an admin.

Uses the active workspace's default sandbox group unless --group is provided.
Without --confirm, preview the deletion without making changes.`,
		Example: `  # Preview credential deletion
  render sandbox-groups credentials delete inference-key

  # Delete the credential
  render sandbox-groups credentials delete inference-key --confirm`,
	}
	cmd.Flags().String("group", "", "Use this sandbox group (defaults to the active workspace's default group)")
	setFlagPlaceholder(cmd.Flags(), "group", "SANDBOX_GROUP_ID")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCredentialsDeleteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		confirm := command.GetConfirmFromContext(cmd.Context())

		_, err := command.NonInteractive(cmd, func() (*sandboxcredential.DeleteOut, error) {
			groupID, err := deps.SandboxGroupService().IDOrDefault(cmd.Context(), input.Group)
			if err != nil {
				return nil, err
			}
			out := &sandboxcredential.DeleteOut{
				Data: sandboxcredential.DeleteOutData{ID: input.CredentialID, SandboxGroupID: groupID},
			}
			if !confirm {
				out.Meta.Message = "re-run with --confirm to delete"
				return out, nil
			}
			if err := deps.SandboxCredentialService().Delete(cmd.Context(), groupID, input.CredentialID); err != nil {
				return nil, err
			}
			out.Meta.Deleted = true
			return out, nil
		}, sandboxCredentialsDeleteTextOutput)
		return err
	}
	return cmd
}

func sandboxCredentialsDeleteTextOutput(out *sandboxcredential.DeleteOut) string {
	if out.Meta.Deleted {
		return fmt.Sprintf("Deleted credential %s in group %s\n", out.Data.ID, out.Data.SandboxGroupID)
	}
	return fmt.Sprintf("This command would delete credential %s in group %s\n\nRe-run with --confirm to proceed\n", out.Data.ID, out.Data.SandboxGroupID)
}
