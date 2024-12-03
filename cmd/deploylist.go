package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/dashboard"
	"github.com/renderinc/cli/pkg/deploy"
	"github.com/renderinc/cli/pkg/pointers"
	"github.com/renderinc/cli/pkg/resource"
	"github.com/renderinc/cli/pkg/text"
	"github.com/renderinc/cli/pkg/tui/views"
)

var deployListCmd = &cobra.Command{
	Use:   "list [serviceID]",
	Short: "List deploys for a service",
	Args:  cobra.MaximumNArgs(1),
}

var InteractiveDeployList = func(ctx context.Context, input views.DeployListInput, r resource.Resource, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, deployListCmd, breadcrumb, &input, views.NewDeployListView(
		ctx,
		input,
		func(c *client.Deploy) tea.Cmd {
			return InteractivePalette(ctx, commandsForDeploy(c, r.ID(), r.Type()), c.Id)
		},
	))
}

func interactiveDeployList(cmd *cobra.Command, input views.DeployListInput) tea.Cmd {
	ctx := cmd.Context()
	if input.ServiceID == "" {
		return command.AddToStackFunc(
			ctx,
			cmd,
			"Deploys",
			&input,
			views.NewServiceList(ctx, views.ServiceInput{}, func(ctx context.Context, r resource.Resource) tea.Cmd {
				input.ServiceID = r.ID()
				return InteractiveDeployList(ctx, input, r, resource.BreadcrumbForResource(r))
			}),
		)
	}

	service, err := resource.GetResource(ctx, input.ServiceID)
	if err != nil {
		command.Fatal(cmd, err)
	}

	return InteractiveDeployList(ctx, input, service, "Deploys for "+resource.BreadcrumbForResource(service))
}

func commandsForDeploy(dep *client.Deploy, serviceID, serviceType string) []views.PaletteCommand {
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
		{
			Name:        "dashboard",
			Description: "Open Render Dashboard to the service's page",
			Action: func(ctx context.Context, args []string) tea.Cmd {
				err := dashboard.OpenDeploy(serviceID, serviceType, dep.Id)
				return command.AddErrToStack(ctx, servicesCmd, err)
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
		var input views.DeployListInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]*client.Deploy, error) {
			_, res, err := views.LoadDeployList(cmd.Context(), input, "")
			return res, err
		}, text.DeployTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		interactiveDeployList(cmd, input)
		return nil
	}
}
