package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/text"
	"github.com/render-oss/cli/pkg/types"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

var ServiceCreateCmd = &cobra.Command{
	Use:   "create",
	Args:  cobra.NoArgs,
	Short: "Create a new service or clone an existing one",
	Long: `Create a new service on Render. This command only runs in non-interactive modes. Provide configuration options with flags.`,
	Example: `  # Create a service from repository configuration
  render services create --name my-api --type web_service --repo https://github.com/org/repo --runtime node --build-command "npm install" --start-command "npm start" --output json

  # Clone configuration from an existing service
  render services create --from srv-abc123 --name my-api-clone --output json`,
}

func init() {
	servicesCmd.AddCommand(ServiceCreateCmd)

	ServiceCreateCmd.RunE = func(cmd *cobra.Command, args []string) error {
		var cliInput servicetypes.ServiceCreateInput
		if err := command.ParseCommand(cmd, args, &cliInput); err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}

		// Interactive mode is not implemented yet; force non-interactive behavior for now.
		command.DefaultFormatNonInteractive(cmd)

		ctx := cmd.Context()

		nonInteractive, err := command.NonInteractive(cmd, func() (*client.Service, error) {
			return createServiceNonInteractive(ctx, cliInput)
		}, func(svc *client.Service) string {
			return text.FormatStringF("Created service %s (%s)", svc.Name, svc.Id)
		})
		if err != nil {
			return err
		}
		if nonInteractive {
			return nil
		}

		// TODO: Implement interactive TUI mode in a later phase
		// This will use views.NewServiceCreateView for guided configuration
		return fmt.Errorf("interactive mode not yet implemented")
	}

	ServiceCreateCmd.Flags().String("name", "", "Set the service name")
	serviceTypeFlag := command.NewEnumInput(servicetypes.ServiceTypeValues(), false)
	ServiceCreateCmd.Flags().Var(serviceTypeFlag, "type", "Set the service type")
	ServiceCreateCmd.Flags().String("from", "", "Clone configuration from an existing service ID or name and override cloned values with other flags")
	ServiceCreateCmd.Flags().String("repo", "", "Set the Git repository URL")
	ServiceCreateCmd.Flags().String("branch", "", "Set the Git branch")
	ServiceCreateCmd.Flags().String("image", "", "Set the Docker image URL")
	regionFlag := command.NewEnumInput(types.RegionValues(), false)
	ServiceCreateCmd.Flags().Var(regionFlag, "region", "Set the deployment region")
	ServiceCreateCmd.Flags().String("plan", "", "Set the service plan")
	runtimeFlag := command.NewEnumInput(servicetypes.ServiceRuntimeValues(), false)
	ServiceCreateCmd.Flags().Var(runtimeFlag, "runtime", "Set the runtime environment")
	ServiceCreateCmd.Flags().String("root-directory", "", "Set the root directory")
	ServiceCreateCmd.Flags().String("build-command", "", "Set the build command")
	ServiceCreateCmd.Flags().String("start-command", "", "Set the start command")
	ServiceCreateCmd.Flags().String("health-check-path", "", "Set the health check path")
	ServiceCreateCmd.Flags().String("publish-directory", "", "Set the publish directory")
	ServiceCreateCmd.Flags().String("cron-command", "", "Set the cron command")
	ServiceCreateCmd.Flags().String("cron-schedule", "", "Set the cron schedule")
	ServiceCreateCmd.Flags().String("environment-id", "", "Set the environment ID")
	ServiceCreateCmd.Flags().StringArray("env-var", nil, "Set environment variables in KEY=VALUE format (can be specified multiple times)")
	ServiceCreateCmd.Flags().StringArray("secret-file", nil, "Set secret files in NAME:LOCAL_PATH format (can be specified multiple times)")
	ServiceCreateCmd.Flags().String("registry-credential", "", "Set the registry credential")
	ServiceCreateCmd.Flags().Bool("auto-deploy", true, "Enable auto-deploy")
	ServiceCreateCmd.Flags().String("pre-deploy-command", "", "Set the pre-deploy command")
	ServiceCreateCmd.Flags().StringArray("build-filter-path", nil, "Set build filter paths (can be specified multiple times)")
	ServiceCreateCmd.Flags().StringArray("build-filter-ignored-path", nil, "Set build filter ignored paths (can be specified multiple times)")
	ServiceCreateCmd.Flags().Int("num-instances", 0, "Set the number of instances")
	ServiceCreateCmd.Flags().Int("max-shutdown-delay", 0, "Set max shutdown delay in seconds")
	previewsFlag := command.NewEnumInput(servicetypes.PreviewsGenerationValues(), false)
	ServiceCreateCmd.Flags().Var(previewsFlag, "previews", "Set preview generation mode")
	ServiceCreateCmd.Flags().Bool("maintenance-mode", false, "Enable maintenance mode")
	ServiceCreateCmd.Flags().String("maintenance-mode-uri", "", "Set the maintenance mode URI")
	ServiceCreateCmd.Flags().StringArray("ip-allow-list", nil, "Set IP allow list entries in cidr=..., description=... format (can be specified multiple times)")

	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "name", command.FlagPlaceholderAnnotation, []string{"NAME"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "type", command.FlagPlaceholderAnnotation, []string{"TYPE"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "from", command.FlagPlaceholderAnnotation, []string{"SERVICE_ID"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "repo", command.FlagPlaceholderAnnotation, []string{"REPO_URL"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "branch", command.FlagPlaceholderAnnotation, []string{"BRANCH"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "image", command.FlagPlaceholderAnnotation, []string{"IMAGE_URL"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "region", command.FlagPlaceholderAnnotation, []string{"REGION"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "plan", command.FlagPlaceholderAnnotation, []string{"PLAN"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "runtime", command.FlagPlaceholderAnnotation, []string{"RUNTIME"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "root-directory", command.FlagPlaceholderAnnotation, []string{"PATH"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "build-command", command.FlagPlaceholderAnnotation, []string{"COMMAND"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "start-command", command.FlagPlaceholderAnnotation, []string{"COMMAND"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "health-check-path", command.FlagPlaceholderAnnotation, []string{"PATH"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "publish-directory", command.FlagPlaceholderAnnotation, []string{"PATH"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "cron-command", command.FlagPlaceholderAnnotation, []string{"COMMAND"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "cron-schedule", command.FlagPlaceholderAnnotation, []string{"SCHEDULE"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "environment-id", command.FlagPlaceholderAnnotation, []string{"ENVIRONMENT_ID"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "env-var", command.FlagPlaceholderAnnotation, []string{"KEY_VALUE"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "secret-file", command.FlagPlaceholderAnnotation, []string{"NAME_PATH"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "registry-credential", command.FlagPlaceholderAnnotation, []string{"CREDENTIAL"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "pre-deploy-command", command.FlagPlaceholderAnnotation, []string{"COMMAND"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "build-filter-path", command.FlagPlaceholderAnnotation, []string{"PATHS"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "build-filter-ignored-path", command.FlagPlaceholderAnnotation, []string{"PATHS"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "num-instances", command.FlagPlaceholderAnnotation, []string{"COUNT"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "max-shutdown-delay", command.FlagPlaceholderAnnotation, []string{"SECONDS"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "previews", command.FlagPlaceholderAnnotation, []string{"PREVIEWS"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "maintenance-mode-uri", command.FlagPlaceholderAnnotation, []string{"URI"})
	setAnnotationBestEffort(ServiceCreateCmd.Flags(), "ip-allow-list", command.FlagPlaceholderAnnotation, []string{"CIDR_DESCRIPTION"})
}

func createServiceNonInteractive(ctx context.Context, cliInput servicetypes.ServiceCreateInput) (*client.Service, error) {
	deps := dependencies.GetFromContext(ctx)
	serviceRepo := deps.ServiceRepo()
	registryService := deps.RegistryService()

	ownerID, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	cliInput = servicetypes.NormalizeServiceCreateCLIInput(cliInput)

	if cliInput.From != nil {
		if err := getConfigFromService(ctx, serviceRepo, &cliInput); err != nil {
			return nil, fmt.Errorf("failed to clone configuration from source service %q: %w", *cliInput.From, err)
		}
	}

	cliInput, err = servicetypes.NormalizeAndValidateCreateInput(cliInput, false)
	if err != nil {
		return nil, err
	}

	if cliInput.RegistryCredential != nil && cliInput.SupportsRegistryCredentials() {
		registryCredentialID, err := registryService.FindOneRegistryCredentialByIDFromNameOrID(ctx, ownerID, *cliInput.RegistryCredential)
		if err != nil {
			return nil, err
		}
		cliInput.RegistryCredential = &registryCredentialID
	}

	body, err := service.BuildCreateRequest(cliInput, ownerID)
	if err != nil {
		return nil, err
	}
	return serviceRepo.CreateService(ctx, body)
}

func getConfigFromService(ctx context.Context, repo *service.Repo, input *servicetypes.ServiceCreateInput) error {
	serviceID, err := repo.ResolveServiceIDFromNameOrID(ctx, *input.From)
	if err != nil {
		return fmt.Errorf("failed to resolve source service %s: %w", *input.From, err)
	}
	input.From = &serviceID

	sourceService, err := repo.GetService(ctx, serviceID)
	if err != nil {
		return fmt.Errorf("failed to load source service %s: %w", serviceID, err)
	}
	service.ServiceFromAPI(input, sourceService)
	return nil
}
