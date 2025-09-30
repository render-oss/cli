package cmd

import (
	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "versions",
	Short: "Manage workflow versions",
}

func init() {
	EarlyAccessCmd.AddCommand(versionCmd)
}
