package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/render-oss/cli/pkg/user"
)

var workspacesCmd = &cobra.Command{
	Use:     "workspaces",
	Short:   "List workspaces",
	GroupID: GroupCore.ID,
}

func loadWorkspaces(ctx context.Context) ([]*client.Owner, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	ownerRepo := owner.NewRepo(c)
	return ownerRepo.ListOwners(ctx, owner.ListInput{})
}

func interactiveWorkspaces(ctx context.Context, input views.ListWorkspaceInput) {
	stack := tui.GetStackFromContext(ctx)
	stack.SetWorkspaceOverride(userEmail(ctx))
	command.AddToStackFunc(ctx, workspacesCmd, "Workspaces", &input, views.NewWorkspaceView(ctx, input))
}

func userEmail(ctx context.Context) string {
	email, _ := config.UserEmail()
	if email != "" {
		return email
	}

	c, err := client.NewDefaultClient()
	if err != nil {
		return ""
	}

	u, err := user.NewRepo(c).CurrentUser(ctx)
	if err != nil {
		return ""
	}

	_ = config.SetUserEmail(u.Email)
	return u.Email
}

func init() {
	rootCmd.AddCommand(workspacesCmd)

	workspacesCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.ListWorkspaceInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]*client.Owner, error) {
			return loadWorkspaces(cmd.Context())
		}, text.WorkspaceTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		interactiveWorkspaces(cmd.Context(), input)
		return nil
	}
}
