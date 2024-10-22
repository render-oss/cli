package cmd

import (
	"context"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/pointers"
	"github.com/renderinc/render-cli/pkg/postgres"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/resource"
	resourcetui "github.com/renderinc/render-cli/pkg/resource/tui"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/types"
	"github.com/spf13/cobra"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List and manage services, cron jobs, and postgres databases",
	Long: `List services, cron jobs, and postgres databases for the currently set workspace.
In interactive mode you can view logs, restart, deploy, SSH, and open PSQL terminals.`,
}

var InteractiveServices = command.Wrap(servicesCmd, loadResourceData, renderResources)

func loadResourceData(ctx context.Context, in ListResourceInput) ([]resource.Resource, error) {
	resourceService, err := newResourceService()
	if err != nil {
		return nil, err
	}
	return resourceService.ListResources(ctx, in.ToParams())
}

type ListResourceInput struct {
	EnvironmentID string `cli:"environment"`
}

func (l ListResourceInput) ToParams() resource.ResourceParams {
	return resource.ResourceParams{
		EnvironmentID: l.EnvironmentID,
	}
}

func renderResources(ctx context.Context, loadData func(input ListResourceInput) ([]resource.Resource, error), in ListResourceInput) (tea.Model, error) {
	columns := resourcetui.ColumnsForResources()

	loadDataFunc := func() ([]resource.Resource, error) {
		return loadData(in)
	}

	createRowFunc := func(r resource.Resource) btable.Row {
		return resourcetui.RowForResource(r)
	}

	onSelect := func(rows []btable.Row) tea.Cmd {
		if len(rows) == 0 {
			return nil
		}

		r, ok := rows[0].Data["resource"].(resource.Resource)
		if !ok {
			return nil
		}

		return selectResource(ctx)(r)
	}

	customOptions := []tui.CustomOption{
		{
			Key:   "w",
			Title: "Change Workspace",
			Function: func(row btable.Row) tea.Cmd {
				return InteractiveWorkspace(ctx, ListWorkspaceInput{})
			},
		},
	}

	t := tui.NewTable(
		columns,
		loadDataFunc,
		createRowFunc,
		onSelect,
		tui.WithCustomOptions[resource.Resource](customOptions),
	)

	return t, nil
}

func optionallyAddCommand(commands []PaletteCommand, command PaletteCommand, allowedTypes []string, resource resource.Resource) []PaletteCommand {
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

func selectResource(ctx context.Context) func(resource.Resource) tea.Cmd {
	return func(r resource.Resource) tea.Cmd {
		type commandWithAllowedTypes struct {
			command      PaletteCommand
			allowedTypes []string
		}

		var commands []PaletteCommand
		commandWithTypes := []commandWithAllowedTypes{
			{
				command: PaletteCommand{
					Name:        "logs",
					Description: "View resource logs",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveLogs(ctx, LogInput{
							ResourceIDs: []string{r.ID()},
						})
					},
				},
			},
			{
				command: PaletteCommand{
					Name:        "restart",
					Description: "Restart the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveRestart(ctx, RestartInput{ResourceID: r.ID()})
					},
				},
			},
			{
				command: PaletteCommand{
					Name:        "psql",
					Description: "Connect to the PostgreSQL database",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractivePSQL(ctx, PSQLInput{PostgresID: r.ID()})
					},
				},
				allowedTypes: []string{postgres.PostgresType},
			},
			{
				command: PaletteCommand{
					Name:        "deploy create",
					Description: "Deploy the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveDeployCreate(ctx, types.DeployInput{ServiceID: r.ID()})
					},
				},
				allowedTypes: service.Types,
			},
			{
				command: PaletteCommand{
					Name:        "deploy list",
					Description: "List deploys for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveDeployList(ctx, DeployListInput{ServiceID: r.ID()})
					},
				},
				allowedTypes: service.Types,
			},
			{
				command: PaletteCommand{
					Name:        "ssh",
					Description: "SSH into the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveSSH(ctx, SSHInput{ServiceID: r.ID()})
					},
				},
				allowedTypes: []string{
					service.WebServiceResourceType, service.PrivateServiceResourceType,
					service.BackgroundWorkerResourceType,
				},
			},
			{
				command: PaletteCommand{
					Name:        "jobs list",
					Description: "List jobs for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveJobList(ctx, JobListInput{ServiceID: r.ID()})
					},
				},
				allowedTypes: []string{
					service.WebServiceResourceType, service.PrivateServiceResourceType,
					service.BackgroundWorkerResourceType, service.CronJobResourceType,
				},
			},
			{
				command: PaletteCommand{
					Name:        "jobs create",
					Description: "List jobs for the service",
					Action: func(ctx context.Context, args []string) tea.Cmd {
						return InteractiveJobCreate(ctx, JobCreateInput{
							ServiceID:    r.ID(),
							StartCommand: pointers.From(""),
							PlanID:       pointers.From(""),
						},)
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

		return InteractiveCommandPalette(ctx, PaletteCommandInput{
			Commands: commands,
		})
	}
}

func newResourceService() (*resource.Service, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}

	serviceRepo := service.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	postgresRepo := postgres.NewRepo(c)

	serviceService := service.NewService(serviceRepo, environmentRepo, projectRepo)
	postgresService := postgres.NewService(postgresRepo, environmentRepo, projectRepo)

	resourceService := resource.NewResourceService(
		serviceService,
		postgresService,
		environmentRepo,
		projectRepo,
	)

	return resourceService, nil
}

func init() {
	rootCmd.AddCommand(servicesCmd)

	servicesCmd.RunE = func(cmd *cobra.Command, args []string) error {
		in := ListResourceInput{}
		err := command.ParseCommand(cmd, args, &in)
		if err != nil {
			return err
		}

		InteractiveServices(cmd.Context(), in)
		return nil
	}

	servicesCmd.Flags().StringP("environment", "e", "", "Comma separated list of environment ids to filter by")
}
