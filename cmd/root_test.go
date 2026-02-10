// pattern: Imperative Shell
package cmd

import (
	"context"
	"testing"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestRootPersistentPreRunOutputResolution(t *testing.T) {
	testCases := []struct {
		name             string
		input            runRootPersistentPreRunInput
		wantOutput       command.Output
		wantStackContext bool
	}{
		{
			name: "default output with unchanged flag uses auto mode and resolves json for non-tty",
			input: runRootPersistentPreRunInput{
				explicitOutput: false,
				outputValue:    "interactive",
				signals: command.RuntimeSignals{
					StdinTTY:  true,
					StdoutTTY: false,
					StderrTTY: true,
				},
			},
			wantOutput: command.JSON,
		},
		{
			name: "explicit interactive remains interactive regardless of non-tty ci signals",
			input: runRootPersistentPreRunInput{
				explicitOutput: true,
				outputValue:    "interactive",
				signals: command.RuntimeSignals{
					StdinTTY:  false,
					StdoutTTY: false,
					StderrTTY: false,
					CI:        true,
				},
			},
			wantOutput:       command.Interactive,
			wantStackContext: true,
		},
		{
			name: "explicit json output is preserved",
			input: runRootPersistentPreRunInput{
				explicitOutput: true,
				outputValue:    "json",
				signals: command.RuntimeSignals{
					StdinTTY:  true,
					StdoutTTY: true,
					StderrTTY: true,
				},
			},
			wantOutput: command.JSON,
		},
		{
			name: "explicit output takes precedence over RENDER_OUTPUT",
			input: runRootPersistentPreRunInput{
				explicitOutput: true,
				outputValue:    "interactive",
				signals: command.RuntimeSignals{
					ForcedOutput: outputPointer(command.JSON),
					StdinTTY:     true,
					StdoutTTY:    true,
					StderrTTY:    true,
				},
			},
			wantOutput:       command.Interactive,
			wantStackContext: true,
		},
		{
			name: "explicit structured output takes precedence over RENDER_OUTPUT",
			input: runRootPersistentPreRunInput{
				explicitOutput: true,
				outputValue:    "yaml",
				signals: command.RuntimeSignals{
					ForcedOutput: outputPointer(command.Interactive),
					StdinTTY:     false,
					StdoutTTY:    false,
					StderrTTY:    false,
					CI:           true,
				},
			},
			wantOutput: command.YAML,
		},
		{
			name: "explicit yaml output is preserved",
			input: runRootPersistentPreRunInput{
				explicitOutput: true,
				outputValue:    "yaml",
				signals: command.RuntimeSignals{
					StdinTTY:  true,
					StdoutTTY: true,
					StderrTTY: true,
				},
			},
			wantOutput: command.YAML,
		},
		{
			name: "ci truthy in auto mode resolves json",
			input: runRootPersistentPreRunInput{
				explicitOutput: false,
				outputValue:    "interactive",
				signals: command.RuntimeSignals{
					StdinTTY:  true,
					StdoutTTY: true,
					StderrTTY: true,
					CI:        true,
				},
			},
			wantOutput: command.JSON,
		},
		{
			name: "all tty and ci false in auto mode resolves interactive",
			input: runRootPersistentPreRunInput{
				explicitOutput: false,
				outputValue:    "interactive",
				signals: command.RuntimeSignals{
					StdinTTY:  true,
					StdoutTTY: true,
					StderrTTY: true,
					CI:        false,
				},
			},
			wantOutput:       command.Interactive,
			wantStackContext: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := runRootPersistentPreRun(t, tc.input)

			output := command.GetFormatFromContext(result.cmd.Context())
			require.NotNil(t, output)
			require.Equal(t, tc.wantOutput, *output)

			stack := tui.GetStackFromContext(result.cmd.Context())
			if tc.wantStackContext {
				require.NotNil(t, stack)
				require.Equal(t, result.deps.Stack(), stack)
				return
			}

			require.Nil(t, stack)
		})
	}
}

type runRootPersistentPreRunInput struct {
	explicitOutput bool
	outputValue    string
	signals        command.RuntimeSignals
}

type runRootPersistentPreRunResult struct {
	cmd  *cobra.Command
	deps *dependencies.Dependencies
}

func runRootPersistentPreRun(t *testing.T, input runRootPersistentPreRunInput) runRootPersistentPreRunResult {
	t.Helper()

	deps := dependencies.New(nil)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return input.signals, nil
	}
	preRun := rootPersistentPreRun(deps)

	cmd := &cobra.Command{Use: "render"}
	cmd.Flags().StringP("output", "o", "interactive", "interactive, json, yaml, or text")
	cmd.Flags().Bool(command.ConfirmFlag, false, "set to skip confirmation prompts")
	cmd.SetContext(context.Background())

	if input.explicitOutput {
		require.NoError(t, cmd.Flags().Set("output", input.outputValue))
	}

	require.NoError(t, preRun(cmd, []string{}))
	return runRootPersistentPreRunResult{
		cmd:  cmd,
		deps: deps,
	}
}

func outputPointer(output command.Output) *command.Output {
	return &output
}
