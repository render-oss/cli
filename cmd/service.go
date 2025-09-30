package cmd

import (
	"context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dashboard"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/resource"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/flows"
	"github.com/render-oss/cli/pkg/tui/views"
	workflowviews "github.com/render-oss/cli/pkg/tui/views/workflows"
	"github.com/render-oss/cli/pkg/types"
	"github.com/render-oss/cli/pkg/workflow"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage services and datastores",
	Long: `Manage services and datastores for the active workspace.
In interactive mode you can view logs, restart, deploy, SSH, and open PSQL sessions.`,
	GroupID: GroupCore.ID,
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
	// We should refactor this command to take the dependencies on construction
	// rather than getting them from the context
	deps := dependencies.GetFromContext(ctx)
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
					Description: "Tail resource logs",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return flows.NewLogFlow(deps).LogsFlow(ctx, views.LogInput{
							ResourceIDs: []string{r.ID()},
							Tail:        true,
						})
					},
				},
				allowedTypes: append([]string{postgres.PostgresType, keyvalue.KeyValueType}, service.NonStaticTypes...),
			},
			{
				command: views.PaletteCommand{
					Name:        "restart",
					Description: "Restart the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveRestart(ctx, views.RestartInput{ResourceID: r.ID()}, "Restart")
					},
				},
				allowedTypes: append([]string{postgres.PostgresType}, service.NonStaticServerTypes...),
			},
			{
				command: views.PaletteCommand{
					Name:        "kv-cli",
					Description: "Connect to the Key Value using either redis-cli or valkey-cli",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveKeyValueCLIView(ctx, &views.RedisCLIInput{RedisIDOrName: r.ID()})
					},
				},
				allowedTypes: []string{keyvalue.KeyValueType},
			},
			{
				command: views.PaletteCommand{
					Name:        "psql",
					Description: "Connect to the PostgreSQL database using psql",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractivePSQLView(ctx, &views.PSQLInput{PostgresIDOrName: r.ID()})
					},
				},
				allowedTypes: []string{postgres.PostgresType},
			},
			{
				command: views.PaletteCommand{
					Name:        "pgcli",
					Description: "Connect to the PostgreSQL database using pgcli",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractivePGCLIView(ctx, &views.PSQLInput{PostgresIDOrName: r.ID()})
					},
				},
				allowedTypes: []string{postgres.PostgresType},
			},
			{
				command: views.PaletteCommand{
					Name:        "deploys create",
					Description: "Deploy the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveDeployCreate(ctx, types.DeployInput{ServiceID: r.ID()}, "Create Deploy")
					},
				},
				allowedTypes: service.Types,
			},
			{
				command: views.PaletteCommand{
					Name:        "deploys list",
					Description: "List deploys for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveDeployList(ctx, views.DeployListInput{ServiceID: r.ID()}, r, "Deploys")
					},
				},
				allowedTypes: service.Types,
			},
			{
				command: views.PaletteCommand{
					Name:        "versions list",
					Description: "List versions for the workflow",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return flows.NewWorkflow(deps, flows.NewLogFlow(deps), false).VersionList(ctx, &workflowviews.VersionListInput{WorkflowID: r.ID()})
					},
				},
				allowedTypes: []string{workflow.WorkflowType},
			},
			{
				command: views.PaletteCommand{
					Name:        "versions release",
					Description: "Release a new version of the workflow",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return flows.NewWorkflow(deps, flows.NewLogFlow(deps), false).VersionRelease(ctx, &workflowviews.VersionReleaseInput{WorkflowID: r.ID()})
					},
				},
				allowedTypes: []string{workflow.WorkflowType},
			},
			{
				command: views.PaletteCommand{
					Name:        "ssh",
					Description: "SSH into the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveSSHView(ctx, &views.SSHInput{ServiceIDOrName: r.ID()}, "SSH")
					},
				},
				allowedTypes: service.NonStaticServerTypes,
			},
			{
				command: views.PaletteCommand{
					Name:        "jobs list",
					Description: "List jobs for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveJobList(ctx, views.JobListInput{ServiceID: r.ID()}, "Jobs")
					},
				},
				allowedTypes: service.NonStaticTypes,
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
				allowedTypes: service.NonStaticTypes,
			},
			{
				command: views.PaletteCommand{
					Name:        "dashboard",
					Description: "Open Render Dashboard to the service's page",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						err := dashboard.OpenResource(r.ID(), r.Type())
						return command.AddErrToStack(ctx, servicesCmd, err)
					},
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
	deps := dependencies.GetFromContext(ctx)
	return command.AddToStackFunc(ctx, servicesCmd, breadcrumb, &in,
		views.NewResourceWithPaletteView(
			ctx,
			in,
			deps.ResourceLoader().LoadResourceData,
			func(r resource.Resource) tea.Cmd {
				return InteractivePalette(ctx, selectResource(ctx)(r), resource.BreadcrumbForResource(r))
			},
			tui.WithCustomOptions[resource.Resource](getServiceTableOptions(ctx)),
		),
	)
}

func getServiceTableOptions(ctx context.Context) []tui.CustomOption {
	return []tui.CustomOption{
		flows.WithCopyID(ctx, servicesCmd),
		flows.WithWorkspaceSelection(ctx),
		flows.WithProjectFilter(ctx, servicesCmd, "Project Filter", &views.ListResourceInput{}, func(ctx context.Context, project *client.Project) tea.Cmd {
			listResourceInput := views.ListResourceInput{}
			breadcrumb := "All Projects"
			if project != nil {
				listResourceInput.Project = project
				listResourceInput.EnvironmentIDs = project.EnvironmentIds
				breadcrumb = project.Name
			}
			return InteractiveServices(ctx, listResourceInput, breadcrumb)
		}),
	}
}

func init() {
	rootCmd.AddCommand(servicesCmd)

	servicesCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if err := checkForDeprecatedFlagUsage(cmd); err != nil {
			return err
		}

		in := views.ListResourceInput{}
		err := command.ParseCommand(cmd, args, &in)
		if err != nil {
			return err
		}

		deps := dependencies.GetFromContext(cmd.Context())
		if nonInteractive, err := command.NonInteractive(cmd, func() ([]resource.Resource, error) {
			return deps.ResourceLoader().LoadResourceData(cmd.Context(), in)
		}, text.ResourceTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		InteractiveServices(cmd.Context(), in, "Services")
		return nil
	}

	servicesCmd.Flags().StringSliceP("environment-ids", "e", nil, "Comma separated list of environment ids to filter by")
	servicesCmd.Flags().Bool("include-previews", false, "Whether to include preview environments when listing services")

	// Flags from the old CLI that we error with a helpful message
	servicesCmd.Flags().String("service-id", "", "")
	if err := servicesCmd.Flags().MarkHidden("service-id"); err != nil {
		panic(err)
	}
}
