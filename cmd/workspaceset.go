package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui/views"
	"github.com/spf13/cobra"
)

var workspaceSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Select a workspace to run commands against",
	Long: `Select a workspace to run commands against.
Your specified workspace will be saved in a config file specified by the RENDER_CLI_CONFIG_PATH environment variable.
If unspecified, the config file will be saved in $HOME/.render/cli.yaml. All subsequent commands will run against this workspace.

Currently, you can only select a workspace in interactive mode.`,
}

var InteractiveWorkspaceSet = func(ctx context.Context, in views.ListWorkspaceInput) tea.Cmd {
	return command.AddToStackFunc(ctx, workspaceSetCmd, "Set Workspace", &in, views.NewWorkspaceView(ctx, in))
}

func init() {
	workspaceSetCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var input views.ListWorkspaceInput
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}
		InteractiveWorkspaceSet(cmd.Context(), input)
		return nil
	}

	workspaceCmd.AddCommand(workspaceSetCmd)
}
