package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/client/version"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	renderstyle "github.com/render-oss/cli/pkg/style"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/tui/views"
)

var longHelp = `Welcome! Use the Render CLI to manage your services, datastores, and environments directly from the command line. Trigger deploys, view logs, start psql/SSH sessions, and more.

The CLI's default interactive mode provides intuitive, menu-based navigation.

To use in non-interactive mode (such as in a script), set each command's --output option to either json or yaml for structured responses. The CLI also detects non-TTY stdout and automatically switches to text output.
`

// rootCmd represents the base command when called without any subcommands.
var rootCmd = newRootCmd()

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "render",
		Short: "Interact with resources on Render",
		Long:  longHelp,
		Example: `  # List services in the active workspace
  render services

  # Output services as JSON
  render services --output json`,

		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			deps := dependencies.GetFromContext(ctx)

			output := command.GetFormatFromContext(ctx)
			if output.Interactive() {
				stack := tui.GetStackFromContext(ctx)
				if stack.IsEmpty() {
					return nil
				}

				var m tea.Model = stack

				local := isLocalCommand(cmd)

				if !local && cmd.Name() != deps.Commands.WorkspaceSetCmd().Name() {
					if !config.IsWorkspaceSet() {
						m = tui.NewConfigWrapper(m, "Set Workspace", views.NewWorkspaceView(ctx, views.ListWorkspaceInput{}))
					}
				}

				if !local && cmd.Name() != loginCmd.Name() {
					m = tui.NewConfigWrapper(m, "Login", views.NewLoginView(ctx))
				}

				_, err := runFullScreenTUI(ctx, m)
				if err != nil {
					panic(fmt.Sprintf("Failed to initialize interface. Use -o to specify a non-interactive output mode: %v", err))
				}
				return nil
			}

			return nil
		},
	}

	root.AddGroup(AllGroups...)
	root.SetHelpTemplate(CustomHelpTemplate)
	root.Version = cfg.Version
	// Most positional args are Render resources, not filesystem paths, so
	// don't fall back to file completion. Commands that do take paths opt
	// back in with ShellCompDirectiveDefault at their definition site.
	root.CompletionOptions.SetDefaultShellCompDirective(cobra.ShellCompDirectiveNoFileComp)
	root.PersistentFlags().StringP("output", "o", "interactive", "Set output format to interactive, json, yaml, or text. Auto-switches to text on non-TTY")
	setFlagPlaceholder(root.PersistentFlags(), "output", command.OutputPlaceholder)
	root.PersistentFlags().Bool(command.ConfirmFlag, false, "Skip all confirmation prompts")
	observeCobraValidationAndHelp(root)

	// Flags from the old CLI that we error with a helpful message.
	root.PersistentFlags().Bool("pretty-json", false, "")
	if err := root.PersistentFlags().MarkHidden("pretty-json"); err != nil {
		panic(err)
	}
	root.PersistentFlags().Bool("json-record-per-line", false, "")
	if err := root.PersistentFlags().MarkHidden("json-record-per-line"); err != nil {
		panic(err)
	}

	return root
}

func isLocalCommand(cmd *cobra.Command) bool {
	v, err := cmd.Flags().GetBool("local")
	if err != nil {
		// Flag doesn't exist or isn't a bool
		return false
	}
	return v
}

