/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

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

var welcomeMsg = lipgloss.NewStyle().Bold(true).Foreground(renderstyle.ColorFocus).
	Render("Render CLI v" + cfg.Version)

var longHelp = fmt.Sprintf(`%s

Welcome! Use the Render CLI to manage your services, datastores, and
environments directly from the command line. Trigger deploys, view logs,
start psql/SSH sessions, and more.

The CLI's default %s mode provides intuitive, menu-based navigation.

To use in %s mode (such as in a script), you can set each command's --output
option to either json or yaml for structured responses. We'll also detect if stdout 
is not a TTY and automatically switch to json output.
`, welcomeMsg, renderstyle.Bold("interactive"), renderstyle.Bold("non-interactive"))

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "render",

	Short: "Interact with resources on Render",
	Long:  longHelp,

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

			if cmd.Name() != deps.Commands.WorkspaceSetCmd().Name() {
				if !config.IsWorkspaceSet() {
					m = tui.NewConfigWrapper(m, "Set Workspace", views.NewWorkspaceView(ctx, views.ListWorkspaceInput{}))
				}
			}

			if cmd.Name() != loginCmd.Name() {
				m = tui.NewConfigWrapper(m, "Login", views.NewLoginView(ctx))
			}

			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err := p.Run()
			if err != nil {
				panic(fmt.Sprintf("Failed to initialize interface. Use -o to specify a non-interactive output mode: %v", err))
			}
			return nil
		}

		return nil
	},
}

func setupWorkflowCommands(deps *dependencies.Dependencies) {
	deps.Commands.Workflow.TaskListCmd = NewTaskListCmd(deps)
	deps.Commands.Workflow.TaskRunCmd = NewTaskRunStartCmd(deps)
	deps.Commands.Workflow.TaskRunListCmd = NewTaskRunListCmd(deps)
	deps.Commands.Workflow.TaskRunDetailsCmd = NewTaskRunDetailsCmd(deps)
	deps.Commands.Workflow.VersionListCmd = NewVersionListCmd(deps)
	deps.Commands.Workflow.VersionReleaseCmd = NewVersionReleaseCmd(deps)
	deps.Commands.Workflow.WorkflowListCmd = workflowListCmd

	taskCmd.AddCommand(deps.Commands.Workflow.TaskListCmd)
	taskRunCmd.AddCommand(deps.Commands.Workflow.TaskRunCmd)
	taskRunCmd.AddCommand(deps.Commands.Workflow.TaskRunListCmd)
	taskRunCmd.AddCommand(deps.Commands.Workflow.TaskRunDetailsCmd)
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

func SetupCommands() error {
	c, err := client.NewDefaultClient()
	if err != nil {
		if errors.Is(err, config.ErrLogin) {
			c, err = client.NotLoggedInClient()
			if err != nil {
				return fmt.Errorf("failed to create client: %w", err)
			}
		} else {
			return fmt.Errorf("failed to create client: %w", err)
		}
	}

	deps := dependencies.New(c)

	setupWorkflowCommands(deps)
	setupLogCommands(deps)
	setupWorkspaceCommands(deps)
	setupRootCmdPersistentRun(deps)

	return nil
}

func setupRootCmdPersistentRun(deps *dependencies.Dependencies) {
	rootCmd.PersistentPreRunE = rootPersistentPreRun(deps)
}

func rootPersistentPreRun(deps *dependencies.Dependencies) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		if err := checkForDeprecatedFlagUsage(cmd); err != nil {
			return err
		}

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
			println(err.Error())
			os.Exit(1)
		}

		explicitOutputSet := cmd.Flags().Changed("output")
		signals, err := deps.DetectRuntimeSignals()
		if err != nil {
			println(err.Error())
			os.Exit(1)
		}
		output, err := command.ResolveAutoOutput(explicitOutputSet, requestedOutput, signals)
		if err != nil {
			println(err.Error())
			os.Exit(1)
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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Check if version flag is explicitly requested before Cobra handles it
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			printVersionWithUpdateCheck()
			return
		}
	}

	if err := SetupCommands(); err != nil {
		os.Exit(1)
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.AddGroup(AllGroups...)

	rootCmd.Version = cfg.Version
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringP("output", "o", "interactive", "interactive, json, yaml, or text (auto: json in non-TTY contexts)")
	rootCmd.PersistentFlags().Bool(command.ConfirmFlag, false, "set to skip confirmation prompts")

	// Flags from the old CLI that we error with a helpful message
	rootCmd.PersistentFlags().Bool("pretty-json", false, "use --output json instead")
	if err := rootCmd.PersistentFlags().MarkHidden("pretty-json"); err != nil {
		panic(err)
	}
	rootCmd.PersistentFlags().Bool("json-record-per-line", false, "use --output json instead")
	if err := rootCmd.PersistentFlags().MarkHidden("json-record-per-line"); err != nil {
		panic(err)
	}
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
