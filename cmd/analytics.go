package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/dependencies"
)

func setupAnalyticsCommands(root *cobra.Command, deps *dependencies.Dependencies) {
	root.AddCommand(newAnalyticsCmd(newAnalyticsSendCmd(deps), newAnalyticsNoticeCmd(deps)))
}

func newAnalyticsCmd(children ...*cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "analytics",
		Short:  "Internal analytics commands",
		Hidden: true,
	}
	markCommandAnalyticsIneligible(cmd)
	markCommandAnalyticsNoticeIneligible(cmd)

	cmd.AddCommand(children...)
	return cmd
}

// isShellCompletionCommand reports whether cmd belongs to Cobra's generated
// completion tree or is one of its internal completion request commands.
func isShellCompletionCommand(cmd *cobra.Command) bool {
	name := rootChildName(cmd)
	return name == "completion" ||
		name == cobra.ShellCompRequestCmd ||
		name == cobra.ShellCompNoDescRequestCmd
}

// rootChildName returns the name of the direct child of cmd's root that contains
// cmd. For example, both `render services` and `render services list` return
// "services", while `render` and nil return "".
func rootChildName(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}

	root := cmd.Root()
	for cmd.Parent() != nil && cmd.Parent() != root {
		cmd = cmd.Parent()
	}
	if cmd.Parent() != root {
		return ""
	}
	return cmd.Name()
}
