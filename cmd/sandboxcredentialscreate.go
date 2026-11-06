package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxcredential"
	"github.com/render-oss/cli/pkg/text"
)

func newSandboxCredentialsCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create <credentialId>",
		Short:        "Create a sandbox group credential",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Create a brokered credential in a sandbox group. Requires an admin.

Uses the active workspace's default sandbox group unless --group is provided.
Pass --domain, --injection-type, and either --value or --value-file.
Header, cookie, and query injection also require --injection-name. Basic
injection uses a value in username:password form. The optional template must
contain 1-5 literal ${SECRET} placeholders.

Use --value-file PATH to read a secret from a file, or --value-file - to read
from stdin. File contents are used exactly as provided, including trailing
newlines. Secret values are never printed.

A successful create saves the desired state; runtime reconciliation may still
be pending.`,
		Example: `  # Create a credential injected into an Authorization header
  render sandbox-groups credentials create inference-key \
    --domain api.example.com --value-file ./secret.txt \
    --injection-type header --injection-name Authorization \
    --injection-template 'Bearer ${SECRET}'

  # Read a secret from stdin
  printf '%s' "$API_TOKEN" | render sandbox-groups credentials create inference-key \
    --domain api.example.com --value-file - \
    --injection-type header --injection-name X-API-Key`,
	}
	addSandboxCredentialWriteFlags(cmd)

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxCredentialsWriteInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		if input.Domain == nil || input.InjectionType == nil || (input.Value == nil && input.ValueFile == nil) {
			return fmt.Errorf("create requires --domain, --injection-type, and either --value or --value-file")
		}

		value, err := readSandboxCredentialValue(cmd, input.Value, input.ValueFile)
		if err != nil {
			return err
		}

		_, err = command.NonInteractive(cmd, func() (*sandboxesclient.SandboxCredential, error) {
			return deps.SandboxCredentialService().Create(cmd.Context(), sandboxcredential.CreateInput{
				SandboxGroupID: input.Group,
				CredentialID:   input.CredentialID,
				Domain:         *input.Domain,
				Value:          *value,
				Enabled:        input.Enabled,
				Methods:        input.Methods,
				Paths:          input.Paths,
				Injection:      *input.injection(),
			})
		}, text.SandboxCredentialDetail)
		return err
	}
	return cmd
}