func setupWorkflowCommands(deps *dependencies.Dependencies) {
	deps.Commands.Workflow.TaskListCmd = NewTaskListCmd(deps)
	deps.Commands.Workflow.RunStartCmd = NewRunStartCmd(deps)
	deps.Commands.Workflow.RunListCmd = NewRunListCmd(deps)
	deps.Commands.Workflow.RunDetailsCmd = NewRunDetailsCmd(deps)
	deps.Commands.Workflow.RunCancelCmd = NewRunCancelCmd(deps)
	deps.Commands.Workflow.VersionListCmd = NewVersionListCmd(deps)
	deps.Commands.Workflow.VersionReleaseCmd = NewVersionReleaseCmd(deps)
	deps.Commands.Workflow.WorkflowListCmd = NewWorkflowListCmd(deps)
	WorkflowsCmd.AddCommand(deps.Commands.Workflow.WorkflowListCmd)

	deps.Commands.Workflow.WorkflowCreateCmd = WorkflowCreateCmd
	WorkflowsCmd.AddCommand(deps.Commands.Workflow.WorkflowCreateCmd)

	taskCmd.AddCommand(deps.Commands.Workflow.TaskListCmd)
	tasksRunsCmd.AddCommand(deps.Commands.Workflow.RunStartCmd)
	tasksRunsCmd.AddCommand(deps.Commands.Workflow.RunListCmd)
	tasksRunsCmd.AddCommand(deps.Commands.Workflow.RunDetailsCmd)
	tasksRunsCmd.AddCommand(deps.Commands.Workflow.RunCancelCmd)

	WorkflowsCmd.AddCommand(workflowStartShortcut(deps))
	WorkflowsCmd.AddCommand(workflowCancelShortcut(deps))

	taskCmd.AddCommand(deprecatedTaskStartCmd(deps))
	runsCmd.AddCommand(deprecatedRunListCmd(deps))
	runsCmd.AddCommand(deprecatedRunDetailsCmd(deps))
	runsCmd.AddCommand(deprecatedRunCancelCmd(deps))

	versionCmd.AddCommand(deps.Commands.Workflow.VersionListCmd)
	versionCmd.AddCommand(deps.Commands.Workflow.VersionReleaseCmd)
}

func setupLogCommands(deps *dependencies.Dependencies) {
	deps.Commands.Logs.LogsCmd = NewLogsCmd(deps)

	rootCmd.AddCommand(deps.Commands.Logs.LogsCmd)
}

func setupWorkspaceCommands(deps *dependencies.Dependencies) {
	deps.Commands.Workspace.WorkspaceSetCmd = WorkspaceSetCmd(deps)

	workspaceCmd.AddCommand(deps.Commands.Workspace.WorkspaceSetCmd)
}

func setupPGCommands(parent *cobra.Command, deps *dependencies.Dependencies) {
	parent.AddCommand(newPgCmd(newPgCreateCmd(deps), newPgDeleteCmd(deps), newPgGetCmd(deps), newPgListCmd(deps), newPgUpdateCmd(deps), newPgSuspendCmd(deps), newPgResumeCmd(deps)))
}

func setupKVCommands(parent *cobra.Command, deps *dependencies.Dependencies) {
	parent.AddCommand(newKVCmd(newKVCreateCmd(deps), newKVDeleteCmd(deps), newKVGetCmd(deps), newKVListCmd(deps), newKVResumeCmd(deps), newKVSuspendCmd(deps), newKVUpdateCmd(deps)))
}

func setupSandboxCommands(earlyAccess *cobra.Command, deps *dependencies.Dependencies) {
	earlyAccess.AddCommand(newSandboxCmd(newSandboxCreateCmd(deps), newSandboxCopyCmd(deps), newSandboxExecCmd(deps), newSandboxListCmd(deps), newSandboxStopCmd(deps)))
}

func setupSandboxGroupsCommands(earlyAccess *cobra.Command, deps *dependencies.Dependencies) {
	earlyAccess.AddCommand(newSandboxGroupsCmd(newSandboxGroupsListCmd(deps)))
}

func setupServiceCommands(deps *dependencies.Dependencies) {
	servicesCmd.AddCommand(newServiceDeleteCmd(deps), newServiceUpdateCmd(deps))
}

// SetupCommands constructs and registers all CLI commands.
func SetupCommands() error {
	_, err := setupCommands()
	return err
}

// setupCommands constructs dependencies and registers all CLI commands.
func setupCommands() (*dependencies.Dependencies, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		if errors.Is(err, config.ErrLogin) {
			c, err = client.NotLoggedInClient()
			if err != nil {
				return nil, fmt.Errorf("failed to create client: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to create client: %w", err)
		}
	}

	deps := dependencies.New(c)

	setupWorkflowCommands(deps)
	setupAnalyticsCommands(rootCmd, deps)
	setupLogCommands(deps)
	setupWorkspaceCommands(deps)
	setupServiceCommands(deps)
	setupKVCommands(rootCmd, deps)
	setupPGCommands(rootCmd, deps)
	setupSandboxCommands(EarlyAccessCmd, deps)
	setupSandboxGroupsCommands(EarlyAccessCmd, deps)
	setupRootCmdPersistentRun(rootCmd, deps)

	return deps, nil
}

