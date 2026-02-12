// pattern: Imperative Shell
package command_test

import (
	"testing"

	"github.com/render-oss/cli/pkg/command"
	"github.com/stretchr/testify/require"
)

func TestDetectRuntimeSignals(t *testing.T) {
	// Capture baseline TTY state from the actual test environment so assertions
	// remain stable regardless of whether tests run in a terminal or CI.
	t.Setenv("RENDER_OUTPUT", "")
	baselineSignals, err := command.DetectRuntimeSignals()
	require.NoError(t, err)

	testCases := []struct {
		name                string
		env                 map[string]string
		wantCI              bool
		wantDumbTerminal    bool
		wantForcedOutputSet bool
		wantForcedOutput    command.Output
		wantErr             bool
	}{
		{
			name: "RENDER_OUTPUT=json sets forced output to json",
			env: map[string]string{
				"RENDER_OUTPUT": "json",
			},
			wantForcedOutputSet: true,
			wantForcedOutput:    command.JSON,
		},
		{
			name: "RENDER_OUTPUT=yaml sets forced output to yaml",
			env: map[string]string{
				"RENDER_OUTPUT": "yaml",
			},
			wantForcedOutputSet: true,
			wantForcedOutput:    command.YAML,
		},
		{
			name: "RENDER_OUTPUT=interactive sets forced output to interactive",
			env: map[string]string{
				"RENDER_OUTPUT": "interactive",
			},
			wantForcedOutputSet: true,
			wantForcedOutput:    command.Interactive,
		},
		{
			name: "RENDER_OUTPUT=text sets forced output to text",
			env: map[string]string{
				"RENDER_OUTPUT": "text",
			},
			wantForcedOutputSet: true,
			wantForcedOutput:    command.TEXT,
		},
		{
			name: "RENDER_OUTPUT parsing is case-insensitive",
			env: map[string]string{
				"RENDER_OUTPUT": "YaMl",
			},
			wantForcedOutputSet: true,
			wantForcedOutput:    command.YAML,
		},
		{
			name: "invalid RENDER_OUTPUT returns error",
			env: map[string]string{
				"RENDER_OUTPUT": "banana",
			},
			wantErr: true,
		},
		{
			name: "CI=true maps to CI signal true",
			env: map[string]string{
				"CI": "true",
			},
			wantCI: true,
		},
		{
			name: "CI=1 maps to CI signal true",
			env: map[string]string{
				"CI": "1",
			},
			wantCI: true,
		},
		{
			name: "non-truthy CI values map to CI signal false",
			env: map[string]string{
				"CI": "yes",
			},
		},
		{
			name: "shell-only env variables are ignored",
			env: map[string]string{
				"PS1":                   ">",
				"$-":                    "i",
				"FISH_INTERACTIVE":      "1",
				"SHELL_INTERACTIVE":     "true",
				"RENDER_INTERACTIVE":    "true",
				"RENDER_NONINTERACTIVE": "1",
			},
		},
		{
			name: "TERM=dumb maps to DumbTerminal signal true",
			env: map[string]string{
				"TERM": "dumb",
			},
			wantDumbTerminal: true,
		},
		{
			name: "TERM=xterm-256color does not set DumbTerminal",
			env: map[string]string{
				"TERM": "xterm-256color",
			},
			wantDumbTerminal: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear env vars that might be set by CI or the host environment
			// before applying test-specific values. t.Setenv restores original
			// values after each subtest.
			t.Setenv("CI", "")
			t.Setenv("TERM", "")
			t.Setenv("RENDER_OUTPUT", "")

			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			signals, err := command.DetectRuntimeSignals()
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, baselineSignals.StdinTTY, signals.StdinTTY)
			require.Equal(t, baselineSignals.StdoutTTY, signals.StdoutTTY)
			require.Equal(t, baselineSignals.StderrTTY, signals.StderrTTY)
			require.Equal(t, tc.wantDumbTerminal, signals.DumbTerminal)
			require.Equal(t, tc.wantCI, signals.CI)
			if !tc.wantForcedOutputSet {
				require.Nil(t, signals.ForcedOutput)
				return
			}

			require.NotNil(t, signals.ForcedOutput)
			require.Equal(t, tc.wantForcedOutput, *signals.ForcedOutput)
		})
	}
}

func TestResolveAutoOutput(t *testing.T) {
	testCases := []struct {
		name            string
		explicitOutput  bool
		requestedOutput command.Output
		signals         command.RuntimeSignals
		wantOutput      command.Output
		wantErr         bool
	}{
		{
			name:            "explicit interactive output takes precedence",
			explicitOutput:  true,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				CI: true,
			},
			wantOutput: command.Interactive,
		},
		{
			name:            "explicit json output takes precedence",
			explicitOutput:  true,
			requestedOutput: command.JSON,
			signals: command.RuntimeSignals{
				CI: true,
			},
			wantOutput: command.JSON,
		},
		{
			name:            "explicit yaml output takes precedence",
			explicitOutput:  true,
			requestedOutput: command.YAML,
			signals: command.RuntimeSignals{
				CI: true,
			},
			wantOutput: command.YAML,
		},
		{
			name:            "explicit text output takes precedence",
			explicitOutput:  true,
			requestedOutput: command.TEXT,
			signals: command.RuntimeSignals{
				CI: true,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "explicit output takes precedence over RENDER_OUTPUT",
			explicitOutput:  true,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				ForcedOutput: outputPointer(command.JSON),
			},
			wantOutput: command.Interactive,
		},
		{
			name:            "RENDER_OUTPUT forced value takes precedence in auto mode",
			explicitOutput:  false,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				ForcedOutput: outputPointer(command.TEXT),
				StdinTTY:     true,
				StdoutTTY:    true,
				StderrTTY:    true,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "auto mode ignores requested structured output and follows signal path",
			explicitOutput:  false,
			requestedOutput: command.JSON,
			signals: command.RuntimeSignals{
				StdinTTY:  false,
				StdoutTTY: true,
				StderrTTY: true,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "all tty and ci false resolves interactive",
			explicitOutput:  false,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				StdinTTY:  true,
				StdoutTTY: true,
				StderrTTY: true,
			},
			wantOutput: command.Interactive,
		},
		{
			name:            "non-tty stdin resolves text",
			explicitOutput:  false,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				StdinTTY:  false,
				StdoutTTY: true,
				StderrTTY: true,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "non-tty stdout resolves text",
			explicitOutput:  false,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				StdinTTY:  true,
				StdoutTTY: false,
				StderrTTY: true,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "non-tty stderr resolves text",
			explicitOutput:  false,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				StdinTTY:  true,
				StdoutTTY: true,
				StderrTTY: false,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "ci true resolves text",
			explicitOutput:  false,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				StdinTTY:  true,
				StdoutTTY: true,
				StderrTTY: true,
				CI:        true,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "TERM=dumb resolves text even with all TTYs",
			explicitOutput:  false,
			requestedOutput: command.Interactive,
			signals: command.RuntimeSignals{
				StdinTTY:     true,
				StdoutTTY:    true,
				StderrTTY:    true,
				DumbTerminal: true,
			},
			wantOutput: command.TEXT,
		},
		{
			name:            "invalid requested output returns error",
			explicitOutput:  false,
			requestedOutput: command.Output("invalid"),
			signals:         command.RuntimeSignals{},
			wantErr:         true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := command.ResolveAutoOutput(tc.explicitOutput, tc.requestedOutput, tc.signals)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.wantOutput, output)
		})
	}
}

func outputPointer(output command.Output) *command.Output {
	return &output
}
