package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/dependencies"
)

func setupAnalyticsCommands(root *cobra.Command, deps *dependencies.Dependencies) {
	root.AddCommand(newAnalyticsCmd(newAnalyticsSendCmd(deps)))
}

func newAnalyticsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "analytics",
		Short:  "Internal analytics commands",
		Hidden: true,
	}
	markCommandAnalyticsIneligible(cmd)

	cmd.AddCommand(children...)
	return cmd
}
