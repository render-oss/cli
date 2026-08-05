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
			name:         "CI uses sync",
			inputs:       strategyInputs{isCI: true},
			wantStrategy: strategySync,
		},
		{
			name:           "CI takes precedence",
			inputs:         strategyInputs{isCI: true, loggingEnabled: true, configuredStrategy: "banana"},
			wantStrategy:   strategySync,
			wantDiagnostic: `unknown RENDER_CLI_ANALYTICS_STRATEGY value "banana"; ignoring it`,
		},
		{
			name:         "no CI and empty strategy use detached",
			inputs:       strategyInputs{},
			wantStrategy: strategyDetached,
		},
		{
			name:         "if logging is enabled, use sync",
			inputs:       strategyInputs{loggingEnabled: true},
			wantStrategy: strategySync,
		},
		{
			name:           "logging takes precedence over configured strategy",
			inputs:         strategyInputs{loggingEnabled: true, configuredStrategy: "banana"},
			wantStrategy:   strategySync,
			wantDiagnostic: `unknown RENDER_CLI_ANALYTICS_STRATEGY value "banana"; ignoring it`,
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
			name:           "unknown strategy uses detached and is diagnosed",
			inputs:         strategyInputs{configuredStrategy: "banana"},
			wantStrategy:   strategyDetached,
			wantDiagnostic: `unknown RENDER_CLI_ANALYTICS_STRATEGY value "banana"; ignoring it`,
		},
		{
			name:           "whitespace around strategy is not ignored",
			inputs:         strategyInputs{configuredStrategy: " sync "},
			wantStrategy:   strategyDetached,
			wantDiagnostic: `unknown RENDER_CLI_ANALYTICS_STRATEGY value " sync "; ignoring it`,
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