func setupRootCmdPersistentRun(root *cobra.Command, deps *dependencies.Dependencies) {
	root.PersistentPreRunE = rootPersistentPreRun(deps)
}

func rootPersistentPreRun(deps *dependencies.Dependencies) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) (err error) {
		ctx := cmd.Context()
		observation := executionObservationFromContext(ctx)
		if observation != nil {
			observation.setup = setupStarted
			// Records the setup outcome on every exit, success included.
			defer func() {
				if err != nil {
					observation.setup = setupFailed
				} else {
					observation.setup = setupSucceeded
				}
			}()
		}

		if err := checkForDeprecatedFlagUsage(cmd); err != nil {
			return err
		}

		// Cobra has already parsed flags/args and selected the command by this point.
		// Suppress usage for errors that happen during command execution.
		cmd.SilenceUsage = true

		confirmFlag, err := cmd.Flags().GetBool(command.ConfirmFlag)
		if err != nil {
			panic(err)
		}

		ctx = command.SetConfirmInContext(ctx, confirmFlag)

		outputFlag, err := cmd.Flags().GetString("output")
		if err != nil {
			panic(err)
		}

		requestedOutput, err := command.StringToOutput(outputFlag)
		if err != nil {
			return printRootPreRunError(cmd, err)
		}

		explicitOutputSet := cmd.Flags().Changed("output")
		signals, err := deps.DetectRuntimeSignals()
		if err != nil {
			return printRootPreRunError(cmd, err)
		}
		output, err := command.ResolveAutoOutput(explicitOutputSet, requestedOutput, signals)
		if err != nil {
			return printRootPreRunError(cmd, err)
		}

		ctx = command.SetFormatInContext(ctx, &output)

		deps.SetStack(tui.NewStack())
		// Setting the dependencies is necessary for now, but we should move to
		// wrapping commands in functions that provide the necessary dependencies.
		ctx = dependencies.SetInContext(ctx, deps)

		if output.Interactive() {
			ctx = tui.SetStackInContext(ctx, deps.Stack())
		}

		cmd.SetContext(ctx)

		return nil
	}
}

// printRootPreRunError prints a setup error before returning it to Cobra.
func printRootPreRunError(cmd *cobra.Command, err error) error {
	printError(cmd, err)
	// The error has already been printed above. Returning it still causes Cobra execution
	// to fail and the top-level executor to return exit code 1.
	cmd.Root().SilenceErrors = true
	return err
}

func printError(cmd *cobra.Command, err error) {
	_, writeErr := fmt.Fprintln(cmd.ErrOrStderr(), err)
	if writeErr != nil {
		panic(writeErr)
	}
}

// printVersionWithUpdateCheck prints the current version and checks for updates
func printVersionWithUpdateCheck() {
	fmt.Printf("render v%s\n", cfg.Version)

	vc := version.NewClient(cfg.RepoURL)
	newVersion, err := vc.NewVersionAvailable()
	if err == nil && newVersion != "" {
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Foreground(renderstyle.ColorWarning).
			Render(fmt.Sprintf("A new version is available: %s\n\nTo upgrade, see: %s",
				renderstyle.Bold("v"+newVersion),
				cfg.InstallationInstructionsURL)))
	} else if err == nil {
		fmt.Println()
		fmt.Println(lipgloss.NewStyle().Foreground(renderstyle.ColorOK).
			Render("You are using the latest version"))
	}
}

