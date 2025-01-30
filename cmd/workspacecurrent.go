package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/owner"
)

var workspaceCurrentCmd = &cobra.Command{
	Use:   "current",
	Short: "Show the currently selected workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		c, err := client.NewDefaultClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		ownerRepo := owner.NewRepo(c)
		workspace, err := config.WorkspaceID()
		if err != nil {
			return err
		}

		owner, err := ownerRepo.RetrieveOwner(cmd.Context(), workspace)
		if err != nil {
			return fmt.Errorf("failed to list owners: %w", err)
		}

		return printWorkspace(cmd, "Active Workspace", owner)
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceCurrentCmd)
}
