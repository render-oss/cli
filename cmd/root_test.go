// pattern: Imperative Shell
package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

const analyticsSubprocessHelperEnv = "RENDER_CLI_TEST_ANALYTICS_SEND_HELPER"

func TestMain(m *testing.M) {
	// Sender re-executes the current binary. In these integration tests that is
	// the cmd test binary, so let the child impersonate the real render entry
	// point and execute the hidden analytics command against the parent's fake.
	if os.Getenv(analyticsSubprocessHelperEnv) == "1" {
		os.Exit(Execute())
	}

	os.Exit(m.Run())
}

func TestRootPersistentPreRunOutputResolution(t *testing.T) {
	testCases := []struct {
		name             string
		input            runRootPersistentPreRunInput
		wantOutput       command.Output
		wantStackContext bool
	}{
		{
			name: "default output with unchanged flag uses auto mode and resolves text for non-tty",
			input: runRootPersistentPreRunInput{
				explicitOutput: false,
				outputValue:    "interactive",
				signals: command.RuntimeSignals{
					StdinTTY:  true,
					StdoutTTY: false,
					StderrTTY: true,
				},
			},
			wantOutput: command.TEXT,
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
			name: "ci truthy in auto mode resolves text",
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
			wantOutput: command.TEXT,
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

func TestRootPersistentPreRunSuppressesUsageForRuntimeErrors(t *testing.T) {
	root, out := newRootCommandForUsageTests()
	runtimeErr := errors.New("network request failed")
	root.AddCommand(&cobra.Command{
		Use:  "login",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runtimeErr
		},
	})
	root.SetArgs([]string{"login"})

	err := root.Execute()

	require.ErrorIs(t, err, runtimeErr)
	output := stripANSI(out.String())
	require.Contains(t, output, "Error: network request failed")
	require.NotContains(t, output, "Usage:")
	require.NotContains(t, output, "render login [flags]")
}

func TestRootArgumentErrorsStillPrintUsage(t *testing.T) {
	root, out := newRootCommandForUsageTests()
	root.AddCommand(&cobra.Command{
		Use:  "login <token>",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	})
	root.SetArgs([]string{"login"})

	err := root.Execute()

	require.Error(t, err)
	output := stripANSI(out.String())
	require.Contains(t, output, "Error: accepts 1 arg(s), received 0")
	require.Contains(t, output, "Usage:")
	require.Contains(t, output, "render login <token> [flags]")
}

func TestRootDeprecatedFlagErrorsStillPrintUsage(t *testing.T) {
	root, out := newRootCommandForUsageTests()
	root.PersistentFlags().Bool("pretty-json", false, "")
	root.AddCommand(&cobra.Command{
		Use:  "login",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	})
	root.SetArgs([]string{"login", "--pretty-json"})

	err := root.Execute()

	require.EqualError(t, err, "use `--output json` instead of `--pretty-json`")
	output := stripANSI(out.String())
	require.Contains(t, output, "Error: use `--output json` instead of `--pretty-json`")
	require.Contains(t, output, "Usage:")
	require.Contains(t, output, "render login [flags]")
}

func TestExitCodeFromError(t *testing.T) {
	testCases := []struct {
		name         string
		runE         func() error
		wantExitCode int
	}{
		{
			name:         "success",
			runE:         func() error { return nil },
			wantExitCode: 0,
		},
		{
			name:         "ordinary error",
			runE:         func() error { return errors.New("failed") },
			wantExitCode: 1,
		},
		{
			name:         "explicit one exit code",
			runE:         func() error { return command.NewExitError(1, nil) },
			wantExitCode: 1,
		},
		{
			// NewExitError tolerates runtime-provided zero values without panicking,
			// but only a nil error may produce a successful process exit.
			name:         "explicit zero exit code is still an error",
			runE:         func() error { return command.NewExitError(0, nil) },
			wantExitCode: 1,
		},
		{
			name: "wrapped explicit exit code",
			runE: func() error {
				return fmt.Errorf("command failed: %w", command.NewExitError(7, nil))
			},
			wantExitCode: 7,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.wantExitCode, exitCodeFromError(tc.runE()))
		})
	}
}

