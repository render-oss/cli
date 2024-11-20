package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/deploy"
	"github.com/renderinc/cli/pkg/pointers"
	"github.com/renderinc/cli/pkg/text"

	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/tui/views"
)

var deployListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List deploys for a service",
	Args:  cobra.ExactArgs(1),
}

var InteractiveDeployList = func(ctx context.Context, input views.DeployListInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, deployListCmd, breadcrumb, &input, views.NewDeployListView(
		ctx,
		input,
		func(c *client.Deploy, s string) tea.Cmd {
			return InteractivePalette(ctx, commandsForDeploy(c, s), c.Id)
		},
	))
}

func commandsForDeploy(dep *client.Deploy, serviceID string) []views.PaletteCommand {
	var startTime *string
	if dep.CreatedAt != nil {
		startTime = pointers.From(dep.CreatedAt.String())
	}

	var endTime *string
	if dep.FinishedAt != nil {
		endTime = pointers.From(dep.FinishedAt.String())
	}

	commands := []views.PaletteCommand{
		{
			Name:        "logs",
			Description: "View deploy logs",
			Action: func(ctx context.Context, args []string) tea.Cmd {
				return InteractiveLogs(
					ctx,
					views.LogInput{
						ResourceIDs: []string{serviceID},
						StartTime:   startTime,
						EndTime:     endTime,
						Direction:   "forward",
					},
					"Logs",
				)
			},
		},
	}

	if deploy.IsCancellable(dep.Status) {
		commands = append(commands, views.PaletteCommand{
			Name:        "cancel",
			Description: "Cancel the deploy",
			Action: func(ctx context.Context, args []string) tea.Cmd {
				return InteractiveDeployCancel(
					ctx,
					views.DeployCancelInput{ServiceID: serviceID, DeployID: dep.Id},
					"Cancel deploy",
				)
			},
		})
	}

	return commands
}

func init() {
	deployCmd.AddCommand(deployListCmd)

	deployListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		serviceID := args[0]

		input := views.DeployListInput{ServiceID: serviceID}

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]*client.Deploy, error) {
			return views.LoadDeployList(cmd.Context(), input)
		}, text.DeployTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		r, err := resource.GetResource(cmd.Context(), serviceID)
		if err != nil {
			return err
		}

		InteractiveDeployList(cmd.Context(), input, "Deploys for "+resource.BreadcrumbForResource(r))
		return nil
	}
}
