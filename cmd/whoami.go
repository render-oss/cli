package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/owner"
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

	ownerRepo := owner.NewRepo(c)

	owners, err := ownerRepo.ListOwners(ctx, owner.ListInput{})
	if err != nil {
		return fmt.Errorf("failed to list owners: %w", err)
	}

	var currentUser *client.Owner
	// an owner list response will always have exactly one owner of type user
	for _, o := range owners {
		if o.Type == client.OwnerTypeUser {
			currentUser = o
			break
		}
	}

	if currentUser == nil {
		return fmt.Errorf("no user found in the list of owners")
	}

	fmt.Printf("Name: %s\n", currentUser.Name)
	fmt.Printf("ID: %s\n", currentUser.Id)
	fmt.Printf("Email: %s\n", currentUser.Email)

	return nil
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}