func TestExecutionResultClassifiesCobraOutcomes(t *testing.T) {
	testCases := []struct {
		name         string
		args         []string
		configure    func(t *testing.T, root, child *cobra.Command)
		wantExitCode int
		wantKind     command.CompletionKind
	}{
		{
			name:         "success",
			args:         []string{"test", "value"},
			wantExitCode: 0,
			wantKind:     command.CompletionKindSuccess,
		},
		{
			name:         "command discovery error",
			args:         []string{"missing"},
			wantExitCode: 1,
			wantKind:     command.CompletionKindDiscoveryError,
		},
		{
			name:         "argument validation error",
			args:         []string{"test"},
			wantExitCode: 1,
			wantKind:     command.CompletionKindValidationError,
		},
		{
			name:         "flag validation error",
			args:         []string{"test", "value", "--missing"},
			wantExitCode: 1,
			wantKind:     command.CompletionKindValidationError,
		},
		{
			name:         "required flag validation error",
			args:         []string{"test", "value"},
			wantExitCode: 1,
			wantKind:     command.CompletionKindValidationError,
			configure: func(t *testing.T, _ *cobra.Command, child *cobra.Command) {
				child.Flags().String("required", "", "required value")
				require.NoError(t, child.MarkFlagRequired("required"))
			},
		},
		{
			name:         "setup error",
			args:         []string{"test", "value", "--output", "invalid"},
			wantExitCode: 1,
			wantKind:     command.CompletionKindSetupError,
		},
		{
			name:         "execution error",
			args:         []string{"test", "value"},
			wantExitCode: 1,
			wantKind:     command.CompletionKindExecutionError,
			configure: func(_ *testing.T, _ *cobra.Command, child *cobra.Command) {
				child.RunE = func(_ *cobra.Command, _ []string) error {
					return errors.New("failed")
				}
			},
		},
		{
			name:         "post-run error",
			args:         []string{"test", "value"},
			wantExitCode: 1,
			wantKind:     command.CompletionKindExecutionError,
			configure: func(_ *testing.T, _ *cobra.Command, child *cobra.Command) {
				child.PostRunE = func(_ *cobra.Command, _ []string) error {
					return errors.New("post-run failed")
				}
			},
		},
		{
			name:         "explicit exit",
			args:         []string{"test", "value"},
			wantExitCode: 7,
			wantKind:     command.CompletionKindExplicitExit,
			configure: func(_ *testing.T, _ *cobra.Command, child *cobra.Command) {
				child.RunE = func(_ *cobra.Command, _ []string) error {
					return command.NewExitError(7, nil)
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := executeAndClassify(t, tc.args, tc.configure)

			require.Equal(t, tc.wantExitCode, result.ExitCode)
			require.Equal(t, tc.wantKind, result.CompletionKind)
			require.GreaterOrEqual(t, result.Duration, time.Duration(0))
			require.NotEmpty(t, result.CommandPath)
		})
	}

	// Help is classified only when the user explicitly requests it, and the
	// result's Command must identify whose help was rendered.
	t.Run("help", func(t *testing.T) {
		addNonRunnableGroup := func(_ *testing.T, root, _ *cobra.Command) {
			root.AddCommand(&cobra.Command{Use: "group"})
		}
		addNestedGroups := func(_ *testing.T, root, _ *cobra.Command) {
			sub := &cobra.Command{Use: "sub"}
			sub.AddCommand(&cobra.Command{
				Use:  "leaf",
				RunE: func(_ *cobra.Command, _ []string) error { return nil },
			})
			group := &cobra.Command{Use: "group"}
			group.AddCommand(sub)
			root.AddCommand(group)
		}
		// The `render ea` shape: NoArgs rejects unknown children with a non-zero
		// exit, and RunE renders help for a bare invocation.
		addRunnableGroup := func(_ *testing.T, root, _ *cobra.Command) {
			group := &cobra.Command{
				Use:  "rgroup",
				Args: cobra.NoArgs,
				RunE: func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
			}
			group.AddCommand(&cobra.Command{
				Use:  "leaf",
				RunE: func(_ *cobra.Command, _ []string) error { return nil },
			})
			root.AddCommand(group)
		}
		helpCases := []struct {
			name            string
			args            []string
			configure       func(t *testing.T, root, child *cobra.Command)
			wantKind        command.CompletionKind
			wantCommandPath string
			wantExitCode    int
		}{
			{
				name:            "root help flag",
				args:            []string{"--help"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render",
			},
			{
				name:            "subcommand help flag",
				args:            []string{"test", "--help"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render test",
			},
			{
				name:            "help command targets subcommand",
				args:            []string{"help", "test"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render test",
			},
			{
				name:            "help command alone shows root help",
				args:            []string{"help"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render",
			},
			{
				// Cobra prints "Unknown help topic" and root usage without
				// rendering help, and the help command itself exits zero. The
				// user failed to name a command, so this is a discovery error.
				name:            "help command with unknown topic is a discovery error",
				args:            []string{"help", "bogus"},
				wantKind:        command.CompletionKindDiscoveryError,
				wantCommandPath: "render help",
			},
			{
				// Command discovery fails before Cobra ever parses --help.
				name:            "help flag on unknown command is a discovery error",
				args:            []string{"bogus", "--help"},
				wantKind:        command.CompletionKindDiscoveryError,
				wantCommandPath: "render",
				wantExitCode:    1,
			},
			{
				name:            "bare root shows help incidentally and is not classified as help",
				args:            []string{},
				wantKind:        command.CompletionKindSuccess,
				wantCommandPath: "render",
			},
			{
				name:            "bare non-runnable group shows help incidentally",
				args:            []string{"group"},
				wantKind:        command.CompletionKindSuccess,
				wantCommandPath: "render group",
				configure:       addNonRunnableGroup,
			},
			{
				name:            "explicit help for non-runnable group",
				args:            []string{"group", "--help"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render group",
				configure:       addNonRunnableGroup,
			},
			{
				// Cobra accepts the unmatched child before rendering group help and
				// exits zero. The user still named a command that does not exist, so
				// classify discovery while preserving Cobra's exit code and output.
				name:            "unknown subcommand of non-runnable group is a discovery error",
				args:            []string{"group", "bogus"},
				wantKind:        command.CompletionKindDiscoveryError,
				wantCommandPath: "render group",
				configure:       addNonRunnableGroup,
			},
			{
				name:            "unknown subcommand of nested group is a discovery error",
				args:            []string{"group", "sub", "bogus"},
				wantKind:        command.CompletionKindDiscoveryError,
				wantCommandPath: "render group sub",
				configure:       addNestedGroups,
			},
			{
				name:            "help flag after unknown subcommand is help",
				args:            []string{"group", "bogus", "--help"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render group",
				configure:       addNonRunnableGroup,
			},
			{
				name:            "help flag before unknown subcommand is help",
				args:            []string{"group", "--help", "bogus"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render group",
				configure:       addNonRunnableGroup,
			},
			{
				// `--` marks everything after it as data, so the token was never a
				// candidate subcommand.
				name:            "args after dash terminator are not an unknown subcommand",
				args:            []string{"group", "--", "bogus"},
				wantKind:        command.CompletionKindSuccess,
				wantCommandPath: "render group",
				configure:       addNonRunnableGroup,
			},
			{
				// The builtin help command resolves as much of the topic as it can
				// and renders help for the group, ignoring the trailing token.
				name:            "help command with unknown nested topic targets the group",
				args:            []string{"help", "group", "bogus"},
				wantKind:        command.CompletionKindHelp,
				wantCommandPath: "render group",
				configure:       addNonRunnableGroup,
			},
			{
				// The Runnable check keeps a manual Help call from a command's own
				// RunE classified as the success it is.
				name:            "runnable group rendering help in RunE is a success",
				args:            []string{"rgroup"},
				wantKind:        command.CompletionKindSuccess,
				wantCommandPath: "render rgroup",
				configure:       addRunnableGroup,
			},
			{
				// A NoArgs runnable group rejects the unknown child during arg
				// validation instead of rendering help. The user mistake is the
				// same as with a non-runnable group, so it classifies as
				// discovery — while keeping this shape's non-zero exit.
				name:            "unknown subcommand of NoArgs runnable group is a discovery error",
				args:            []string{"rgroup", "bogus"},
				wantKind:        command.CompletionKindDiscoveryError,
				wantCommandPath: "render rgroup",
				wantExitCode:    1,
				configure:       addRunnableGroup,
			},
		}

		for _, tc := range helpCases {
			t.Run(tc.name, func(t *testing.T) {
				result := executeAndClassify(t, tc.args, tc.configure)

				require.Equal(t, tc.wantKind, result.CompletionKind)
				require.NotEmpty(t, result.CommandPath)
				require.Equal(t, tc.wantCommandPath, result.CommandPath)
				require.Equal(t, tc.wantExitCode, result.ExitCode)
			})
		}
	})
}

// executeAndClassify executes a fresh root command with a runnable
// `test <value>` child and returns the classified result, mirroring Execute.
func executeAndClassify(t *testing.T, args []string, configure func(t *testing.T, root, child *cobra.Command)) command.ExecutionResult {
	t.Helper()

	root, _ := newRootCommandForUsageTests()
	child := &cobra.Command{
		Use:  "test <value>",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	}
	if configure != nil {
		configure(t, root, child)
	}
	root.AddCommand(child)
	root.SetArgs(args)

	observation := prepareExecutionObservation(root)
	startedAt := time.Now()
	executed, err := root.ExecuteC()
	return newClassifiedExecutionResult(executed, err, observation, startedAt)
}

func TestPrepareExecutionObservationClearsRetainedState(t *testing.T) {
	root, _ := newRootCommandForUsageTests()
	root.AddCommand(&cobra.Command{
		Use:  "test <value>",
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
	})

	root.SetArgs([]string{"test"})
	firstObservation := prepareExecutionObservation(root)
	executedCommand, err := root.ExecuteC()
	firstResult := newClassifiedExecutionResult(executedCommand, err, firstObservation, time.Now())
	require.Equal(t, command.CompletionKindValidationError, firstResult.CompletionKind)

	root.SetArgs([]string{"test", "value"})
	secondObservation := prepareExecutionObservation(root)
	require.Same(t, firstObservation, secondObservation)
	executedCommand, err = root.ExecuteC()
	secondResult := newClassifiedExecutionResult(executedCommand, err, secondObservation, time.Now())
	require.Equal(t, command.CompletionKindSuccess, secondResult.CompletionKind)
	require.Equal(t, setupSucceeded, secondObservation.setup)

	// Preparing again returns setup to its zero value; the classifier relies on
	// a fresh observation reading as not started.
	thirdObservation := prepareExecutionObservation(root)
	require.Same(t, secondObservation, thirdObservation)
	require.Equal(t, setupNotStarted, thirdObservation.setup)
	require.False(t, thirdObservation.launchedFullScreenTUI)
}

func TestClassifiedExecutionResultIncludesFullScreenTUILaunch(t *testing.T) {
	command := &cobra.Command{Use: "test"}
	observation := &executionObservation{launchedFullScreenTUI: true}

	result := newClassifiedExecutionResult(command, nil, observation, time.Now())

	require.True(t, result.LaunchedFullScreenTUI)
}

func TestPrepareExecutionObservationClearsRetainedHelpRequest(t *testing.T) {
	root, _ := newRootCommandForUsageTests()

	// Cobra retains parsed flag values when a command tree is reused. The E2E
	// harness and package tests reuse the root tree, so an explicit help request
	// from one execution must not affect classification of the next execution.
	root.SetArgs([]string{"--help"})
	firstObservation := prepareExecutionObservation(root)
	executedCommand, err := root.ExecuteC()
	firstResult := newClassifiedExecutionResult(executedCommand, err, firstObservation, time.Now())
	require.Equal(t, command.CompletionKindHelp, firstResult.CompletionKind)

	// Bare root renders help incidentally and is therefore success, not help.
	root.SetArgs(nil)
	secondObservation := prepareExecutionObservation(root)
	executedCommand, err = root.ExecuteC()
	secondResult := newClassifiedExecutionResult(executedCommand, err, secondObservation, time.Now())
	require.Equal(t, command.CompletionKindSuccess, secondResult.CompletionKind)
}

func TestPrepareExecutionObservationClearsRetainedUnknownSubcommand(t *testing.T) {
	root, _ := newRootCommandForUsageTests()
	root.AddCommand(&cobra.Command{Use: "group"})

	root.SetArgs([]string{"group", "bogus"})
	firstObservation := prepareExecutionObservation(root)
	executedCommand, err := root.ExecuteC()
	firstResult := newClassifiedExecutionResult(executedCommand, err, firstObservation, time.Now())
	require.Equal(t, command.CompletionKindDiscoveryError, firstResult.CompletionKind)

	root.SetArgs([]string{"group"})
	secondObservation := prepareExecutionObservation(root)
	executedCommand, err = root.ExecuteC()
	secondResult := newClassifiedExecutionResult(executedCommand, err, secondObservation, time.Now())
	require.Equal(t, command.CompletionKindSuccess, secondResult.CompletionKind)
}

var analyticsWorkspaceID = testids.WorkspaceID("analytics")

// TestCompletedCommandsEmitAnalytics drives real commands through the full
// classify-and-emit path against the renderapi fake and asserts on the events
// the server collected. Exhaustive per-outcome classification lives in
// TestExecutionResultClassifiesCobraOutcomes; this proves the wiring end to end
// and that the emitted command path is the verbatim, argument-free Cobra path.
func TestCompletedCommandsEmitAnalytics(t *testing.T) {
	testCases := []struct {
		name          string
		args          []string
		seedWorkspace bool
		wantCommand   string
		wantKind      telemetryclient.CliTelemetryEventPOSTInputCompletionKind
		wantExitCode  int
	}{
		{
			name:          "success",
			args:          []string{"postgres", "list", "--output", "json"},
			seedWorkspace: true,
			wantCommand:   "render postgres list",
			wantKind:      telemetryclient.Success,
		},
		{
			name:        "help",
			args:        []string{"postgres", "list", "--help"},
			wantCommand: "render postgres list",
			wantKind:    telemetryclient.Help,
		},
		{
			name:         "discovery error",
			args:         []string{"bogus"},
			wantCommand:  "render",
			wantKind:     telemetryclient.DiscoveryError,
			wantExitCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := renderapi.NewServer(t)
			if tc.seedWorkspace {
				server.Owners.Add(renderapi.NewOwner(client.Owner{Id: analyticsWorkspaceID, Name: "Analytics Workspace"}))
				t.Setenv("RENDER_WORKSPACE", analyticsWorkspaceID)
			}

			result := executeWithAnalytics(t, server, t.TempDir(), true, tc.args...)

			events := server.CliTelemetry.Instances
			require.Len(t, events, 1)
			require.Equal(t, tc.wantCommand, events[0].Command)
			require.Equal(t, tc.wantKind, events[0].CompletionKind)
			require.Equal(t, tc.wantExitCode, events[0].ExitCode)
			require.Equal(t, tc.wantExitCode, result.ExitCode)
		})
	}
}

func TestInstallationIDCreatedOnlyWhenAnalyticsEnabled(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: analyticsWorkspaceID, Name: "Analytics Workspace"}))
	t.Setenv("RENDER_WORKSPACE", analyticsWorkspaceID)
	configDir := t.TempDir()
	installationIDPath := filepath.Join(configDir, "state", "installation-id.txt")

	result := executeWithAnalytics(t, server, configDir, false, "postgres", "list", "--output", "json")
	require.Equal(t, 0, result.ExitCode)
	require.Empty(t, server.CliTelemetry.Instances, "disabled analytics should not emit an event")
	_, err := os.Stat(installationIDPath)
	require.ErrorIs(t, err, os.ErrNotExist, "disabled analytics should not create installation ID state")

	result = executeWithAnalytics(t, server, configDir, true, "postgres", "list", "--output", "json")
	require.Equal(t, 0, result.ExitCode)
	require.Len(t, server.CliTelemetry.Instances, 1)
	contents, err := os.ReadFile(installationIDPath)
	require.NoError(t, err)
	installationID := strings.TrimSpace(string(contents))
	require.NotEmpty(t, installationID)
	require.Equal(t, installationID, server.CliTelemetry.Instances[0].InstallationId,
		"the emitted event should carry the persisted installation ID")
}

// executeWithAnalytics builds a fresh CLI app whose client and analytics sender
// both target the fake, runs args through the same runExecution/onExecutionComplete
// path Execute uses, and returns the classified result. The emitted event lands
// on server.CliTelemetry.
func executeWithAnalytics(
	t *testing.T,
	server *renderapi.Server,
	configDir string,
	shouldSend bool,
	args ...string,
) command.ExecutionResult {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	t.Setenv("RENDER_CLI_CONFIG_PATH", "")
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_HOST", server.URL()+"/")
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "sync")
	t.Setenv(analyticsSubprocessHelperEnv, "1")
	if shouldSend {
		t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", "1")
	} else {
		t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", "")
	}

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{}, nil
	}

	root := newRootCmd()
	setupPGCommands(root, deps)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	result := runExecution(root, time.Now())
	onExecutionComplete(result, deps, root)
	return result
}

func newRootCommandForUsageTests() (*cobra.Command, *bytes.Buffer) {
	deps := dependencies.New(nil)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{
			StdinTTY:  true,
			StdoutTTY: false,
			StderrTTY: true,
		}, nil
	}

	root := &cobra.Command{
		Use:               "render",
		PersistentPreRunE: rootPersistentPreRun(deps),
	}
	observeCobraValidationAndHelp(root)
	root.PersistentFlags().StringP("output", "o", "interactive", "interactive, json, yaml, or text")
	root.PersistentFlags().Bool(command.ConfirmFlag, false, "set to skip confirmation prompts")

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)

	return root, &out
}

