package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/types"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

func newPgUpdateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "update <postgresID|postgresName>",
		Short:        "Update a Render Postgres database",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Long: `Update an existing Render Postgres database.

The positional argument is the target database (ID dpg-... or name). At least
one mutating flag must be supplied. Use --name to rename the database; the
positional argument always identifies the target and is never the new name.

Environment, project, workspace, and region are immutable. A database cannot be
moved between them; the --project and --environment flags are for name
disambiguation only.

Only the fields you pass are changed; everything else is left untouched.

Name lookup is scoped to your active workspace. If a name isn't found, switch
workspaces with 'render workspace set <name|ID>' and try again, or pass the
Postgres ID instead (which works across workspaces). If a name matches more
than one database, narrow the search with --project <id|name> or
--environment <id|name>.

The --ip-allow-list flag replaces the server-side list; pass it once per entry.
To remove all allow-list entries, pass --clear-ip-allow-list. The two flags are
mutually exclusive.`,
		Example: `  # Rename
  render ea pg update dpg-abc123def456ghi789jkl0 --name application_db

  # Change plan
  render ea pg update my-db --plan pro_4gb

  # Grow the disk and enable autoscaling
  render ea pg update my-db --disk-size-gb 50 --disk-autoscaling

  # Replace the IP allow-list (entire list, not append)
  render ea pg update my-db \
    --ip-allow-list "cidr=203.0.113.5/32,description=office" \
    --ip-allow-list "cidr=10.0.0.0/8,description=internal"

  # Clear the IP allow-list
  render ea pg update my-db --clear-ip-allow-list

  # Disambiguate a name that exists in multiple environments
  render ea pg update my-db --environment production --plan pro_8gb

  # JSON output
  render ea pg update dpg-abc123def456ghi789jkl0 --plan pro_4gb --output json`,
	}

	cmd.Flags().String("project", "",
		"Narrow lookup to a project (ID or name, optional) when the same Postgres database name exists in multiple projects.")
	cmd.Flags().String("environment", "",
		"Narrow lookup to an environment (ID or name, optional) when the same Postgres database name exists in multiple environments.")

	cmd.Flags().String("name", "", "Rename the database")
	cmd.Flags().String("plan", "", "Set the plan to one of: "+strings.Join(postgres.ModernPlans, " | ")+". Custom enterprise plan names are also accepted.")

	cmd.Flags().Int("disk-size-gb", 0, "Set the disk size in GB. Must be 1 or a multiple of 5.")
	cmd.Flags().Bool("disk-autoscaling", false, "Enable disk autoscaling. Pass --disk-autoscaling=false to disable.")
	cmd.Flags().Bool("high-availability", false, "Enable high availability (Pro plans and above). Pass --high-availability=false to disable.")

	cmd.Flags().String("datadog-api-key", "", "Set the Datadog API key for monitoring. Pass an empty string to remove.")
	cmd.Flags().String("datadog-site", "", "Set the Datadog region/site (e.g. US1, US3, EU)")

	cmd.Flags().StringArray("ip-allow-list", nil,
		"Replace the IP allow-list with the supplied entries (format: cidr=<range>,description=<label>). Repeat the flag for multiple entries.")
	cmd.Flags().Bool("clear-ip-allow-list", false,
		"Remove all IP allow-list entries. Mutually exclusive with --ip-allow-list")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// No interactive flow yet; collapse --output interactive onto text so
		// the standard NonInteractive path handles every format.
		command.DefaultFormatNonInteractive(cmd)

		var input pgtypes.UpdatePostgresInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input.ProjectIDOrName = types.OptionalNonZeroString(input.ProjectIDOrName)
		input.EnvironmentIDOrName = types.OptionalNonZeroString(input.EnvironmentIDOrName)

		_, err := command.NonInteractive(cmd,
			func() (*postgres.PostgresUpdateOut, error) {
				result, err := deps.PostgresService().Update(cmd.Context(), input)
				if err != nil {
					return nil, err
				}
				out := postgres.NewPostgresUpdateOut(result.Before, result.After)
				return &out, nil
			},
			func(out *postgres.PostgresUpdateOut) string {
				return pgUpdateSuccessMessage(out)
			},
		)
		return err
	}

	return cmd
}

func pgUpdateSuccessMessage(out *postgres.PostgresUpdateOut) string {
	details := "Full details:\n  " + strings.ReplaceAll(text.PostgresDetail(&out.Data), "\n", "\n  ")
	diff := text.PostgresUpdateDiff(out.Diff)
	if diff == "" {
		return fmt.Sprintf("No changes applied to Postgres database\n\n%s\n", details)
	}
	return fmt.Sprintf("Updated Postgres database\n\nChanges:\n%s\n\n%s\n", diff, details)
}
