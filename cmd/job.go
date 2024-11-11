package cmd

import (
	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:     "jobs",
	Short:   "Manage one-off jobs",
	GroupID: GroupCore.ID,
}

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobListCmd, jobCreateCmd, jobCancelCmd)
}