func TestExecuteInvokesOnExecutionComplete(t *testing.T) {
	// These cases pin callback delivery, not classification (that is
	// TestExecutionResultClassifiesCobraOutcomes' job). Delivery has exactly two
	// flavors — Cobra returned nil or an error — and the error case matters most:
	// Cobra skips post-run hooks on error, which is why completion lives in
	// Execute rather than a hook.
	testCases := []struct {
		name         string
		args         []string
		wantKind     command.CompletionKind
		wantExitCode int
	}{
		{
			name:         "nil-error path",
			args:         []string{"render", "--help"},
			wantKind:     command.CompletionKindHelp,
			wantExitCode: 0,
		},
		{
			name:         "error path",
			args:         []string{"render", "command-that-does-not-exist"},
			wantKind:     command.CompletionKindDiscoveryError,
			wantExitCode: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var callbackParams []command.ExecutionResult
			spy := func(result command.ExecutionResult, _ *dependencies.Dependencies, _ *cobra.Command) {
				callbackParams = append(callbackParams, result)
			}
			originalCallback := onExecutionComplete
			onExecutionComplete = spy
			t.Cleanup(func() { onExecutionComplete = originalCallback })

			// Let SetupCommands build a Render API client without config or network access.
			t.Setenv("RENDER_API_KEY", "test-api-key")
			t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))
			t.Setenv("RENDER_LOG_ANALYTICS", "")

			originalArgs := os.Args
			os.Args = tc.args
			t.Cleanup(func() { os.Args = originalArgs })

			var out bytes.Buffer
			rootCmd.SetOut(&out)
			rootCmd.SetErr(&out)
			t.Cleanup(func() {
				rootCmd.SetOut(nil)
				rootCmd.SetErr(nil)
			})

			exitCode := Execute()

			require.Equal(t, tc.wantExitCode, exitCode)
			require.Len(t, callbackParams, 1)
			require.Equal(t, tc.wantKind, callbackParams[0].CompletionKind)
			require.Equal(t, exitCode, callbackParams[0].ExitCode)
		})
	}
}

