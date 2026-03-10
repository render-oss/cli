package cmd

import "github.com/spf13/cobra"

// When uncommenting init() below, add this import:
//   "github.com/render-oss/cli/pkg/command"

// CommandTemplateCmd is the canonical command template for new CLI commands.
// Copy this file when starting a new command and then rename symbols/fields.
var CommandTemplateCmd = &cobra.Command{
	// Command name and positional arguments.
	// Examples:
	//   "mycommand"                          // No arguments
	//   "mycommand [serviceID]"              // Optional argument
	//   "mycommand <serviceID>"              // Required argument
	Use: "mycommand [serviceID]",

	// `Args` is required if the `Use` field has positional arguments.
	// Keep `Use` and `Args` consistent:
	//   Use: "mycommand"                    -> Args: (omit)
	//   Use: "mycommand [serviceID]"        -> Args: cobra.MaximumNArgs(1)
	//   Use: "mycommand <serviceID>"        -> Args: cobra.ExactArgs(1)
	//   Use: "mycommand <workspaceID> <id>" -> Args: cobra.ExactArgs(2)
	Args: cobra.MaximumNArgs(1),

	// One-line summary.
	// Start with an action verb and avoid a trailing period.
	Short: "Manage build pipelines for the active workspace",

	// Detailed description for the DETAILS section in help output.
	// Use complete sentences and include practical usage context.
	// The help template auto-wraps at 80 characters.
	Long: "Manages build pipelines for the active workspace. In interactive mode, you can view pipeline status, trigger builds, and modify pipeline settings. Use flags to filter by environment or change output format for scripting.",

	// Group in root help output.
	// Common options: GroupCore, GroupAuth, GroupSession, GroupManagement.
	// Top-level exceptions without GroupID include: docs, ea, skills.
	GroupID: GroupCore.ID,

	// Include 1-5 practical examples.
	// Indent each line with 2 spaces and annotate examples with # comments.
	Example: `  # List all pipelines
  render mycommand

  # Output as JSON for scripting
  render mycommand --output json

  # Filter by environment
  render mycommand -e env-abc123`,
}

// SUBCOMMAND TEMPLATE
// Prefer existing verbs when possible: list, create, get, set, update,
// delete, cancel, validate.
var CommandTemplateListCmd = &cobra.Command{
	// Optional and required notation:
	// [optional], <required>, [A|B], <A|B>
	Use: "list [id]",
	// Keep concise and avoid trailing period.
	Short: "List resources for the active workspace",
	// Match `Args` to `Use`.
	Args: cobra.MaximumNArgs(1),
}

// Uncomment this in your copied command file after renaming symbols.
//
// func init() {
// 	rootCmd.AddCommand(myCommandCmd)
// 	myCommandCmd.AddCommand(myCommandListCmd)
//
// 	// Define flags inline. Use kebab-case names and action-verb descriptions without trailing periods.
// 	myCommandCmd.Flags().StringSliceP("environment-ids", "e", nil, "Filter by the specified environment IDs")
// 	setAnnotationBestEffort(myCommandCmd.Flags(), "environment-ids", command.FlagPlaceholderAnnotation, []string{"ENV_IDS"})
//
// 	myCommandCmd.Flags().Bool("include-previews", false, "Include preview environments in the list")
//
// 	myCommandCmd.RunE = func(cmd *cobra.Command, args []string) error {
// 		// TODO: parse input and execute command logic.
// 		return nil
// 	}
// }
