package cmd

import (
	"github.com/spf13/cobra"
)

var objectCmd = &cobra.Command{
	Use:   "objects",
	Short: "Manage object storage",
	Long: `Manage object storage for your Render services.

Object storage allows you to store and retrieve arbitrary data. Use these commands
to list, upload, download, and delete objects.

In local development mode (when running with 'render ea tasks dev' or with the
--local flag), objects are stored in the .render/objects/ directory.

Available commands:
  list     - List objects in storage
  put      - Upload a file to object storage
  get      - Download a file from object storage
  delete   - Delete an object from storage

Examples:
  render ea objects list --region=oregon
  render ea objects put my/object/key --file=./data.txt --region=oregon
  render ea objects get my/object/key --file=./output.txt --region=oregon
  render ea objects delete my/object/key --region=oregon --yes
`,
}

func init() {
	// Add persistent flags shared by all object commands
	objectCmd.PersistentFlags().String("region", "", "Target region (required)")
	objectCmd.PersistentFlags().Bool("local", false, "Use local storage (.render/objects/) instead of cloud storage")

	// Mark region as required for all object commands
	objectCmd.MarkPersistentFlagRequired("region")

	EarlyAccessCmd.AddCommand(objectCmd)
}