func TestCombinedFlagUsagesIncludesDefaultValue(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("output", "interactive", "Output format.")

	got := CombinedFlagUsages(flags, nil)

	require.Contains(t, got, `(default: "interactive")`)
}

func TestCombinedFlagUsagesIncludesSingleSpaceStringDefault(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("delimiter", " ", "Delimiter to use.")

	got := CombinedFlagUsages(flags, nil)

	require.Contains(t, got, `(default: " ")`)
}

func TestCombinedFlagUsagesIncludesDeprecationText(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("old-flag", "", "Old flag.")
	flags.Lookup("old-flag").Deprecated = "use --new-flag instead"

	got := CombinedFlagUsages(flags, nil)

	require.Contains(t, got, "(DEPRECATED: use --new-flag instead)")
}

func TestCombinedFlagUsagesIncludesZeroNumericAndDurationDefaults(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Int("limit", 0, "Max records.")
	flags.Duration("timeout", 0*time.Second, "Request timeout.")

	got := CombinedFlagUsages(flags, nil)

	require.Contains(t, got, "--limit")
	require.Contains(t, got, "(default: 0)")
	require.Contains(t, got, "--timeout")
	require.Contains(t, got, "(default: 0s)")
}

func TestRootServicesHelpOmitsBoolNoArgSuffix(t *testing.T) {
	root := &cobra.Command{
		Use:   "render",
		Short: "Render root",
	}
	root.SetHelpTemplate(CustomHelpTemplate)
	root.PersistentFlags().Bool(command.ConfirmFlag, false, "Skip all confirmation prompts")

	services := &cobra.Command{
		Use:   "services",
		Short: "List services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	root.AddCommand(services)

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"services"})

	require.NoError(t, root.Execute())

	helpOutput := stripANSI(out.String())
	require.Contains(t, helpOutput, "--help")
	require.NotContains(t, helpOutput, "--help[=true|false]")
	require.NotContains(t, helpOutput, "--confirm[=true|false]")
}

