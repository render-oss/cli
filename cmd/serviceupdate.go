package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	servicepkg "github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/text"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

func newServiceUpdateCmd(deps *dependencies.Dependencies) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <service>",
		Args:  cobra.ExactArgs(1),
		Short: "Update configuration for an existing service",
		Long: `Update a service on Render. This command only runs in non-interactive modes.

Provide configuration updates with flags.`,
		Example: `  # Rename a service
  render services update my-service --name my-new-name --output json

  # Change a service plan
  render services update srv-abc123 --plan pro --output json`,
	}

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		var cliInput servicetypes.ServiceUpdateInput
		if err := command.ParseCommand(cmd, args, &cliInput); err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		// Interactive mode is not implemented yet; force non-interactive behavior for now.
		command.DefaultFormatNonInteractive(cmd)

		cliInput = servicetypes.NormalizeServiceUpdateCLIInput(cliInput)
		if err := cliInput.ValidateUpdate(); err != nil {
			return err
		}

		nonInteractive, err := command.NonInteractiveWithConfirm(cmd, func() (*servicepkg.UpdateOut, error) {
			return updateServiceNonInteractive(cmd.Context(), deps, cliInput)
		}, serviceUpdateTextOutput, nil)
		if err != nil {
			return err
		}
		if nonInteractive {
			return nil
		}

		// TODO: Implement interactive TUI mode in a later phase
		return fmt.Errorf("interactive mode not yet implemented")
	}

	// Identity and source flags
	cmd.Flags().String("name", "", "Service name")
	cmd.Flags().String("repo", "", "Git repository URL")
	cmd.Flags().String("branch", "", "Git branch")
	cmd.Flags().String("image", "", "Docker image URL")

	// Deployment configuration flags
	cmd.Flags().String("plan", "", "Service plan")
	runtimeFlag := command.NewEnumInput(servicetypes.ServiceRuntimeValues(), false)
	cmd.Flags().Var(runtimeFlag, "runtime", "Runtime environment")
	cmd.Flags().String("root-directory", "", "Root directory")

	// Build and start commands
	cmd.Flags().String("build-command", "", "Build command")
	cmd.Flags().String("start-command", "", "Start command")
	cmd.Flags().String("pre-deploy-command", "", "Pre-deploy command")

	// Type-specific flags
	cmd.Flags().String("health-check-path", "", "Health check path")
	cmd.Flags().String("publish-directory", "", "Publish directory")
	cmd.Flags().String("cron-command", "", "Cron command")
	cmd.Flags().String("cron-schedule", "", "Cron schedule")

	// Registry flag
	cmd.Flags().String("registry-credential", "", "Registry credential")

	// Behavior flags
	cmd.Flags().Bool("auto-deploy", false, "Enable auto-deploy")

	// Build filter flags
	cmd.Flags().StringArray("build-filter-path", nil, "Build filter path (can be specified multiple times)")
	cmd.Flags().StringArray("build-filter-ignored-path", nil, "Build filter ignored path (can be specified multiple times)")

	// Instance and scaling flags
	cmd.Flags().Int("num-instances", 0, "Number of instances")
	cmd.Flags().Int("max-shutdown-delay", 0, "Max shutdown delay in seconds")

	// Preview and preview generation flags
	previewsFlag := command.NewEnumInput(servicetypes.PreviewsGenerationValues(), false)
	cmd.Flags().Var(previewsFlag, "previews", "Preview generation mode")

	// Maintenance mode flags
	cmd.Flags().Bool("maintenance-mode", false, "Enable maintenance mode")
	cmd.Flags().String("maintenance-mode-uri", "", "Maintenance mode URI")

	// IP allow list flag
	cmd.Flags().StringArray("ip-allow-list", nil, "IP allow list entry in cidr=..., description=... format (can be specified multiple times)")

	return cmd
}

func updateServiceNonInteractive(ctx context.Context, deps *dependencies.Dependencies, cliInput servicetypes.ServiceUpdateInput) (*servicepkg.UpdateOut, error) {
	serviceRepo := deps.ServiceRepo()
	serviceService := deps.ServiceService()

	// Resolve service ID
	idOrName, err := cliInput.ParseServiceID()
	if err != nil {
		return nil, err
	}

	serviceID, err := serviceRepo.ResolveServiceIDFromNameOrID(ctx, idOrName)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve service %q: %w", idOrName, err)
	}

	// TODO: BuildUpdateRequest to construct the PATCH body
	// For now, use an empty UpdateServiceJSONRequestBody as a placeholder
	body := client.UpdateServiceJSONRequestBody{}

	if _, err := serviceRepo.UpdateService(ctx, serviceID, body); err != nil {
		return nil, err
	}

	updated, err := serviceService.GetService(ctx, serviceID)
	if err != nil {
		return nil, err
	}

	out := servicepkg.NewUpdateOutFromModel(updated)
	return &out, nil
}

func serviceUpdateTextOutput(out *servicepkg.UpdateOut) string {
	return "Updated this service:\n\n" + text.ServiceDetail(&out.Data) + "\n"
}
