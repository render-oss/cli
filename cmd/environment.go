package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/spf13/cobra"
)

var environmentCmd = &cobra.Command{
	Use:   "environments [projectID]",
	Args:  cobra.ExactArgs(1),
	Short: "List environments",
	Long: `List environments for the currently set workspace and the specified project.
In interactive mode you can view the services for an environment.`,
}

var InteractiveEnvironment = func(ctx context.Context, input views.EnvironmentInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, environmentCmd, breadcrumb, &input, views.NewEnvironmentList(ctx, input,
		func(ctx context.Context, e *client.Environment) tea.Cmd {
			return InteractiveServices(ctx, views.ListResourceInput{
				EnvironmentIDs: []string{e.Id},
			}, e.Name)
		},
		tui.WithCustomOptions[*client.Environment]([]tui.CustomOption{
			{
				Key:   "w",
				Title: "Change Workspace",
				Function: func(row btable.Row) tea.Cmd {
					return InteractiveWorkspaceSet(ctx, views.ListWorkspaceInput{})
				},
			},
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

		if nonInteractive, err := command.NonInteractive(cmd.Context(), cmd, func() (any, error) {
			return views.LoadEnvironments(cmd.Context(), input)
		}, nil); err != nil {
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

		InteractiveEnvironment(cmd.Context(), input, "Environments for " + proj.Name)
		return nil
	}
}
