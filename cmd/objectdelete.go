package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/storage"
	"github.com/render-oss/cli/pkg/text"
)

type ObjectDeleteInput struct {
	Keys   []string
	Yes    bool   `cli:"yes"`
	Region string `cli:"region"`
	Local  bool   `cli:"local"`
}

func (i *ObjectDeleteInput) Validate(interactive bool) error {
	if len(i.Keys) == 0 {
		return fmt.Errorf("at least one key is required")
	}
	if i.Region == "" {
		i.Region = cfg.GetRegion()
	}
	if i.Region == "" {
		return fmt.Errorf("--region is required (or set RENDER_REGION environment variable)")
	}
	return nil
}

var objectDeleteCmd = &cobra.Command{
	Use:   "delete <key> [key...]",
	Short: "Delete one or more objects from storage",
	Long: `Delete one or more objects from storage.

This operation is irreversible. By default, you will be prompted for confirmation
unless you specify the --yes flag.

In local development mode (--local flag or RENDER_USE_LOCAL_DEV=true),
files are deleted from the .render/objects/ directory.

Examples:
  render ea objects delete my/object/key --region=oregon
  render ea objects delete my/object/key --region=oregon --yes
  render ea objects delete seed/file-00000.txt seed/file-00001.txt seed/file-00002.txt --region=oregon --yes
  render ea objects delete test/file --region=oregon --local --yes
`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input ObjectDeleteInput
		input.Keys = args

		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		// Prompt for confirmation unless --yes is specified or non-interactive
		if !input.Yes && command.IsInteractive(cmd.Context()) {
			if !confirmDelete(input.Keys) {
				fmt.Println("Delete cancelled.")
				return nil
			}
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() ([]*storage.DeleteResult, error) {
			return deleteObjects(cmd, input)
		}, text.ObjectDeleteMultiple); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		results, err := deleteObjects(cmd, input)
		if err != nil {
			return err
		}
		fmt.Print(text.ObjectDeleteMultiple(results))
		return nil
	},
}

func deleteObjects(cmd *cobra.Command, input ObjectDeleteInput) ([]*storage.DeleteResult, error) {
	ctx := cmd.Context()

	cfg := storage.ServiceConfig{
		Local:  input.Local,
		Region: input.Region,
	}

	svc, err := storage.NewServiceFromContext(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage service: %w", err)
	}

	var results []*storage.DeleteResult
	for _, key := range input.Keys {
		result, err := svc.Delete(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("failed to delete %s: %w", key, err)
		}
		results = append(results, result)
	}

	return results, nil
}

func confirmDelete(keys []string) bool {
	reader := bufio.NewReader(os.Stdin)
	if len(keys) == 1 {
		fmt.Printf("Are you sure you want to delete object '%s'? This cannot be undone. [y/N]: ", keys[0])
	} else {
		fmt.Printf("Are you sure you want to delete %d objects? This cannot be undone. [y/N]: ", len(keys))
	}
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

func init() {
	objectDeleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	objectCmd.AddCommand(objectDeleteCmd)
}
