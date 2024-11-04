/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/renderinc/render-cli/pkg/command"
	"github.com/renderinc/render-cli/pkg/tui"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "render",
	Short: "**ALPHA** Interact with resources on Render",
	Long: `**WARNING: CLI IN ALPHA, ALL INTERACTIONS SUBJECT TO CHANGE**

The Render CLI allows you to interact with resources on Render from the command line.
View your projects, environments, and services, trigger deployments, view logs, and more.

By default the CLI will run in interactive mode, giving you a visual interface to interact with resources.
You can also use the CLI in non-interactive mode by specifying the output format with the --output with
either json or yaml.
`,
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

	rootCmd.PersistentFlags().StringP("output", "o", "interactive", "interactive, json, or yaml")
	rootCmd.PersistentFlags().Bool(command.ConfirmFlag, false, "set to skip confirmation prompts")
}
