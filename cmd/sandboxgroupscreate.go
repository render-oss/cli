package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/sandboxgroup"
	"github.com/render-oss/cli/pkg/text"
)

type SandboxGroupsCreateInput struct {
	Name                string  `cli:"name"`
	EnvironmentIDOrName *string `cli:"environment"`
}

func (i *SandboxGroupsCreateInput) Validate(_ bool) error {
	if i.Name == "" {
		return fmt.Errorf("--name is required")
	}
	return nil
}

func newSandboxGroupsCreateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "create",
		Short:        "Create a sandbox group in the active workspace",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		Long: `Create a sandbox group in your workspace.

Group names are unique within a workspace. The workspace's first group becomes
its default, where "render sandboxes create" places sandboxes unless
--group is given.

Use --environment to create the group in a project environment. Without it,
the group isn't in a project.`,
		Example: `  # Create a sandbox group
  render sandbox-groups create --name agents

  # Create a sandbox group in a project environment
  render sandbox-groups create --name agents --environment production

  # Output as JSON
  render sandbox-groups create --name agents -o json`,
	}

	cmd.Flags().String("name", "", "Name for the group, unique within the workspace")
	cmd.Flags().String("environment", "",
		"Set the environment to create the group in (ID or name, optional). Example: production or evm-abc123def456")
	setAllFlagPlaceholders(cmd, map[string]string{
		"name":        "NAME",
		"environment": "ENVIRONMENT",
	})

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		command.DefaultFormatNonInteractive(cmd)

		var input SandboxGroupsCreateInput
		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return err
		}

		_, err := command.NonInteractive(cmd, func() (*sandboxesclient.SandboxGroup, error) {
			return deps.SandboxGroupService().Create(cmd.Context(), sandboxgroup.CreateInput{
				Name:                input.Name,
				EnvironmentIDOrName: input.EnvironmentIDOrName,
			})
		}, text.SandboxGroupDetail)
		return err
	}

	return cmd
}
