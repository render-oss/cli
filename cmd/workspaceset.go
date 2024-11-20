package cmd

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/command"
	"github.com/renderinc/cli/pkg/text"
	"github.com/renderinc/cli/pkg/tui/views"
)

var workspaceSetCmd = &cobra.Command{
	Use:   "set [workspaceName|workspaceID]",
	Short: "Set the CLI's active workspace",
	Long: `Set the CLI's active workspace. All CLI commands run against the active workspace.

The active workspace is saved in a config file specified by the RENDER_CLI_CONFIG_PATH environment variable.
If unspecified, the config file is saved in $HOME/.render/cli.yaml.`,
}

var InteractiveWorkspaceSet = func(ctx context.Context, in views.ListWorkspaceInput) tea.Cmd {
	return command.AddToStackFunc(ctx, workspaceSetCmd, "Set Workspace", &in, views.NewWorkspaceView(ctx, in))
}

func init() {
	workspaceSetCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) >= 1 {
			workspaceIDOrName := args[0]
			return nonInteractiveSetWorkspace(cmd, workspaceIDOrName)
		}

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

func nonInteractiveSetWorkspace(cmd *cobra.Command, workspaceIDOrName string) error {
	o, err := views.SelectWorkspace(cmd.Context(), views.GetWorkspaceInput{IDOrName: workspaceIDOrName})
	if err != nil {
		return err
	}

	return printWorkspace(cmd, "Workspace set to", o)
}

type printableOwner struct {
	*client.Owner `json:"inline"`
	prefix        string
}

func (p *printableOwner) String() string {
	return fmt.Sprintf("%s: %s (%s)\n", p.prefix, p.Name, p.Id)
}

func printWorkspace(cmd *cobra.Command, prefix string, o *client.Owner) error {
	po := &printableOwner{
		Owner:  o,
		prefix: prefix,
	}

	_, err := command.PrintData(cmd, po, func(p *printableOwner) string {
		return text.FormatStringF("%s: %s (%s)", prefix, o.Name, o.Id)
	})
	return err
}
