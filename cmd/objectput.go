package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/storage"
	"github.com/render-oss/cli/pkg/text"
)

type ObjectPutInput struct {
	Key      string `cli:"arg:0"`
	FilePath string `cli:"file"`
	Region   string `cli:"region"`
	Local    bool   `cli:"local"`
}

func (i *ObjectPutInput) Validate(interactive bool) error {
	if i.Key == "" {
		return fmt.Errorf("key is required")
	}
	if i.FilePath == "" {
		return fmt.Errorf("--file is required")
	}
	if i.Region == "" {
		i.Region = cfg.GetRegion()
	}
	if i.Region == "" {
		return fmt.Errorf("--region is required (or set RENDER_REGION environment variable)")
	}
	return nil
}

var objectPutCmd = &cobra.Command{
	Use:   "put <key> --file=<path>",
	Short: "Upload a file to object storage",
	Long: `Upload a file to object storage.

The key is the path/name under which the object will be stored. Keys can include
path-like structures (e.g., "uploads/images/photo.jpg").

In local development mode (--local flag or RENDER_USE_LOCAL_DEV=true),
files are stored in the .render/objects/ directory.

Examples:
  render ea objects put my/object/key --file=/path/to/file.txt --region=oregon
  render ea objects put uploads/data.json --file=./data.json --region=oregon
  render ea objects put test/file --file=./test.txt --region=oregon --local
  render ea objects put my/key --file=./file.txt --region=oregon -o json
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input ObjectPutInput

		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		if expanded, err := command.ExpandPath(input.FilePath); err != nil {
			return fmt.Errorf("failed to resolve file path: %w", err)
		} else {
			input.FilePath = expanded
		}

		if nonInteractive, err := command.NonInteractive(cmd, func() (*storage.UploadResult, error) {
			return uploadObject(cmd, input)
		}, text.ObjectUpload); err != nil {
			return err
		} else if nonInteractive {
			return nil
		}

		// For now, object commands only support non-interactive mode
		// Interactive mode will be added when LIST endpoint is available
		result, err := uploadObject(cmd, input)
		if err != nil {
			return err
		}
		fmt.Print(text.ObjectUpload(result))
		return nil
	},
}

func uploadObject(cmd *cobra.Command, input ObjectPutInput) (*storage.UploadResult, error) {
	ctx := cmd.Context()

	cfg := storage.ServiceConfig{
		Local:  input.Local,
		Region: input.Region,
	}

	svc, err := storage.NewServiceFromContext(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage service: %w", err)
	}

	result, err := svc.Upload(ctx, input.Key, input.FilePath)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func init() {
	objectPutCmd.Flags().StringP("file", "f", "", "Path to the local file to upload (required)")
	objectPutCmd.MarkFlagRequired("file")
	objectPutCmd.MarkFlagFilename("file")

	objectCmd.AddCommand(objectPutCmd)
}
