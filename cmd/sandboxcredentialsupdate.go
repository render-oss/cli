package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

func newSandboxCredentialsUpdateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "update <credentialId>",
		Short:        "Update a sandbox group credential",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Update a brokered credential. Use --value-file PATH to read a secret from a
file, or --value-file - to read it from stdin. File contents are used exactly as
provided, including trailing newlines. Secret values are never printed.
Uses the active workspace's default sandbox group unless --group is provided.

Omitted update fields keep their current values. Use --methods= or --paths= to
clear filters. Injection flags replace the injection configuration together and
require --injection-type; header, cookie, and query also require --injection-name.
The optional template must contain 1-5 literal ${SECRET} placeholders.

Changing a credential's domain or secret value requires an admin.
A successful write saves the desired state; runtime reconciliation may still be
pending.`,
		Example: `  # Rotate a credential value from a file
  render sandbox-groups credentials update inference-key --value-file ./secret.txt

  # Disable a credential
  render sandbox-groups credentials update inference-key --enabled=false

  # Clear request filters
  render sandbox-groups credentials update inference-key --methods= --paths=`,
	}
	addSandboxCredentialWriteFlags(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCredentialsWriteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		if err := validateSandboxCredentialID(input.CredentialID); err != nil {
			return err
		}
		if input.Domain == nil && input.Value == nil && input.ValueFile == nil && input.injection() == nil && input.Enabled == nil && !cmd.Flags().Changed("methods") && !cmd.Flags().Changed("paths") {
			return fmt.Errorf("specify at least one credential field to update")
		}

		value, err := readSandboxCredentialValue(cmd, input.Value, input.ValueFile)
		if err != nil {
			return err
		}

		_, err = command.NonInteractive(cmd, func() (*sandboxesclient.SandboxCredential, error) {
			return deps.SandboxCredentialService().Update(cmd.Context(), input.updateInput(cmd, value))
		}, text.SandboxCredentialDetail)
		return err
	}
	return cmd
}
