package cmd

import (
	"context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/spf13/cobra"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/resource"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/renderinc/render-cli/pkg/types"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List and manage services, cron jobs, and postgres databases",
	Long: `List services, cron jobs, and postgres databases for the currently set workspace.
In interactive mode you can view logs, restart, deploy, SSH, and open PSQL terminals.`,
}

func optionallyAddCommand(commands []views.PaletteCommand, command views.PaletteCommand, allowedTypes []string, resource resource.Resource) []views.PaletteCommand {
	if len(allowedTypes) == 0 {
		return append(commands, command)
	}

	for _, allowedType := range allowedTypes {
		if resource.Type() == allowedType {
			return append(commands, command)
		}
	}

	return commands
}

func selectResource(ctx context.Context) func(resource.Resource) []views.PaletteCommand {
	return func(r resource.Resource) []views.PaletteCommand {
		type commandWithAllowedTypes struct {
			command      views.PaletteCommand
			allowedTypes []string
		}

		var commands []views.PaletteCommand
		commandWithTypes := []commandWithAllowedTypes{
			{
				command: views.PaletteCommand{
					Name:        "logs",
					Description: "View resource logs",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveLogs(ctx, views.LogInput{
							ResourceIDs: []string{r.ID()},
						}, "Logs")
					},
				},
			},
			{
				command: views.PaletteCommand{
					Name:        "restart",
					Description: "Restart the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveRestart(ctx, views.RestartInput{ResourceID: r.ID()}, "Restart")
					},
				},
			},
			{
				command: views.PaletteCommand{
					Name:        "psql",
					Description: "Connect to the PostgreSQL database",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractivePSQLView(ctx, &views.PSQLInput{PostgresID: r.ID()})
					},
				},
				allowedTypes: []string{postgres.PostgresType},
			},
			{
				command: views.PaletteCommand{
					Name:        "deploy create",
					Description: "Deploy the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveDeployCreate(ctx, types.DeployInput{ServiceID: r.ID()}, "Create Deploy")
					},
				},
				allowedTypes: service.Types,
			},
			{
				command: views.PaletteCommand{
					Name:        "deploy list",
					Description: "List deploys for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveDeployList(ctx, views.DeployListInput{ServiceID: r.ID()}, "List Deploys")
					},
				},
				allowedTypes: service.Types,
			},
			{
				command: views.PaletteCommand{
					Name:        "ssh",
					Description: "SSH into the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveSSHView(ctx, &views.SSHInput{ServiceID: r.ID()}, "SSH")
					},
				},
				allowedTypes: []string{
					service.WebServiceResourceType, service.PrivateServiceResourceType,
					service.BackgroundWorkerResourceType,
				},
			},
			{
				command: views.PaletteCommand{
					Name:        "jobs list",
					Description: "List jobs for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveJobList(ctx, views.JobListInput{ServiceID: r.ID()}, "List Jobs")
					},
				},
				allowedTypes: []string{
					service.WebServiceResourceType, service.PrivateServiceResourceType,
					service.BackgroundWorkerResourceType, service.CronJobResourceType,
				},
			},
			{
				command: views.PaletteCommand{
					Name:        "jobs create",
					Description: "Create a new job for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveJobCreate(ctx, &views.JobCreateInput{
							ServiceID:    r.ID(),
							StartCommand: pointers.From(""),
							PlanID:       pointers.From(""),
						}, resource.BreadcrumbForResource(r))
					},
				},
				allowedTypes: []string{
					service.WebServiceResourceType, service.PrivateServiceResourceType,
					service.BackgroundWorkerResourceType, service.CronJobResourceType,
				},
			},
		}

		for _, c := range commandWithTypes {
			commands = optionallyAddCommand(commands, c.command, c.allowedTypes, r)
		}

		// sort commands by name
		sort.Slice(commands, func(i, j int) bool {
			return commands[i].Name < commands[j].Name
		})

		return commands
	}
}

func InteractiveServices(ctx context.Context, in views.ListResourceInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(ctx, servicesCmd, breadcrumb, &in,
		views.NewResourceWithPaletteView(
			ctx,
			in,
			func(r resource.Resource) tea.Cmd {
				return InteractivePalette(ctx, selectResource(ctx)(r), resource.BreadcrumbForResource(r))
			},
			tui.WithCustomOptions[resource.Resource](getServiceTableOptions(ctx)),
		),
	)
}

func getServiceTableOptions(ctx context.Context) []tui.CustomOption {
	return []tui.CustomOption{
		{
			Key:   "w",
			Title: "Change Workspace",
			Function: func(row btable.Row) tea.Cmd {
				return InteractiveWorkspaceSet(ctx, views.ListWorkspaceInput{})
			},
		},
		{
			Key:   "f",
			Title: "Filter by Project",
			Function: func(row btable.Row) tea.Cmd {
				return command.AddToStackFunc(ctx, servicesCmd, "Project Filter", &views.ListResourceInput{},
					views.NewProjectFilterView(ctx, func(ctx context.Context, project *client.Project) tea.Cmd {
						listResourceInput := views.ListResourceInput{}
						breadcrumb := "All Projects"
						if project != nil {
							listResourceInput.Project = project
							listResourceInput.EnvironmentIDs = project.EnvironmentIds
							breadcrumb = project.Name
						}
						return InteractiveServices(ctx, listResourceInput, breadcrumb)
					}))
			},
		},
	}
}

func init() {
	rootCmd.AddCommand(servicesCmd)

	servicesCmd.RunE = func(cmd *cobra.Command, args []string) error {
		in := views.ListResourceInput{}
		err := command.ParseCommand(cmd, args, &in)
		if err != nil {
			return err
		}

		if nonInteractive, err := command.NonInteractive(cmd.Context(), cmd, func() (any, error) {
			return views.LoadResourceData(cmd.Context(), in)
		}, nil); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveServices(cmd.Context(), in, "Services")
		return nil
	}

	servicesCmd.Flags().StringSliceP("environment-ids", "e", nil, "Comma separated list of environment ids to filter by")
	servicesCmd.Flags().Bool("include-previews", false, "Whether to include preview environments when listing services")
}
