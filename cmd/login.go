package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/tui/views"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Render using the dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return views.NonInteractiveLogin(cmd)
	},
	GroupID: GroupAuth.ID,
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
