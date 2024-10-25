package cmd

import (
	"context"

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

var InteractiveSSHView = func(ctx context.Context, input *views.SSHInput) tea.Cmd {
	return command.AddToStackFunc(ctx, sshCmd, input, views.NewSSHView(ctx, input))
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

		InteractiveSSHView(ctx, &input)
		return nil
	}
}
