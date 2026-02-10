package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/storage"
	"github.com/render-oss/cli/pkg/text"
)

type ObjectGetInput struct {
	Key      string `cli:"arg:0"`
	FilePath string `cli:"file"`
	Region   string `cli:"region"`
	Local    bool   `cli:"local"`
}

func (i *ObjectGetInput) Validate(interactive bool) error {
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

var objectGetCmd = &cobra.Command{
	Use:   "get <key> [--file=<path>]",
	Short: "Download a file from object storage",
	Long: `Download a file from object storage.

If --file is not specified, the object content is written to stdout,
which is useful for piping to other commands.

In local development mode (--local flag or RENDER_USE_LOCAL_DEV=true),
files are read from the .render/objects/ directory.

Examples:
  render ea objects get my/object/key --file=/path/to/output.txt --region=oregon
  render ea objects get my/object/key --region=oregon > output.txt
  render ea objects get my/object/key --region=oregon | jq .
  render ea objects get uploads/data.json --file=./data.json --region=oregon
  render ea objects get test/file --file=./output.txt --region=oregon --local
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var input ObjectGetInput

		if err := command.ParseCommand(cmd, args, &input); err != nil {
			return fmt.Errorf("failed to parse input: %w", err)
		}

		// If writing to file, use structured output
		if input.FilePath != "" {
			if nonInteractive, err := command.NonInteractive(cmd, func() (*storage.DownloadResult, error) {
				return downloadObjectToFile(cmd, input)
			}, text.ObjectDownload); err != nil {
				return err
			} else if nonInteractive {
				return nil
			}

			result, err := downloadObjectToFile(cmd, input)
			if err != nil {
				return err
			}
			fmt.Print(text.ObjectDownload(result))
			return nil
		}

		// If no file specified, write to stdout (raw content, no formatting)
		return downloadObjectToStdout(cmd, input)
	},
}

func downloadObjectToFile(cmd *cobra.Command, input ObjectGetInput) (*storage.DownloadResult, error) {
	ctx := cmd.Context()

	cfg := storage.ServiceConfig{
		Local:  input.Local,
		Region: input.Region,
	}

	svc, err := storage.NewServiceFromContext(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage service: %w", err)
	}

	// Create output file
	file, err := os.Create(input.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	result, err := svc.Download(ctx, input.Key, file)
	if err != nil {
		// Clean up partial file on error
		os.Remove(input.FilePath)
		return nil, err
	}

	result.LocalPath = input.FilePath
	return result, nil
}

func downloadObjectToStdout(cmd *cobra.Command, input ObjectGetInput) error {
	ctx := cmd.Context()

	cfg := storage.ServiceConfig{
		Local:  input.Local,
		Region: input.Region,
	}

	svc, err := storage.NewServiceFromContext(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed to create storage service: %w", err)
	}

	_, err = svc.Download(ctx, input.Key, os.Stdout)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	objectGetCmd.Flags().StringP("file", "f", "", "Output file path (default: stdout)")
	objectGetCmd.MarkFlagFilename("file")

	objectCmd.AddCommand(objectGetCmd)
}
