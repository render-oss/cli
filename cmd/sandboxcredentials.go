package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newSandboxCredentialsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credentials",
		Short: "Manage sandbox brokered credentials",
		Long: `Manage brokered credentials for a sandbox group. The gateway injects each
secret into matching outbound requests. Secret values are never returned.

Uses the active workspace's default sandbox group unless --group is provided.
Creating, deleting, restoring, or changing a secret value or domain requires an
admin. Developers can change injection, method/path filters, and enabled state.`,
	}
	cmd.AddCommand(children...)
	return cmd
}

func validateSandboxCredentialID(id string) error {
	// The generated client resolves these as URL navigation rather than IDs.
	// Leave credential naming rules to the server.
	if id == "" || id == "." || id == ".." {
		return fmt.Errorf("credential ID must not be empty, '.' or '..'")
	}
	return nil
}
