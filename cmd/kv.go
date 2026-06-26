package cmd

import "github.com/spf13/cobra"

func newKVCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "keyvalues",
		Aliases: []string{"kv", "keyvalue"},
		Short:   "Manage Render Key Value instances (alias: kv)",
		GroupID: GroupCore.ID,
	}
	cmd.AddCommand(children...)
	return cmd
}
