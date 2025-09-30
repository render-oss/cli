package cmd

import (
	"github.com/spf13/cobra"
)

var EarlyAccessCmd = &cobra.Command{
	Use:   "ea",
	Short: "Early access commands",
	Long:  `These commands are in early access and are subject to change.`,
}

func init() {
	rootCmd.AddCommand(EarlyAccessCmd)
}