func TestRootHelpOmitsBoolNoArgSuffix(t *testing.T) {
	root := &cobra.Command{
		Use:   "render",
		Short: "Render root",
	}
	root.SetHelpTemplate(CustomHelpTemplate)
	root.PersistentFlags().Bool(command.ConfirmFlag, false, "Skip all confirmation prompts")

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})

	require.NoError(t, root.Execute())

	helpOutput := stripANSI(out.String())
	require.Contains(t, helpOutput, "--help")
	require.NotContains(t, helpOutput, "--help[=true|false]")
	require.NotContains(t, helpOutput, "--confirm[=true|false]")
}

func TestRootHelpOmitsEmptyGroupHeaders(t *testing.T) {
	root := &cobra.Command{
		Use:   "render",
		Short: "Render root",
	}
	root.SetHelpTemplate(CustomHelpTemplate)
	root.AddGroup(&cobra.Group{ID: "core", Title: "Core"})
	root.AddGroup(&cobra.Group{ID: "empty", Title: "Unused Group"})
	root.AddCommand(&cobra.Command{Use: "services", Short: "List services", GroupID: "core", Run: func(_ *cobra.Command, _ []string) {}})

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"--help"})

	require.NoError(t, root.Execute())

	helpOutput := stripANSI(out.String())
	require.Contains(t, helpOutput, "Core")
	require.NotContains(t, helpOutput, "Unused Group")
}

