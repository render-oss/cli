package cmd

import (
	"context"

	btable "github.com/evertras/bubble-table/table"
	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/service"
	"github.com/renderinc/render-cli/pkg/tui"
	"github.com/renderinc/render-cli/pkg/tui/views"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/renderinc/render-cli/pkg/command"
	"github.com/spf13/cobra"
)

// sshCmd represents the ssh command
var sshCmd = &cobra.Command{
	Use:   "ssh [serviceID]",
	Args:  cobra.MaximumNArgs(1),
	Short: "SSH into a server",
	Long:  `SSH into a server given a service ID. Optionally pass the service id as an argument.`,
}

func InteractiveSSHView(ctx context.Context, input *views.SSHInput, breadcrumb string) tea.Cmd {
	return command.AddToStackFunc(
		ctx,
		sshCmd,
		breadcrumb,
		input,
		views.NewSSHView(ctx, input, tui.WithCustomOptions[*service.Model](getSSHTableOptions(ctx, breadcrumb))),
	)
}

func getSSHTableOptions(ctx context.Context, breadcrumb string) []tui.CustomOption {
	return []tui.CustomOption{
		{
			Key:   "w",
			Title: "Change Workspace",
			Function: func(row btable.Row) tea.Cmd {
				return InteractiveWorkspaceSet(ctx, views.ListWorkspaceInput{})
			},
		},
		{
			Key:   "f",
			Title: "Filter by Project",
			Function: func(row btable.Row) tea.Cmd {
				return command.AddToStackFunc(ctx, servicesCmd, "Project Filter", &views.SSHInput{},
					views.NewProjectFilterView(ctx, func(ctx context.Context, project *client.Project) tea.Cmd {
						input := views.SSHInput{}
						if project != nil {
							input.Project = project
							input.EnvironmentIDs = project.EnvironmentIds
						}
						return InteractiveSSHView(ctx, &input, breadcrumb)
					}))
			},
		},
	}
}

func init() {
	rootCmd.AddCommand(sshCmd)

	sshCmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		input := views.SSHInput{}
		err := command.ParseCommand(cmd, args, &input)
		if err != nil {
			return err
		}

		InteractiveSSHView(ctx, &input, "SSH")
		return nil
	}
}
