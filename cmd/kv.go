package cmd

import "github.com/spf13/cobra"

var kvCmd = &cobra.Command{
	Use:     "kv",
	Aliases: []string{"keyvalue"},
	Short:   "Manage Key Value store instances (early access)",
}

func init() {
	EarlyAccessCmd.AddCommand(kvCmd)
}