func TestRootGeneratesShellCompletions(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		t.Run(shell, func(t *testing.T) {
			root := newRootCmd()
			root.PersistentPostRunE = nil
			root.AddCommand(&cobra.Command{
				Use:     "placeholder",
				GroupID: GroupCore.ID,
				Run:     func(*cobra.Command, []string) {},
			})

			var output bytes.Buffer
			root.SetOut(&output)
			root.SetErr(&output)
			root.SetArgs([]string{"completion", shell})

			require.NoError(t, root.Execute())
			require.NotEmpty(t, output.String())
		})
	}
}

func TestRootSuppressesFileCompletionByDefault(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "resource positional", args: []string{"workspace", "set", ""}},
		{name: "command without positionals", args: []string{"logs", ""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requireCompletionDirective(t, tc.args, cobra.ShellCompDirectiveNoFileComp)
		})
	}
}

// requireCompletionDirective asserts Cobra's machine-readable completion
// protocol. The numeric directive is the final stdout line; Cobra separately
// prints the corresponding Go constant names to stderr as diagnostic text.
func requireCompletionDirective(t *testing.T, args []string, want cobra.ShellCompDirective) {
	t.Helper()

	output := strings.TrimSpace(runCompletionRequest(t, args))
	lines := strings.Split(output, "\n")
	trailer := lines[len(lines)-1]
	encoded, ok := strings.CutPrefix(trailer, ":")
	require.True(t, ok, "completion output must end with a directive trailer")
	directive, err := strconv.Atoi(encoded)
	require.NoError(t, err)
	require.Equal(t, want, cobra.ShellCompDirective(directive))
}

