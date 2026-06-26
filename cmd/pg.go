package cmd

import "github.com/spf13/cobra"

func newPgCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pg",
		Aliases: []string{"postgres"},
		Short:   "Manage Render Postgres databases",
		Long: `Manage Render Postgres databases.

Lives under 'ea' while the command surface stabilizes. Postgres itself is
generally available, but the flag set and output format for these commands may
still change. We'll promote them out of 'ea' once we're confident in the
contract.`,
	}
	cmd.AddCommand(children...)
	return cmd
}
