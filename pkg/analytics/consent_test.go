package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/config"
)

func TestResolveConsent(t *testing.T) {
	testCases := []struct {
		name           string
		doNotTrack     string
		disableEnv     string
		configDisabled bool
		// wantReason is OptOutReasonNone when analytics should stay enabled.
		wantReason OptOutReason
	}{
		{name: "enabled by default with no signals and no config file"},
		{name: "DO_NOT_TRACK=1 opts out", doNotTrack: "1", wantReason: OptOutReasonDoNotTrack},
		{name: "DO_NOT_TRACK=true opts out", doNotTrack: "true", wantReason: OptOutReasonDoNotTrack},
		{name: "DO_NOT_TRACK=0 does not opt out", doNotTrack: "0"},
		{name: "RENDER_CLI_DISABLE_ANALYTICS=1 opts out", disableEnv: "1", wantReason: OptOutReasonDisableAnalyticsEnv},
		{name: "RENDER_CLI_DISABLE_ANALYTICS=true opts out", disableEnv: "true", wantReason: OptOutReasonDisableAnalyticsEnv},
		{name: "RENDER_CLI_DISABLE_ANALYTICS=TRUE opts out", disableEnv: "TRUE", wantReason: OptOutReasonDisableAnalyticsEnv},
		{name: "RENDER_CLI_DISABLE_ANALYTICS=0 does not opt out", disableEnv: "0"},
		{name: "RENDER_CLI_DISABLE_ANALYTICS=false does not opt out", disableEnv: "false"},
		{name: "analytics.disabled in config opts out", configDisabled: true, wantReason: OptOutReasonAnalyticsDisabledConfig},
		{name: "env wins the reported reason over config", doNotTrack: "1", configDisabled: true, wantReason: OptOutReasonDoNotTrack},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DO_NOT_TRACK", tc.doNotTrack)
			t.Setenv("RENDER_CLI_DISABLE_ANALYTICS", tc.disableEnv)
			t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
			t.Setenv("RENDER_CLI_CONFIG_PATH", "")

			if tc.configDisabled {
				cfgFile := &config.Config{Analytics: config.AnalyticsConfig{Disabled: true}}
				require.NoError(t, cfgFile.Persist())
			}

			consent := ResolveConsent()

			require.Equal(t, tc.wantReason == OptOutReasonNone, consent.Granted)
			require.Equal(t, tc.wantReason, consent.OptOutReason)
		})
	}
}
