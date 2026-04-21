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

The key is the path/name under which the object will be stored. Keys can include path-like structures (e.g., "uploads/images/photo.jpg").

In local development mode (--local flag or RENDER_USE_LOCAL_DEV=true), files are stored in the .render/objects/ directory.`,
	Example: `  # Upload a local file
  render ea objects put backups/2026-04-15/users.ndjson --file=./exports/users.ndjson --region=oregon

  # Upload with a relative file path
  render ea objects put assets/images/logo.png --file=./public/logo.png --region=oregon

  # Upload to local object storage
  render ea objects put local-dev/fixtures/sample.json --file=./fixtures/sample.json --region=oregon --local`,
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
	objectPutCmd.Flags().StringP("file", "f", "", "Path to the local file to upload (Required)")
	objectPutCmd.MarkFlagRequired("file")
	objectPutCmd.MarkFlagFilename("file")
	setAnnotationBestEffort(objectPutCmd.Flags(), "file", command.FlagPlaceholderAnnotation, []string{"PATH"})

	objectCmd.AddCommand(objectPutCmd)
}
