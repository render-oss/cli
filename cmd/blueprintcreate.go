package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/text"
)

func newBlueprintCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create a Blueprint and plan its first sync",
		Args:         cobra.NoArgs,
		Hidden:       true,
		SilenceUsage: true,
		Long: `Create a Blueprint from a remote repository and plan its first sync in the active workspace.

Without --repo, the command uses the URL of the local Git repository's remote
named "origin" and the current local branch.

With --repo, the command uses that repository and its default branch. The
repository URL can be HTTPS or SSH.

In either case, --branch overrides the branch. If the local checkout has a
detached HEAD, provide --branch or --repo.

--path is relative to the repository root, not the current directory, and
defaults to render.yaml. Render reads the pushed repository, so push local
changes first.

Creating a Blueprint does not approve its first sync. With --auto-sync enabled,
future pushes sync automatically after the first approval. Existing resources
are left unchanged unless --existing-resources=adopt is supplied.

When a successful plan requires manual values, this command creates a file for
manually-supplied environment variables at
` + blueprint.ManualEnvVarsFileHelpPath + `. Values start as empty strings;
you can edit the file before approving the sync. Existing manual env var files
are replaced. Set RENDER_CLI_CONFIG_DIR to use a different config directory.

Output defaults to text. Use --output json or yaml for structured responses.`,
		Example: `  # Use the local remote named "origin" and the current branch
  render ea blueprints create

  # Use a remote repository and its default branch
  render ea blueprints create --repo https://github.com/acme/my-app

  # Set the branch and repository-relative Blueprint path
  render ea blueprints create --repo https://github.com/acme/my-app --branch release --path infra/render.yaml

  # Explicitly adopt matching resources and disable future automatic syncs
  render ea blueprints create --repo https://github.com/acme/my-app --existing-resources adopt --auto-sync=false

  # Output the Blueprint and planning result as JSON
  render ea blueprints create --repo https://github.com/acme/my-app --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			command.DefaultFormatNonInteractive(cmd)
			var input blueprint.CreateInput
			if err := command.ParseCommand(cmd, args, &input); err != nil {
				return err
			}
			workspaceID, err := config.WorkspaceID()
			if err != nil {
				return err
			}
			var result *bptypes.CreateBlueprintResponse
			var fileErr error
			_, err = command.NonInteractive(cmd,
				func() (blueprint.CreateOut, error) {
					var err error
					result, err = deps.BlueprintService().Create(cmd.Context(), workspaceID, input)
					if err != nil {
						return blueprint.CreateOut{}, err
					}
					filePath, err := blueprint.PrepareManualEnvVarsFile(result.Blueprint.Id, result.PlanningOutcome, result.Sync)
					if err != nil {
						// Print the saved Blueprint result before reporting a local file error.
						fileErr = err
						return blueprint.CreateOut{Data: result}, nil
					}
					return blueprint.NewCreateOut(result, filePath), nil
				},
				text.BlueprintCreate,
			)
			if err != nil {
				return err
			}
			if fileErr != nil {
				return fileErr
			}
			if result == nil {
				return errors.New("create blueprint: command completed without a creation result")
			}
			return exitForPlanningOutcome(cmd, result.PlanningOutcome)
		},
	}

	cmd.Flags().String("repo", "", `Set the repository URL (defaults to the local remote named "origin")`)
	cmd.Flags().String("branch", "", "Set the branch (defaults to the local branch, or the repository's default branch with --repo)")
	cmd.Flags().String("path", "render.yaml", "Set the Blueprint file path relative to the repository root")
	cmd.Flags().String("name", "", "Set the Blueprint name (defaults to the repository name)")
	cmd.Flags().Bool("auto-sync", true, "Automatically sync future pushes after the first approval")
	cmd.Flags().String("existing-resources", string(bptypes.CreateNew), "Set how to handle existing resources: create_new or adopt")
	setAllFlagPlaceholders(cmd, map[string]string{
		"repo":               "REPO",
		"branch":             "BRANCH",
		"path":               "PATH",
		"name":               "NAME",
		"existing-resources": "EXISTING_RESOURCE_MODE",
	})
	return cmd
}
