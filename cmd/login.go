package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/tui/views"
)

var loginCmd = newLoginCmd()

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to Render using the Render Dashboard",
		Example: `  # Authenticate with Render
  render login`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return views.NonInteractiveLogin(cmd)
		},
		GroupID: GroupAuth.ID,
	}

	markCommandAnalyticsIneligible(cmd)

	return cmd
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
