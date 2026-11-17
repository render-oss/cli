package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/blang/semver/v4"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/updatenotice"
	"github.com/render-oss/cli/pkg/command"
)

// fixedReleaseClient reports the same latest release, published two days ago,
// for every lookup.
type fixedReleaseClient string

func (c fixedReleaseClient) LatestRelease(context.Context) (updatenotice.Release, error) {
	v, err := semver.Parse(string(c))
	return updatenotice.Release{Version: v, PublishedAt: time.Now().Add(-48 * time.Hour)}, err
}

func TestUpdateNoticeWaitsForAnalyticsNotice(t *testing.T) {
	h := newAnalyticsHarness(t, analyticsHarnessInitialState{stderrTTY: true})
	t.Setenv("RENDER_CLI_DISABLE_UPDATE_NOTIFIER", "")

	run := func() string {
		check := updatenotice.Start(updatenotice.Conditions{Interactive: true}, fixedReleaseClient("9.9.9"), "1.0.0")
		require.NotNil(t, check)

		invocation := h.newInvocation()
		invocation.root.SetArgs([]string{"ping"})
		result := runExecution(invocation.root, time.Now())
		onExecutionComplete(result, invocation.deps, invocation.root, check)
		return ansi.Strip(invocation.stderr.String())
	}

	first := run()
	require.Contains(t, first, "The Render CLI collects usage data")
	require.NotContains(t, first, "Update available", "only one notice per command")

	second := run()
	require.NotContains(t, second, "The Render CLI collects usage data")
	require.Contains(t, second, "Update available: v1.0.0 → v9.9.9")
}

func TestUpdateNoticeSkipsStructuredOutput(t *testing.T) {
	testCases := []struct {
		output   command.Output
		wantShow bool
	}{
		{output: command.JSON},
		{output: command.YAML},
		{output: command.TEXT, wantShow: true},
		{output: command.Interactive, wantShow: true},
	}

	for _, tc := range testCases {
		t.Run(string(tc.output), func(t *testing.T) {
			h := newAnalyticsHarness(t, analyticsHarnessInitialState{stderrTTY: true, noticeMarkerPresent: true})
			t.Setenv("RENDER_CLI_DISABLE_UPDATE_NOTIFIER", "")
			check := updatenotice.Start(updatenotice.Conditions{Interactive: true}, fixedReleaseClient("9.9.9"), "1.0.0")

			invocation := h.newInvocation()
			result := command.ExecutionResult{OutputFormat: &tc.output}
			onExecutionComplete(result, invocation.deps, invocation.root, check)

			if tc.wantShow {
				require.Contains(t, invocation.stderr.String(), "Update available")
			} else {
				require.NotContains(t, invocation.stderr.String(), "Update available")
			}
		})
	}
}

// With stderr piped, the analytics notice is never printed, so it must not
// hold back the update notice an AI agent reads there.
func TestUpdateNoticeReachesPipedAgentWithoutAnalyticsMarker(t *testing.T) {
	h := newAnalyticsHarness(t, analyticsHarnessInitialState{})
	t.Setenv("RENDER_CLI_DISABLE_UPDATE_NOTIFIER", "")
	check := updatenotice.Start(updatenotice.Conditions{Agent: true}, fixedReleaseClient("9.9.9"), "1.0.0")

	invocation := h.newInvocation()
	invocation.root.SetArgs([]string{"ping"})
	result := runExecution(invocation.root, time.Now())
	onExecutionComplete(result, invocation.deps, invocation.root, check)

	require.Contains(t, invocation.stderr.String(), "Update available: v1.0.0 → v9.9.9. ")
	require.NotContains(t, invocation.stderr.String(), "The Render CLI collects usage data")
}
