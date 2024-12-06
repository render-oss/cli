package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/project"
	"github.com/renderinc/cli/pkg/text"
	"github.com/renderinc/cli/pkg/tui"
	"github.com/renderinc/cli/pkg/tui/views"
)

var environmentCmd = &cobra.Command{
	Use:   "environments [projectID]",
	Args:  cobra.ExactArgs(1),
	Short: "List environments",
	Long: `List environments for a specified project in the active workspace.
In interactive mode you can view each environment's individual services.`,
	GroupID: GroupManagement.ID,
}

var InteractiveEnvironment = func(ctx context.Context, input views.EnvironmentInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, environmentCmd, breadcrumb, &input, views.NewEnvironmentList(ctx, input,
		func(ctx context.Context, e *client.Environment) tea.Cmd {
			return InteractiveServices(ctx, views.ListResourceInput{
				EnvironmentIDs: []string{e.Id},
			}, e.Name)
		},
		tui.WithCustomOptions[*client.Environment]([]tui.CustomOption{
			WithCopyID(ctx, servicesCmd),
			WithWorkspaceSelection(ctx),
		}),
	))
}

func init() {
	rootCmd.AddCommand(environmentCmd)

	environmentCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.EnvironmentInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]*client.Environment, error) {
			return views.LoadEnvironments(cmd.Context(), input)
		}, text.EnvironmentTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		c, err := client.NewDefaultClient()
		if err != nil {
			return err
		}
		projectRepo := project.NewRepo(c)
		proj, err := projectRepo.GetProject(cmd.Context(), input.ProjectID)
		if err != nil {
			return err
		}

		InteractiveEnvironment(cmd.Context(), input, "Environments for "+proj.Name)
		return nil
	}
}
