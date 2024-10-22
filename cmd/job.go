package cmd

import (
	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Manage jobs",
	Long:  `List, create, and cancel jobs for services.`,
}

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobListCmd, jobCreateCmd, jobCancelCmd)
}
