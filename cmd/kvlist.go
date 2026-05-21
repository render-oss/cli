package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

var kvListCmd = &cobra.Command{
	Use:          "list",
	Short:        "List Key Value store instances",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	Long: `List Key Value store instances in the active workspace.

Use --project to narrow results to a single project, --environment to narrow
to a single environment, or both — when both are supplied, the environment is
resolved within that project.`,
	Example: `  # List all Key Value instances in the active workspace
  render ea kv list

  # List all Key Value instances in a project
  render ea kv list --project my-project

  # Filter by environment name
  render ea kv list --environment production

  # Disambiguate an environment name by project
  render ea kv list --project my-project --environment production

  # JSON output
  render ea kv list --output json`,
}

func init() {
	kvListCmd.Flags().String("project", "",
		"Project ID or name (optional). Narrows results to environments in this project.")
	kvListCmd.Flags().String("environment", "",
		"Environment ID or name (optional). Narrows results to this environment.")

	kvListCmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeListInput(input)

		_, err := command.NonInteractive(cmd, func() ([]*keyvalue.Model, error) {
			params := &client.ListKeyValueParams{}

			envIDs, ok, err := resolveListEnvIDs(cmd, input)
			if err != nil {
				return nil, err
			}
			if ok {
				if len(envIDs) == 0 {
					return nil, nil
				}
				envParam := client.EnvironmentIdParam(envIDs)
				params.EnvironmentId = &envParam
			}

			return keyvalue.List(cmd.Context(), params)
		}, text.KeyValueTable)
		return err
	}

	kvCmd.AddCommand(kvListCmd)
}

// resolveListEnvIDs translates --project/--environment selectors into the
// environment IDs to filter on. The second return value is true when the
// caller should apply an environment filter; false means list workspace-wide.
func resolveListEnvIDs(cmd *cobra.Command, input kvtypes.KeyValueListInput) ([]string, bool, error) {
	if input.ProjectIDOrName == nil && input.EnvironmentIDOrName == nil {
		return nil, false, nil
	}

	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, false, err
	}
	scope, err := resolve.New(c).ResolveScopeInActiveWorkspace(cmd.Context(), resolve.ActiveWorkspaceScopeInput{
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
	if err != nil {
		return nil, false, err
	}
	if scope.Environment != nil {
		return []string{scope.Environment.Id}, true, nil
	}
	return scope.Project.EnvironmentIds, true, nil
}
