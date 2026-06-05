package cmd

import "github.com/spf13/cobra"

func newKVCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "kv",
		Aliases: []string{"keyvalue"},
		Short:   "Manage Key Value store instances (early access)",
	}
	cmd.AddCommand(children...)
	return cmd
}
