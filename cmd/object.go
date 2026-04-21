package cmd

import (
	"github.com/render-oss/cli/pkg/command"
	"github.com/spf13/cobra"
)

var objectCmd = &cobra.Command{
	Use:   "objects",
	Short: "Manage object storage in early access",
	Long: `Manage object storage for your Render services.

Object storage allows you to store and retrieve arbitrary data. Use these commands to list, upload, download, and delete objects.

The --region flag specifies which region to use. Alternatively, set the RENDER_REGION environment variable. The --region flag takes precedence if both are provided.

When using the --local flag, objects are stored in the .render/objects/ directory instead of cloud storage.`,
	Example: `  # List objects in object storage
  render ea objects list --region=oregon

  # Upload an object
  render ea objects put backups/2026-04-15/users.ndjson --file=./exports/users.ndjson --region=oregon

  # Download an object
  render ea objects get backups/2026-04-15/users.ndjson --file=./downloads/users.ndjson --region=oregon

  # Delete an object
  render ea objects delete uploads/test/avatar.png --region=oregon --yes`,
}

func init() {
	// Add persistent flags shared by all object commands
	objectCmd.PersistentFlags().String("region", "", "Set target region (or set the RENDER_REGION env var)")
	objectCmd.PersistentFlags().Bool("local", false, "Use local storage (.render/objects/) instead of cloud storage")
	setAnnotationBestEffort(objectCmd.PersistentFlags(), "region", command.FlagPlaceholderAnnotation, []string{"REGION"})

	EarlyAccessCmd.AddCommand(objectCmd)
}
