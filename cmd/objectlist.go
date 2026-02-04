package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/storage"
	"github.com/render-oss/cli/pkg/text"
)

type ObjectListInput struct {
	Region string `cli:"region"`
	Local  bool   `cli:"local"`
	Limit  int    `cli:"limit"`
}

func (i *ObjectListInput) Validate(interactive bool) error {
	if i.Region == "" {
		i.Region = cfg.GetRegion()
	}
	if i.Region == "" {
		return fmt.Errorf("--region is required (or set RENDER_REGION environment variable)")
	}
	if i.Limit <= 0 {
		return fmt.Errorf("--limit must be greater than 0")
	}
	return nil
}

var objectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List objects in storage",
	Long: `List objects in object storage for a specific region.

Displays object keys, content types, sizes, and last modified timestamps.

In local development mode (--local flag or RENDER_USE_LOCAL_DEV=true),
lists files from the .render/objects/ directory.

Examples:
  render ea objects list --region=oregon
  render ea objects list --region=oregon --limit=50
  render ea objects list --region=oregon --local
  render ea objects list --region=oregon -o json
  render ea objects list --region=oregon -o text
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var input ObjectListInput

		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]storage.ObjectInfo, error) {
			return listObjects(cmd, input)
		}, text.ObjectTable); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		// For now, object commands only support non-interactive mode
		// Interactive mode (TUI) can be added in the future
		result, err := listObjects(cmd, input)
		if err != nil {
			return err
		}
		fmt.Print(text.ObjectTable(result))
		return nil
	},
}

func listObjects(cmd *cobra.Command, input ObjectListInput) ([]storage.ObjectInfo, error) {
	ctx := cmd.Context()

	cfg := storage.ServiceConfig{
		Local:  input.Local,
		Region: input.Region,
	}

	svc, err := storage.NewServiceFromContext(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage service: %w", err)
	}

	result, err := svc.List(ctx, "", input.Limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	return result.Objects, nil
}

func init() {
	objectListCmd.Flags().Int("limit", 100, "Maximum number of objects to return")

	objectCmd.AddCommand(objectListCmd)
}
