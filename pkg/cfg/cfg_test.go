package cfg_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/cfg"
)

func TestShouldLogAnalytics(t *testing.T) {
	testCases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "enabled by 1", value: "1", want: true},
		{name: "unset", value: "", want: false},
		{name: "zero", value: "0", want: false},
		{name: "true is not accepted", value: "true", want: false},
		{name: "yes is not accepted", value: "yes", want: false},
		{name: "whitespace around 1 is not accepted", value: " 1", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_LOG_ANALYTICS", tc.value)

			require.Equal(t, tc.want, cfg.ShouldLogAnalytics())
		})
	}
}

func TestAnalyticsDevGateOpen(t *testing.T) {
	testCases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "enabled by 1", value: "1", want: true},
		{name: "unset", value: "", want: false},
		{name: "zero", value: "0", want: false},
		{name: "true is not accepted", value: "true", want: false},
		{name: "yes is not accepted", value: "yes", want: false},
		{name: "whitespace around 1 is not accepted", value: " 1", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", tc.value)

			require.Equal(t, tc.want, cfg.AnalyticsDevGateOpen())
		})
	}
}

func TestDoNotTrack(t *testing.T) {
	testCases := []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "TRUE", want: true},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "anything", want: false},
	}

	for _, tc := range testCases {
		t.Setenv("DO_NOT_TRACK", tc.value)

		require.Equal(t, tc.want, cfg.DoNotTrack(), "DO_NOT_TRACK=%q", tc.value)
	}
}

func TestAnalyticsDisabledByEnv(t *testing.T) {
	testCases := []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "0", want: false},
		{value: "false", want: false},
		{value: "yes", want: false},
		{value: "1", want: true},
		{value: "true", want: true},
		{value: "TRUE", want: true},
	}

	for _, tc := range testCases {
		t.Setenv("RENDER_CLI_DISABLE_ANALYTICS", tc.value)

		require.Equal(t, tc.want, cfg.AnalyticsDisabledByEnv(), "RENDER_CLI_DISABLE_ANALYTICS=%q", tc.value)
	}
}

func TestAnalyticsStrategyReturnsRawValue(t *testing.T) {
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", " sync ")

	require.Equal(t, " sync ", cfg.AnalyticsStrategy())
}

func TestIsCI(t *testing.T) {
	testCases := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "true is CI", value: "true", want: true},
		{name: "uppercase TRUE is CI", value: "TRUE", want: true},
		{name: "1 is CI", value: "1", want: true},
		{name: "unset", value: "", want: false},
		{name: "false", value: "false", want: false},
		{name: "yes is not accepted", value: "yes", want: false},
		{name: "0", value: "0", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CI", tc.value)

			require.Equal(t, tc.want, cfg.IsCI())
		})
	}
}
