package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
)

var blueprintValidateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate a render.yaml file",
	Long: `Validate a render.yaml blueprint file for errors before committing.

Validates:
  - YAML syntax
  - Schema validation (required fields, types)
  - Semantic validation (valid plans, regions, etc.)
  - Conflict checking against existing resources

Examples:
  render blueprints validate                    # Validate ./render.yaml
  render blueprints validate ./my-blueprint.yaml  # Validate specific file
  render blueprints validate -o json            # Output as JSON`,
	Args:         cobra.MaximumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		return runBlueprintValidate(ctx, cmd, args)
	},
}

func init() {
	blueprintsCmd.AddCommand(blueprintValidateCmd)
	blueprintValidateCmd.Flags().StringP("workspace", "w", "", "Workspace ID to validate against (defaults to current workspace)")
}

func runBlueprintValidate(ctx context.Context, cmd *cobra.Command, args []string) error {
	output := command.GetFormatFromContext(ctx)

	filePath := "render.yaml"
	if len(args) > 0 {
		filePath = args[0]
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("file not found: %s", absPath)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	workspaceID, err := cmd.Flags().GetString("workspace")
	if err != nil {
		return fmt.Errorf("failed to get workspace flag: %w", err)
	}
	if workspaceID == "" {
		workspaceID, err = config.WorkspaceID()
		if err != nil {
			return fmt.Errorf("no workspace specified and no default workspace set. Use --workspace or run 'render workspace set'")
		}
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("ownerId", workspaceID); err != nil {
		return fmt.Errorf("failed to write ownerId field: %w", err)
	}

	part, err := writer.CreateFormFile("file", filepath.Base(absPath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(content); err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	c, err := client.NewDefaultClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := c.ValidateBlueprintWithBodyWithResponse(
		ctx,
		writer.FormDataContentType(),
		&body,
	)
	if err != nil {
		return fmt.Errorf("failed to validate blueprint: %w", err)
	}

	if resp.StatusCode() != 200 {
		return fmt.Errorf("validation request failed with status %d: %s", resp.StatusCode(), string(resp.Body))
	}

	result := resp.JSON200

	if output.Interactive() {
		return printValidationResultInteractive(absPath, result)
	}

	_, err = command.PrintData(cmd, result, func(r *bptypes.ValidateBlueprintResponse) string {
		jsonBytes, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return fmt.Sprintf("{\"error\": \"failed to format result: %v\"}\n", err)
		}
		return string(jsonBytes) + "\n"
	})
	return err
}

func formatValidationError(e bptypes.ValidationError) string {
	var location string
	if e.Line != nil && *e.Line > 0 {
		location = fmt.Sprintf("line %d", *e.Line)
		if e.Column != nil && *e.Column > 0 {
			location += fmt.Sprintf(", column %d", *e.Column)
		}
	}

	path := ""
	if e.Path != nil {
		path = *e.Path
	}

	switch {
	case location != "" && path != "":
		return fmt.Sprintf("  %s (%s): %s", path, location, e.Error)
	case path != "":
		return fmt.Sprintf("  %s: %s", path, e.Error)
	case location != "":
		return fmt.Sprintf("  %s: %s", location, e.Error)
	default:
		return fmt.Sprintf("  %s", e.Error)
	}
}

func printValidationResultInteractive(filePath string, result *bptypes.ValidateBlueprintResponse) error {
	if result.Valid {
		fmt.Printf("%s is valid\n", filePath)

		if result.Plan != nil {
			fmt.Println("\nPlan summary:")
			if result.Plan.Services != nil && len(*result.Plan.Services) > 0 {
				fmt.Printf("  %-14s %d\n", "Services:", len(*result.Plan.Services))
			}
			if result.Plan.Databases != nil && len(*result.Plan.Databases) > 0 {
				fmt.Printf("  %-14s %d\n", "Databases:", len(*result.Plan.Databases))
			}
			if result.Plan.KeyValue != nil && len(*result.Plan.KeyValue) > 0 {
				fmt.Printf("  %-14s %d\n", "Key Value:", len(*result.Plan.KeyValue))
			}
			if result.Plan.EnvGroups != nil && len(*result.Plan.EnvGroups) > 0 {
				fmt.Printf("  %-14s %d\n", "Env Groups:", len(*result.Plan.EnvGroups))
			}
			if result.Plan.TotalActions != nil {
				fmt.Printf("  %-14s %d\n", "Total Actions:", *result.Plan.TotalActions)
			}
		}
		return nil
	}

	if result.Errors != nil {
		for _, e := range *result.Errors {
			fmt.Println(formatValidationError(e))
		}
	}

	return fmt.Errorf("%s has validation errors", filePath)
}
