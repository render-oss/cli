package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/dependencies"
)

func newAnalyticsSendCmd(deps *dependencies.Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:    "send <event-file>",
		Short:  "Send an analytics event from a file",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		// A synchronous send wires this command's stderr to the stderr of the
		// command the user actually ran, so Cobra's "Error: ..." would surface
		// there and read as though that command failed. Reporting an analytics
		// failure is [analytics.Sender.SendFile]'s job, on the diagnostics writer
		// and under the analytics prefix. The error is still returned so the
		// non-zero exit code tells a waiting parent process the send failed.
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return deps.Analytics().SendFile(cmd.Context(), args[0], cmd.ErrOrStderr())
		},
	}
}
