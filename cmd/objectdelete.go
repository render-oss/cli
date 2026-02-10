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
	Key    string `cli:"arg:0"`
	Yes    bool   `cli:"yes"`
	Region string `cli:"region"`
	Local  bool   `cli:"local"`
}

func (i *ObjectDeleteInput) Validate(interactive bool) error {
	if i.Key == "" {
		return fmt.Errorf("key is required")
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
	Use:   "delete <key>",
	Short: "Delete an object from storage",
	Long: `Delete an object from storage.

This operation is irreversible. By default, you will be prompted for confirmation
unless you specify the --yes flag.

In local development mode (--local flag or RENDER_USE_LOCAL_DEV=true),
files are deleted from the .render/objects/ directory.

Examples:
  render ea objects delete my/object/key --region=oregon
  render ea objects delete my/object/key --region=oregon --yes
  render ea objects delete uploads/data.json --region=oregon --yes
  render ea objects delete test/file --region=oregon --local --yes
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input ObjectDeleteInput

		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		// Prompt for confirmation unless --yes is specified or non-interactive
		if !input.Yes && command.IsInteractive(cmd.Context()) {
			if !confirmDelete(input.Key) {
				fmt.Println("Delete cancelled.")
				return nil
			}
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() (*storage.DeleteResult, error) {
			return deleteObject(cmd, input)
		}, text.ObjectDelete); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		result, err := deleteObject(cmd, input)
		if err != nil {
			return err
		}
		fmt.Print(text.ObjectDelete(result))
		return nil
	},
}

func deleteObject(cmd *cobra.Command, input ObjectDeleteInput) (*storage.DeleteResult, error) {
	ctx := cmd.Context()

	cfg := storage.ServiceConfig{
		Local:  input.Local,
		Region: input.Region,
	}

	svc, err := storage.NewServiceFromContext(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage service: %w", err)
	}

	result, err := svc.Delete(ctx, input.Key)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func confirmDelete(key string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Are you sure you want to delete object '%s'? This cannot be undone. [y/N]: ", key)
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
