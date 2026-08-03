package analytics

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveStrategy(t *testing.T) {
	testCases := []struct {
		name           string
		inputs         strategyInputs
		wantStrategy   sendStrategy
		wantDiagnostic string
	}{
		{
			name:         "CI true uses sync",
			inputs:       strategyInputs{ci: "true"},
			wantStrategy: strategySync,
		},
		{
			name:         "CI true takes precedence",
			inputs:       strategyInputs{ci: "true", loggingEnabled: true, sendingEnabled: true, configuredStrategy: "banana"},
			wantStrategy: strategySync,
		},
		{
			name:         "CI false is not CI",
			inputs:       strategyInputs{ci: "false"},
			wantStrategy: strategyDetached,
		},
		{
			name:         "CI 1 is not CI",
			inputs:       strategyInputs{ci: "1"},
			wantStrategy: strategyDetached,
		},
		{
			name:         "uppercase CI true is not CI",
			inputs:       strategyInputs{ci: "TRUE"},
			wantStrategy: strategyDetached,
		},
		{
			name:         "unset CI and empty strategy use detached",
			inputs:       strategyInputs{},
			wantStrategy: strategyDetached,
		},
		{
			name:         "logging and sending use sync",
			inputs:       strategyInputs{loggingEnabled: true, sendingEnabled: true},
			wantStrategy: strategySync,
		},
		{
			name:         "logging and sending take precedence over configured strategy",
			inputs:       strategyInputs{loggingEnabled: true, sendingEnabled: true, configuredStrategy: "banana"},
			wantStrategy: strategySync,
		},
		{
			name:         "logging without sending uses detached",
			inputs:       strategyInputs{loggingEnabled: true},
			wantStrategy: strategyDetached,
		},
		{
			name:         "configured sync uses sync",
			inputs:       strategyInputs{configuredStrategy: "sync"},
			wantStrategy: strategySync,
		},
		{
			name:         "configured auto uses detached",
			inputs:       strategyInputs{configuredStrategy: "auto"},
			wantStrategy: strategyDetached,
		},
		{
			name:         "unknown strategy uses detached silently",
			inputs:       strategyInputs{configuredStrategy: "banana"},
			wantStrategy: strategyDetached,
		},
		{
			name:           "unknown strategy is diagnosed when logging",
			inputs:         strategyInputs{loggingEnabled: true, configuredStrategy: "banana"},
			wantStrategy:   strategyDetached,
			wantDiagnostic: `unknown RENDER_CLI_ANALYTICS_STRATEGY value "banana"; using auto`,
		},
		{
			name:           "whitespace around strategy is not ignored",
			inputs:         strategyInputs{loggingEnabled: true, configuredStrategy: " sync "},
			wantStrategy:   strategyDetached,
			wantDiagnostic: `unknown RENDER_CLI_ANALYTICS_STRATEGY value " sync "; using auto`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			strategy, diagnostic := resolveStrategy(tc.inputs)

			require.Equal(t, tc.wantStrategy, strategy)
			require.Equal(t, tc.wantDiagnostic, diagnostic)
		})
	}
}