// runCompletionRequest executes `render __complete <args...>` and returns the
// completion output, including the directive trailer Cobra prints for shells.
func runCompletionRequest(t *testing.T, args []string) string {
	t.Helper()

	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_CLI_CONFIG_PATH", filepath.Join(t.TempDir(), "cli.yaml"))
	t.Setenv("RENDER_LOG_ANALYTICS", "")

	originalArgs := os.Args
	os.Args = append([]string{"render", "__complete"}, args...)
	t.Cleanup(func() { os.Args = originalArgs })

	var stdout, diagnostics bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&diagnostics)
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	require.Equal(t, 0, Execute())
	return stdout.String()
}

func TestGetDescriptiveTypeNameUsesAnnotationWhenPresent(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("region", "", "Filter by region.")
	setFlagPlaceholder(flags, "region", "REGION")

	require.Equal(t, "REGION", getDescriptiveTypeName(flags.Lookup("region"), "string"))
	require.Equal(t, "string", getDescriptiveTypeName(flags.Lookup("missing"), "string"))
}

func TestRootOutputFlagHasPlaceholderAnnotation(t *testing.T) {
	outputFlag := rootCmd.PersistentFlags().Lookup("output")
	require.NotNil(t, outputFlag)

	placeholder, ok := placeholderFromAnnotation(outputFlag)
	require.True(t, ok)
	require.Equal(t, command.OutputPlaceholder, placeholder)
}

func TestServicesEnvironmentIDsFlagHasPlaceholderAnnotation(t *testing.T) {
	envIDsFlag := servicesCmd.Flags().Lookup("environment-ids")
	require.NotNil(t, envIDsFlag)

	placeholder, ok := placeholderFromAnnotation(envIDsFlag)
	require.True(t, ok)
	require.Equal(t, placeholderEnvIDs, placeholder)
}

func requireFlagPlaceholder(t *testing.T, flags *pflag.FlagSet, flagName, expected string) {
	t.Helper()

	require.NotNil(t, flags)
	flag := flags.Lookup(flagName)
	require.NotNil(t, flag)
	placeholder, ok := placeholderFromAnnotation(flag)
	require.True(t, ok)
	require.Equal(t, expected, placeholder)
}

