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

	"github.com/renderinc/render-cli/pkg/cfg"
	"github.com/renderinc/render-cli/pkg/command"
	renderstyle "github.com/renderinc/render-cli/pkg/style"
	"github.com/renderinc/render-cli/pkg/tui"
)

var welcomeMsg = lipgloss.NewStyle().Bold(true).Foreground(renderstyle.ColorFocus).
	Render("Welcome to the Render CLI!")

var betaMsg = lipgloss.NewStyle().Foreground(renderstyle.ColorInfo).
	Render("Note: The Render CLI is currently in beta, and may change as we release improvements and new features.")

var longHelp = fmt.Sprintf(`%s

%s

The Render CLI lets you manage your Render projects, environments, and services directly from the command line.
You can trigger deployments, view logs, and more—right from your terminal.

The CLI defaults to %s mode, offering an easy-to-use visual experience that makes it easier to find what you're looking for.
Prefer working without the interface? Use %s mode by specifying the --output option with either json or yaml for structured, scriptable responses.
`, welcomeMsg, betaMsg, renderstyle.Bold("interactive"), renderstyle.Bold("non-interactive"))

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use: "render",

	Short: "Interact with resources on Render",
	Long:  longHelp,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		confirmFlag, err := cmd.Flags().GetBool(command.ConfirmFlag)
		if err != nil {
			panic(err)
		}

		ctx = command.SetConfirmInContext(ctx, confirmFlag)

		outputFlag, err := cmd.Flags().GetString("output")
		if err != nil {
			panic(err)
		}

		output, err := command.StringToOutput(outputFlag)
		if err != nil {
			println(err.Error())
			os.Exit(1)
		}
		ctx = command.SetFormatInContext(ctx, &output)

		if output == command.Interactive {
			stack := tui.NewStack()

			ctx = tui.SetStackInContext(ctx, stack)
		}

		cmd.SetContext(ctx)

		return nil
	},

	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		output := command.GetFormatFromContext(ctx)
		if output == nil || *output == command.Interactive {
			if isPipe() {
				return errors.New("please specify `-o json` or `-o yaml` to pipe output")
			}

			stack := tui.GetStackFromContext(ctx)
			if stack == nil {
				return nil
			}

			p := tea.NewProgram(stack, tea.WithAltScreen())
			_, err := p.Run()
			if err != nil {
				panic(fmt.Sprintf("failed to initialize interface: %v", err))
			}
			return nil
		}

		return nil
	},
}

func isPipe() bool {
	stdout, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	isTerminal := (stdout.Mode() & os.ModeCharDevice) == os.ModeCharDevice
	return !isTerminal
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
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
	rootCmd.PersistentFlags().StringP("output", "o", "interactive", "interactive, json, or yaml")
	rootCmd.PersistentFlags().Bool(command.ConfirmFlag, false, "set to skip confirmation prompts")
}
