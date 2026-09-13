package cmd

import (
	"github.com/spf13/cobra"

	"github.com/render-oss/cli/internal/analyticsnotice"
	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func analyticsNoticeConditions(signals command.RuntimeSignals) analyticsnotice.Conditions {
	return analyticsnotice.Conditions{CI: signals.CI, StderrTTY: signals.StderrTTY}
}

// newAnalyticsNoticeCmd builds the hidden command that discloses analytics collection
// if it has not been displayed yet on this machine
// Installers can invoke this command after install or upgrade
func newAnalyticsNoticeCmd(deps *dependencies.Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:    "notice",
		Short:  "Show the analytics notice if it has not been shown yet",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			signals, err := deps.DetectRuntimeSignals()
			if err != nil {
				return err
			}

			analyticsnotice.ShowIfNeeded(
				command.NewStream(cmd.ErrOrStderr()),
				analyticsNoticeConditions(signals),
				analytics.ResolveConsent().OptOutReason,
			)
			return nil
		},
	}
}