func TestSetAllFlagPlaceholders(t *testing.T) {
	t.Run("applies placeholders for all value flags", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("region", "", "region")
		cmd.Flags().String("plan", "", "plan")

		setAllFlagPlaceholders(cmd, map[string]string{
			"region": "REGION",
			"plan":   "PLAN",
		})

		requireFlagPlaceholder(t, cmd.Flags(), "region", "REGION")
		requireFlagPlaceholder(t, cmd.Flags(), "plan", "PLAN")
	})

	t.Run("requires placeholders for local value flags only", func(t *testing.T) {
		root := &cobra.Command{Use: "root"}
		root.PersistentFlags().String("output", "", "output")
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("region", "", "region")
		cmd.Flags().Bool("confirm", false, "confirm")
		cmd.Flags().String("hidden", "", "hidden")
		require.NoError(t, cmd.Flags().MarkHidden("hidden"))
		root.AddCommand(cmd)

		setAllFlagPlaceholders(cmd, map[string]string{
			"region": "REGION",
		})

		requireFlagPlaceholder(t, cmd.Flags(), "region", "REGION")
	})

	t.Run("panics when a value flag is missing a placeholder", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("region", "", "region")
		cmd.Flags().String("plan", "", "plan")

		require.Panics(t, func() {
			setAllFlagPlaceholders(cmd, map[string]string{
				"region": "REGION",
			})
		})
	})

	t.Run("panics when any flag does not exist", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}

		require.Panics(t, func() {
			setAllFlagPlaceholders(cmd, map[string]string{
				"missing": "MISSING",
			})
		})
	})

	t.Run("panics for empty placeholder set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}

		require.Panics(t, func() {
			setAllFlagPlaceholders(cmd, map[string]string{})
		})
	})

	t.Run("panics for nil placeholder set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}

		require.Panics(t, func() {
			setAllFlagPlaceholders(cmd, nil)
		})
	})

	t.Run("panics for nil command", func(t *testing.T) {
		require.Panics(t, func() {
			setAllFlagPlaceholders(nil, map[string]string{
				"region": "REGION",
			})
		})
	})
}

func TestSetAnnotationBestEffort(t *testing.T) {
	t.Run("returns true for existing flag", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.String("output", "", "output format")

		ok := setAnnotationBestEffort(flags, "output", "test.annotation", []string{"FORMAT"})
		require.True(t, ok)
		require.Equal(t, []string{"FORMAT"}, flags.Lookup("output").Annotations["test.annotation"])
	})

	t.Run("returns false for missing flag", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

		require.NotPanics(t, func() {
			ok := setAnnotationBestEffort(flags, "missing", flagPlaceholderAnnotation, []string{"FORMAT"})
			require.False(t, ok)
		})
	})

	t.Run("returns false for nil flagset", func(t *testing.T) {
		require.False(t, setAnnotationBestEffort(nil, "output", flagPlaceholderAnnotation, []string{"FORMAT"}))
	})
}

func TestSetFlagPlaceholder(t *testing.T) {
	t.Run("applies placeholder to existing flag", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.String("output", "", "output format")

		require.True(t, setFlagPlaceholder(flags, "output", "FORMAT"))

		requireFlagPlaceholder(t, flags, "output", "FORMAT")
	})

	t.Run("best-effort: returns false for missing flag without panicking", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

		require.NotPanics(t, func() {
			require.False(t, setFlagPlaceholder(flags, "missing", "FORMAT"))
		})
	})

	t.Run("best-effort: returns false for nil flagset without panicking", func(t *testing.T) {
		require.NotPanics(t, func() {
			require.False(t, setFlagPlaceholder(nil, "output", "FORMAT"))
		})
	})
}

func TestIsRootVersionRequest(t *testing.T) {
	flags := pflag.NewFlagSet("root", pflag.ContinueOnError)
	flags.StringP("output", "o", "interactive", "")
	flags.Bool(command.ConfirmFlag, false, "")

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"empty args", nil, false},
		{"no version", []string{"services", "list"}, false},
		{"bare --version", []string{"--version"}, true},
		{"bare -v", []string{"-v"}, true},
		{"version after global -o flag", []string{"-o", "text", "--version"}, true},
		{"version after --output= flag", []string{"--output=text", "--version"}, true},
		{"version after --confirm bool flag", []string{"--confirm", "--version"}, true},
		{"version after subcommand should not match", []string{"pg", "create", "--version", "17"}, false},
		{"output alone", []string{"--output=text"}, false},
		{"version flag form via equals", []string{"--version=true"}, true},
		{"multi-char single-dash arg falls through to cobra", []string{"-output", "text", "--version"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isRootVersionRequest(tc.args, flags))
		})
	}
}

func stripANSI(input string) string {
	ansiEscapePattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiEscapePattern.ReplaceAllString(input, "")
}
