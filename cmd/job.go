package cmd

import (
	"github.com/spf13/cobra"
)

var jobCmd = &cobra.Command{
	Use:     "jobs",
	Short:   "Create and manage one-off jobs",
	GroupID: GroupCore.ID,
	Example: `  # List jobs for a service
  render jobs list srv-abc123

  # Create a job for a service
  render jobs create srv-abc123 --start-command "bundle exec rake task"`,
}

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.AddCommand(jobListCmd, JobCreateCmd, jobCancelCmd)
}
