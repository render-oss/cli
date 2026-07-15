package cmd

import (
	"github.com/spf13/cobra"
)

var EarlyAccessCmd = newEarlyAccessCmd()

func newEarlyAccessCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ea",
		Short: "Use early access commands",
		Long:  `These commands are in early access and are subject to change.`,
		Example: `  # List early access object storage resources
  render ea objects list --region=oregon`,
		// Reject unknown subcommands with a non-zero exit. Cobra flags unknown
		// commands only for the root, so a nested parent needs NoArgs, plus a
		// RunE so arg validation runs (it also shows help for a bare "render ea").
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}

func init() {
	rootCmd.AddCommand(EarlyAccessCmd)
}
