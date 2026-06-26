package cmd

import "github.com/spf13/cobra"

func newPgCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "postgres",
		Aliases: []string{"pg"},
		Short:   "Manage Render Postgres databases (alias: pg)",
		Long:    `Manage Render Postgres databases.`,
		GroupID: GroupCore.ID,
	}
	cmd.AddCommand(children...)
	return cmd
}
