package cmd

import (
	"fmt"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/user"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display information about the current user",
	Example: `  # Show the currently authenticated user
  render whoami`,
	RunE: func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)
		return runWhoami(cmd)
	},
	GroupID: GroupAuth.ID,
}

func runWhoami(cmd *cobra.Command) error {
	c, err := client.NewDefaultClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	userRepo := user.NewRepo(c)
	_, err = command.NonInteractive(cmd, func() (*client.User, error) {
		currentUser, err := userRepo.CurrentUser(cmd.Context())
		if err != nil {
			return nil, fmt.Errorf("failed to get current user: %w", err)
		}
		return currentUser, nil
	}, formatWhoamiText)
	return err
}

func formatWhoamiText(currentUser *client.User) string {
	return fmt.Sprintf("Name: %s\nEmail: %s\nID: %s\n", currentUser.Name, currentUser.Email, currentUser.Id)
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
