package analytics

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectAgentSignals(t *testing.T) {
	testCases := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "no agent environment",
			env:  map[string]string{},
			want: []string{},
		},
		{
			name: "multiple nested agents are sorted",
			env: map[string]string{
				"VSCODE_AGENT":    "copilot",
				"CODEX_THREAD_ID": "thread-123",
				"CLAUDECODE":      "1",
			},
			want: []string{"CLAUDECODE", "CODEX_THREAD_ID", "VSCODE_AGENT"},
		},
		{
			name: "empty presence marker is inactive",
			env:  map[string]string{"CODEX_THREAD_ID": ""},
			want: []string{},
		},
		{
			name: "unknown and secret-looking variables are ignored",
			env: map[string]string{
				"AGENT":              "codex",
				"OPENAI_API_KEY":     "secret-canary-value",
				"RENDER_FAKE_SECRET": "another-secret-canary",
			},
			want: []string{},
		},
		{
			name: "unknown variables are ignored alongside active signals",
			env: map[string]string{
				"CODEX_THREAD_ID":    "thread-123",
				"AGENT":              "codex",
				"OPENAI_API_KEY":     "secret-canary-value",
				"RENDER_FAKE_SECRET": "another-secret-canary",
			},
			want: []string{"CODEX_THREAD_ID"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ClearSignalEnvVars(t)
			for name, value := range tc.env {
				t.Setenv(name, value)
			}

			require.Equal(t, tc.want, DetectAgentSignals())
		})
	}
}

func TestDetectAgentSignalsActivationSemantics(t *testing.T) {
	testCases := []struct {
		envVar        string
		activating    []string
		nonActivating []string
	}{
		{
			envVar:        "AMP_CURRENT_THREAD_ID",
			activating:    []string{"thread-123", "1"},
			nonActivating: []string{""},
		},
		{
			envVar:        "CLAUDECODE",
			activating:    []string{"1", "true", "TRUE", " True ", " 1 "},
			nonActivating: []string{"", "0", "false", "yes"},
		},
		{
			envVar:        "CLINE",
			activating:    []string{"1", "true", "TRUE", " True ", " 1 "},
			nonActivating: []string{"", "0", "false", "yes"},
		},
		{
			envVar:        "CLINE_ACTIVE",
			activating:    []string{"1", "true", "TRUE", " True ", " 1 "},
			nonActivating: []string{"", "0", "false", "yes"},
		},
		{
			envVar:        "CODEX_THREAD_ID",
			activating:    []string{"thread-123", "1"},
			nonActivating: []string{""},
		},
		{
			envVar:        "COPILOT_AGENT_SESSION_ID",
			activating:    []string{"session-123", "1"},
			nonActivating: []string{""},
		},
		{
			envVar:        "CURSOR_AGENT",
			activating:    []string{"1", "cursor"},
			nonActivating: []string{""},
		},
		{
			envVar:        "CURSOR_TRACE_ID",
			activating:    []string{"trace-123", "1"},
			nonActivating: []string{""},
		},
		{
			envVar:        "GEMINI_CLI",
			activating:    []string{"1", "true", "TRUE", " True ", " 1 "},
			nonActivating: []string{"", "0", "false", "yes"},
		},
		{
			envVar:        "GOOSE_TERMINAL",
			activating:    []string{"1", "true", "TRUE", " True ", " 1 "},
			nonActivating: []string{"", "0", "false", "yes"},
		},
		{
			envVar:        "OPENCODE_CLIENT",
			activating:    []string{"desktop", "acp"},
			nonActivating: []string{"", "cli", "DESKTOP", "terminal"},
		},
		{
			envVar:        "PI_CODING_AGENT",
			activating:    []string{"1", "true", "TRUE", " True ", " 1 "},
			nonActivating: []string{"", "0", "false", "yes"},
		},
		{
			envVar:        "VSCODE_AGENT",
			activating:    []string{"copilot", "1"},
			nonActivating: []string{""},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.envVar, func(t *testing.T) {
			ClearSignalEnvVars(t)

			for _, value := range tc.activating {
				t.Setenv(tc.envVar, value)
				require.Equalf(t, []string{tc.envVar}, DetectAgentSignals(),
					"value %q should activate %s", value, tc.envVar)
			}
			for _, value := range tc.nonActivating {
				t.Setenv(tc.envVar, value)
				require.Equalf(t, []string{}, DetectAgentSignals(),
					"value %q should not activate %s", value, tc.envVar)
			}
		})
	}
}

func TestDetectAgentSignalsReturnsNonNilEmptySlice(t *testing.T) {
	ClearSignalEnvVars(t)

	got := DetectAgentSignals()

	require.NotNil(t, got)
	require.Empty(t, got)

	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(encoded))
}
