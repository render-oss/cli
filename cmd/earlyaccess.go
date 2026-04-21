package cmd

import (
	"github.com/spf13/cobra"
)

var EarlyAccessCmd = &cobra.Command{
	Use:   "ea",
	Short: "Use early access commands",
	Long:  `These commands are in early access and are subject to change.`,
	Example: `  # List early access object storage resources
  render ea objects list --region=oregon`,
}

func init() {
	rootCmd.AddCommand(EarlyAccessCmd)
}
