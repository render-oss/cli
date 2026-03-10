// pattern: Imperative Shell
package cmd

import (
	"bytes"
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

func TestCombinedFlagUsagesIncludesDefaultValue(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("output", "interactive", "Output format.")

	got := CombinedFlagUsages(flags, nil)

	require.Contains(t, got, `(default "interactive")`)
}

func TestCombinedFlagUsagesIncludesSingleSpaceStringDefault(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("delimiter", " ", "Delimiter to use.")

	got := CombinedFlagUsages(flags, nil)

	require.Contains(t, got, `(default " ")`)
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
	require.Contains(t, got, "(default 0)")
	require.Contains(t, got, "--timeout")
	require.Contains(t, got, "(default 0s)")
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

func TestGetDescriptiveTypeNameUsesAnnotationWhenPresent(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("region", "", "Filter by region.")
	require.NoError(t, flags.SetAnnotation("region", command.FlagPlaceholderAnnotation, []string{"REGION"}))

	require.Equal(t, "REGION", getDescriptiveTypeName(flags.Lookup("region"), "string"))
	require.Equal(t, "string", getDescriptiveTypeName(flags.Lookup("missing"), "string"))
}

func TestRootOutputFlagHasPlaceholderAnnotation(t *testing.T) {
	outputFlag := rootCmd.PersistentFlags().Lookup("output")
	require.NotNil(t, outputFlag)

	values, ok := outputFlag.Annotations[command.FlagPlaceholderAnnotation]
	require.True(t, ok)
	require.Equal(t, []string{command.OutputPlaceholder}, values)
}

func TestServicesEnvironmentIDsFlagHasPlaceholderAnnotation(t *testing.T) {
	envIDsFlag := servicesCmd.Flags().Lookup("environment-ids")
	require.NotNil(t, envIDsFlag)

	values, ok := envIDsFlag.Annotations[command.FlagPlaceholderAnnotation]
	require.True(t, ok)
	require.Equal(t, []string{placeholderEnvIDs}, values)
}

func TestSetAnnotationBestEffort(t *testing.T) {
	t.Run("returns true for existing flag", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
		flags.String("output", "", "output format")

		ok := setAnnotationBestEffort(flags, "output", command.FlagPlaceholderAnnotation, []string{"FORMAT"})
		require.True(t, ok)
		require.Equal(t, []string{"FORMAT"}, flags.Lookup("output").Annotations[command.FlagPlaceholderAnnotation])
	})

	t.Run("returns false for missing flag", func(t *testing.T) {
		flags := pflag.NewFlagSet("test", pflag.ContinueOnError)

		require.NotPanics(t, func() {
			ok := setAnnotationBestEffort(flags, "missing", command.FlagPlaceholderAnnotation, []string{"FORMAT"})
			require.False(t, ok)
		})
	})

	t.Run("returns false for nil flagset", func(t *testing.T) {
		require.False(t, setAnnotationBestEffort(nil, "output", command.FlagPlaceholderAnnotation, []string{"FORMAT"}))
	})
}

func stripANSI(input string) string {
	ansiEscapePattern := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	return ansiEscapePattern.ReplaceAllString(input, "")
}