// isRootVersionRequest reports whether --version / -v appears among the
// root-level arguments, before the first subcommand token. It uses rootFlags to
// skip values consumed by global flags such as `-o text`.
func isRootVersionRequest(args []string, rootFlags *pflag.FlagSet) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--version" || arg == "-v" || strings.HasPrefix(arg, "--version=") {
			return true
		}
		if !strings.HasPrefix(arg, "-") || arg == "--" {
			return false
		}
		if strings.Contains(arg, "=") {
			continue
		}

		var flag *pflag.Flag
		if strings.HasPrefix(arg, "--") {
			flag = rootFlags.Lookup(strings.TrimPrefix(arg, "--"))
		} else {
			short := strings.TrimPrefix(arg, "-")
			if len(short) != 1 {
				return false
			}
			flag = rootFlags.ShorthandLookup(short)
		}
		if flag == nil {
			return false
		}
		if flag.Value.Type() != "bool" && flag.NoOptDefVal == "" {
			i++
		}
	}
	return false
}

// exitCodeFromError maps Cobra's result to the process exit code. Nil is the only
// successful result; errors carrying a nonzero ExitCode preserve that exact code,
// while all other errors map to 1.
func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitCoder command.ExitCoder
	if errors.As(err, &exitCoder) {
		if exitCode := exitCoder.ExitCode(); exitCode != 0 {
			return exitCode
		}
	}
	return 1
}

// Execute is the public entry point for the cmd package.
// It configures and runs the root command, then returns its process exit code to main.
// Every execution funnels through the single onExecutionComplete call here, no
// matter which path inside execute produced the result.
func Execute() int {
	result, deps := execute()
	onExecutionComplete(result, deps, rootCmd)
	return result.ExitCode
}

// execute runs one CLI execution and returns its classified result.
func execute() (command.ExecutionResult, *dependencies.Dependencies) {
	startedAt := time.Now()

	// Resolve --version before building the client. Only treat --version / -v as
	// the CLI's global version flag when it appears before a subcommand;
	// otherwise let subcommands own their flags. setupCommands builds the API
	// client, whose construction can trigger a synchronous OAuth token refresh
	// (NewDefaultClient -> maybeRefreshAPIToken) that is unbounded and could hang
	// --version on a bad network. Returning here also means version requests emit
	// no analytics — an accepted trade-off, signalled by the nil dependencies.
	if isRootVersionRequest(os.Args[1:], rootCmd.PersistentFlags()) {
		printVersionWithUpdateCheck()
		return newExecutionResult(rootCmd, command.CompletionKindVersion, 0, startedAt), nil
	}

	deps, err := setupCommands()
	if err != nil {
		printError(rootCmd, err)
		return newExecutionResult(rootCmd, command.CompletionKindSetupError, 1, startedAt), nil
	}

	return runExecution(rootCmd, startedAt), deps
}

func init() {
	// Template functions are global to cobra. Help template registration is
	// per-command and lives in newRootCmd.
	cobra.AddTemplateFunc("combinedFlagUsages", CombinedFlagUsages)
	cobra.AddTemplateFunc("wrapText", wrapText)
	cobra.AddTemplateFunc("cliVersion", cliVersion)
	cobra.AddTemplateFunc("boldText", renderstyle.Bold)
	cobra.AddTemplateFunc("formatExamples", formatExamples)
	cobra.AddTemplateFunc("getUsageArgs", getUsageArgs)
	cobra.AddTemplateFunc("hasVisibleGroupCommands", hasVisibleGroupCommands)
	cobra.AddTemplateFunc("trimPeriod", trimTrailingPeriod)
	cobra.AddTemplateFunc("groupHeader", groupHeaderText)
}

// checkForDeprecatedFlagUsage checks for usage of deprecated flags and returns an error with the new flag if found.
// These can be removed after a few months.
func checkForDeprecatedFlagUsage(cmd *cobra.Command) error {
	prettyFlag, err := cmd.Flags().GetBool("pretty-json")
	if err == nil && prettyFlag {
		return errors.New("use `--output json` instead of `--pretty-json`")
	}

	recordPerLineFlag, err := cmd.Flags().GetBool("json-record-per-line")
	if err == nil && recordPerLineFlag {
		return errors.New("use `--output json` instead of `--json-record-per-line`")
	}

	// used in services command
	serviceID, err := cmd.Flags().GetString("service-id")
	if err == nil && serviceID != "" {
		return errors.New("provide service ID as an argument instead of using the --service-id flag")
	}

	return nil
}

// RootCmd is set to export the root command for use in tests
var RootCmd = rootCmd
