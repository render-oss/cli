package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/tui/views"
	"github.com/render-oss/cli/pkg/types"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

func newPgCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create a new Postgres database",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `Create a new Postgres database on Render.

In interactive mode, a wizard guides you through the core choices for the
database. The wizard owns those prompted values. Flag-only settings, such as
--disk-size-gb, --database-name, --database-user, --ip-allow-list,
--parameter-override, and --read-replica, are still included in the create
request.

Use --confirm to skip the wizard and create immediately from flags and defaults.
When --confirm is used with the default interactive output mode, output is
printed as text. Use --output json, yaml, or text for non-interactive output.

Examples:
  # Launch the interactive wizard
  render ea pg create

  # Create immediately with defaults and text output
  render ea pg create --confirm

  # Create immediately with explicit values
  render ea pg create --confirm --name analytics --plan pro_8gb --version 17 --region ohio

  # Include flag-only settings while using the wizard for prompted values
  render ea pg create \
    --ip-allow-list "cidr=203.0.113.5/32,description=office" \
    --ip-allow-list "cidr=10.0.0.0/8,description=internal"

  # Machine-readable output
  render ea pg create --output json`,
	}

	cmd.Flags().String("name", "", "Database name (generated if not provided)")
	cmd.Flags().String("workspace", "", "Workspace ID or name. Defaults to the active workspace (set via 'render workspace set').")
	cmd.Flags().String("project", "", "Project ID or name (optional). Scopes environment lookup; if the project has exactly one environment it is used automatically.")
	cmd.Flags().String("environment", "", "Environment ID or name (optional). Example: Production or evm-abc123def456")

	cmd.Flags().String("plan", "", "Plan name. Examples: "+strings.Join(postgres.ModernPlans, " | "))
	cmd.Flags().Int("version", 0, fmt.Sprintf("Postgres major version. Defaults to %d.", postgres.DefaultPostgresVersion))

	regionFlag := command.NewEnumInput(types.RegionValues(), false)
	cmd.Flags().Var(regionFlag, "region", "Region: "+strings.Join(types.RegionValues(), " | ")+" (server picks if unset)")

	cmd.Flags().String("database-name", "", "Postgres database name (server generates one if unset)")
	cmd.Flags().String("database-user", "", "Postgres database user (server generates one if unset)")

	cmd.Flags().Int("disk-size-gb", 0, "Disk size in GB. Must be 1 or a multiple of 5. Server picks a sensible default based on compute size if unset.")
	cmd.Flags().Bool("disk-autoscaling", false, "Enable disk autoscaling")
	cmd.Flags().Bool("high-availability", false, "Enable high availability (Pro plans and above)")

	cmd.Flags().String("datadog-api-key", "", "Datadog API key for monitoring")
	cmd.Flags().String("datadog-site", "", "Datadog region/site (e.g. US1, US3, EU). Server default is US1.")

	cmd.Flags().StringArray("ip-allow-list", nil,
		"Restrict inbound traffic to specific IP ranges. Repeat the flag for multiple entries.\n"+
			"Format: cidr=<range>,description=<label>")
	cmd.Flags().StringArray("parameter-override", nil,
		"Override a Postgres server parameter. Repeat the flag for multiple entries.\n"+
			"Format: KEY=VALUE\n"+
			"Example: --parameter-override max_connections=100")
	cmd.Flags().StringArray("read-replica", nil,
		"Name of a read replica to create alongside the primary. Repeat the flag for multiple replicas.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if command.GetConfirmFromContext(cmd.Context()) {
			command.DefaultFormatNonInteractive(cmd)
		}

		var input pgtypes.CreatePostgresInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		// There are two execution paths:
		//  1. Non-interactive output (text/json/yaml): create and print via the shared formatter.
		//  2. Interactive output: run the TUI wizard, which owns its styled output.
		// --confirm skips prompts, so collapse default interactive output to text before
		// this gate. command.NonInteractive returns (false, nil) only when the resolved
		// output format is still interactive, without calling loadData.
		nonInteractive, err := command.NonInteractive(cmd,
			func() (*client.PostgresDetail, error) {
				return deps.PostgresService().Create(cmd.Context(), input)
			},
			func(pg *client.PostgresDetail) string {
				return pgCreateSuccessMessage(pg)
			},
		)
		if err != nil || nonInteractive {
			return err
		}

		repos := views.PostgresCreateRepos{
			Owners:   deps.OwnerRepo(),
			Projects: deps.ProjectRepo(),
			Envs:     deps.EnvironmentRepo(),
			Postgres: deps.PostgresRepo(),
		}
		_, err = views.RunPostgresCreate(cmd, repos, input)
		return err
	}

	return cmd
}

func pgCreateSuccessMessage(pg *client.PostgresDetail) string {
	return fmt.Sprintf(
		"Created Postgres database\n\n%s\n",
		text.PostgresDetail(pg),
	)
}
