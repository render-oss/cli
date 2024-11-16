package cmd

import (
	"context"
	"fmt"

	"github.com/renderinc/cli/pkg/user"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display information about the current user",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWhoami(cmd.Context())
	},
	GroupID: GroupAuth.ID,
}

func runWhoami(ctx context.Context) error {
	c, err := client.NewDefaultClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	userRepo := user.NewRepo(c)
	currentUser, err := userRepo.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}

	fmt.Printf("Name: %s\n", currentUser.Name)
	fmt.Printf("Email: %s\n", currentUser.Email)

	return nil
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
