package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/v2/pkg/dashboard"
)

var docsCmd = &cobra.Command{
	Use:   "docs",
	Short: "Open the Render docs in your browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		return dashboard.Open("https://render.com/docs")
	},
}

func init() {
	rootCmd.AddCommand(docsCmd)
}
