package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/text"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

func newKVListCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Short:        "List Render Key Value instances",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `List Render Key Value instances in the active workspace.

Use --project to narrow results to a single project, --environment to narrow
to a single environment, or both — when both are supplied, the environment is
resolved within that project.`,
		Example: `  # List all Key Value instances in the active workspace
  render kv list

  # List all Key Value instances in a project
  render kv list --project my-project

  # Filter by environment name
  render kv list --environment production

  # Disambiguate an environment name by project
  render kv list --project my-project --environment production

  # JSON output
  render kv list --output json`,
	}

	cmd.Flags().String("project", "",
		"Narrow results to environments in a project (ID or name, optional).")
	cmd.Flags().String("environment", "",
		"Narrow results to a single environment (ID or name, optional).")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input kvtypes.KeyValueListInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}
		input = kvtypes.NormalizeListInput(input)

		_, err := command.NonInteractive(cmd, func() (*keyvalue.KeyValueListOut, error) {
			params := &client.ListKeyValueParams{}

			envIDs, ok, err := resolveListEnvIDs(cmd.Context(), deps, input)
			if err != nil {
				return nil, err
			}
			if ok {
				if len(envIDs) == 0 {
					out := keyvalue.NewKeyValueListOut(nil)
					return &out, nil
				}
				envParam := client.EnvironmentIdParam(envIDs)
				params.EnvironmentId = &envParam
			}

			models, err := deps.KeyValueService().ListKeyValue(cmd.Context(), params)
			if err != nil {
				return nil, err
			}
			out := keyvalue.NewKeyValueListOut(models)
			return &out, nil
		}, func(out *keyvalue.KeyValueListOut) string {
			return text.KeyValueTable(out.Data)
		})
		return err
	}

	return cmd
}

// resolveListEnvIDs translates --project/--environment selectors into the
// environment IDs to filter on. The second return value is true when the
// caller should apply an environment filter; false means list workspace-wide.
func resolveListEnvIDs(ctx context.Context, deps *dependencies.Dependencies, input kvtypes.KeyValueListInput) ([]string, bool, error) {
	if input.ProjectIDOrName == nil && input.EnvironmentIDOrName == nil {
		return nil, false, nil
	}

	scope, err := deps.Resolver().ResolveScopeInActiveWorkspace(ctx, resolve.ActiveWorkspaceScopeInput{
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
