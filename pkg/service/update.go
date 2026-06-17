package service

import (
	"errors"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

const registryCredentialUpdateIncompatibleRuntimeError = "--registry-credential must be paired with --image unless the service uses the docker runtime"

// BuildUpdateRequest converts a normalized ServiceUpdateInput into the API
// PATCH body for the resolved service type. It only sets fields represented by
// update flags; omitted fields are left nil so the API leaves them unchanged.
func BuildUpdateRequest(
	before client.Service,
	input servicetypes.ServiceUpdateInput,
) (client.UpdateServiceJSONRequestBody, error) {
	if input.Runtime != nil {
		return client.UpdateServiceJSONRequestBody{}, servicetypes.ErrRuntimeUpdateNotSupported
	}

	body := client.UpdateServiceJSONRequestBody{}

	if input.Name != "" {
		body.Name = &input.Name
	}
	if input.Repo != nil {
		body.Repo = input.Repo
	}
	if input.Branch != nil {
		body.Branch = input.Branch
	}
	if input.RootDirectory != nil {
		body.RootDir = input.RootDirectory
	}
	if input.Image != nil {
		body.Image = &client.Image{ImagePath: *input.Image}
		if input.RegistryCredential != nil {
			body.Image.RegistryCredentialId = input.RegistryCredential
			// Registry credentials are context-sensitive: with --image they
			// authenticate the top-level image source; otherwise, Docker
			// services use them in envSpecificDetails. Clear this local copy
			// after consuming it so the PATCH body sets only one destination.
			input.RegistryCredential = nil
		}
	}
	if input.AutoDeploy != nil {
		autoDeploy := client.AutoDeployYes
		if !*input.AutoDeploy {
			autoDeploy = client.AutoDeployNo
		}
		body.AutoDeploy = pointers.From(autoDeploy)
	}
	if buildFilter := buildFilterFromInputs(input.BuildFilterPaths, input.BuildFilterIgnoredPaths); buildFilter != nil {
		body.BuildFilter = buildFilter
	}

	serviceDetails, err := buildUpdateServiceDetails(before, input)
	if err != nil {
		return client.UpdateServiceJSONRequestBody{}, err
	}
	body.ServiceDetails = serviceDetails

	return body, nil
}

func buildUpdateServiceDetails(
	before client.Service,
	input servicetypes.ServiceUpdateInput,
) (*client.ServicePATCH_ServiceDetails, error) {
	runtime, err := extractServiceRuntime(before)
	if err != nil {
		return nil, err
	}

	switch before.Type {
	case client.WebService:
		return buildWebServiceUpdateDetails(before, runtime, input)
	case client.PrivateService:
		return buildPrivateServiceUpdateDetails(runtime, input)
	case client.BackgroundWorker:
		return buildBackgroundWorkerUpdateDetails(runtime, input)
	case client.CronJob:
		return buildCronJobUpdateDetails(runtime, input)
	case client.StaticSite:
		return buildStaticSiteUpdateDetails(input)
	default:
		return nil, nil
	}
}

// extractServiceRuntime reads the current runtime from the service details
// union. Runtime is nested under each runtime-backed service type rather than
// exposed as a top-level field on client.Service.
func extractServiceRuntime(service client.Service) (*client.ServiceRuntime, error) {
	switch service.Type {
	case client.WebService:
		details, err := service.ServiceDetails.AsWebServiceDetails()
		if err != nil {
			return nil, err
		}
		return &details.Runtime, nil
	case client.PrivateService:
		details, err := service.ServiceDetails.AsPrivateServiceDetails()
		if err != nil {
			return nil, err
		}
		return &details.Runtime, nil
	case client.BackgroundWorker:
		details, err := service.ServiceDetails.AsBackgroundWorkerDetails()
		if err != nil {
			return nil, err
		}
		return &details.Runtime, nil
	case client.CronJob:
		details, err := service.ServiceDetails.AsCronJobDetails()
		if err != nil {
			return nil, err
		}
		return &details.Runtime, nil
	case client.StaticSite:
		return nil, nil
	default:
		return nil, nil
	}
}

func buildWebServiceUpdateDetails(
	before client.Service,
	runtime *client.ServiceRuntime,
	input servicetypes.ServiceUpdateInput,
) (*client.ServicePATCH_ServiceDetails, error) {
	var details client.WebServiceDetailsPATCH

	envSpecificDetails, err := buildUpdateEnvSpecificDetails(
		runtime,
		input,
	)
	if err != nil {
		return nil, err
	}
	details.EnvSpecificDetails = envSpecificDetails
	if input.Plan != nil {
		details.Plan = toClientPlan(input.Plan)
	}
	if input.HealthCheckPath != nil {
		details.HealthCheckPath = input.HealthCheckPath
	}
	if input.PreDeployCommand != nil {
		details.PreDeployCommand = input.PreDeployCommand
	}
	if input.MaxShutdownDelay != nil {
		details.MaxShutdownDelaySeconds = maxShutdownDelayFromInput(input.MaxShutdownDelay)
	}
	if input.Previews != nil {
		details.Previews = previewsFromInput(input.Previews)
	}
	if input.MaintenanceMode != nil || input.MaintenanceModeURI != nil {
		beforeDetails, err := before.ServiceDetails.AsWebServiceDetails()
		if err != nil {
			return nil, err
		}
		details.MaintenanceMode = maintenanceModeFromUpdateInput(
			beforeDetails.MaintenanceMode,
			input.MaintenanceMode,
			input.MaintenanceModeURI,
		)
	}
	if len(input.IPAllowList) > 0 {
		ipAllowList, err := ParseIPAllowListInputs(input.IPAllowList)
		if err != nil {
			return nil, err
		}
		details.IpAllowList = ipAllowList
	}

	if details == (client.WebServiceDetailsPATCH{}) {
		return nil, nil
	}

	serviceDetails := &client.ServicePATCH_ServiceDetails{}
	if err := serviceDetails.FromWebServiceDetailsPATCH(details); err != nil {
		return nil, err
	}
	return serviceDetails, nil
}

func maintenanceModeFromUpdateInput(existing *client.MaintenanceMode, enabled *bool, uri *string) *client.MaintenanceMode {
	if enabled == nil && uri == nil {
		return nil
	}

	maintenanceMode := client.MaintenanceMode{}
	if existing != nil {
		maintenanceMode = *existing
	}
	if enabled != nil {
		maintenanceMode.Enabled = *enabled
	}
	if uri != nil {
		maintenanceMode.Uri = *uri
	}
	return &maintenanceMode
}

func buildPrivateServiceUpdateDetails(
	runtime *client.ServiceRuntime,
	input servicetypes.ServiceUpdateInput,
) (*client.ServicePATCH_ServiceDetails, error) {
	var details client.PrivateServiceDetailsPATCH

	envSpecificDetails, err := buildUpdateEnvSpecificDetails(
		runtime,
		input,
	)
	if err != nil {
		return nil, err
	}
	details.EnvSpecificDetails = envSpecificDetails
	if input.Plan != nil {
		details.Plan = paidPlan(toClientPlan(input.Plan))
	}
	if input.PreDeployCommand != nil {
		details.PreDeployCommand = input.PreDeployCommand
	}
	if input.MaxShutdownDelay != nil {
		details.MaxShutdownDelaySeconds = maxShutdownDelayFromInput(input.MaxShutdownDelay)
	}
	if input.Previews != nil {
		details.Previews = previewsFromInput(input.Previews)
	}

	if details == (client.PrivateServiceDetailsPATCH{}) {
		return nil, nil
	}

	serviceDetails := &client.ServicePATCH_ServiceDetails{}
	if err := serviceDetails.FromPrivateServiceDetailsPATCH(details); err != nil {
		return nil, err
	}
	return serviceDetails, nil
}

func buildBackgroundWorkerUpdateDetails(
	runtime *client.ServiceRuntime,
	input servicetypes.ServiceUpdateInput,
) (*client.ServicePATCH_ServiceDetails, error) {
	var details client.BackgroundWorkerDetailsPATCH

	envSpecificDetails, err := buildUpdateEnvSpecificDetails(
		runtime,
		input,
	)
	if err != nil {
		return nil, err
	}
	details.EnvSpecificDetails = envSpecificDetails
	if input.Plan != nil {
		details.Plan = paidPlan(toClientPlan(input.Plan))
	}
	if input.PreDeployCommand != nil {
		details.PreDeployCommand = input.PreDeployCommand
	}
	if input.MaxShutdownDelay != nil {
		details.MaxShutdownDelaySeconds = maxShutdownDelayFromInput(input.MaxShutdownDelay)
	}
	if input.Previews != nil {
		details.Previews = previewsFromInput(input.Previews)
	}

	if details == (client.BackgroundWorkerDetailsPATCH{}) {
		return nil, nil
	}

	serviceDetails := &client.ServicePATCH_ServiceDetails{}
	if err := serviceDetails.FromBackgroundWorkerDetailsPATCH(details); err != nil {
		return nil, err
	}
	return serviceDetails, nil
}

func buildCronJobUpdateDetails(
	runtime *client.ServiceRuntime,
	input servicetypes.ServiceUpdateInput,
) (*client.ServicePATCH_ServiceDetails, error) {
	var details client.CronJobDetailsPATCH

	envSpecificDetails, err := buildUpdateEnvSpecificDetailsForCronJob(
		runtime,
		input,
	)
	if err != nil {
		return nil, err
	}
	details.EnvSpecificDetails = envSpecificDetails
	if input.Plan != nil {
		details.Plan = paidPlan(toClientPlan(input.Plan))
	}
	if input.CronSchedule != nil {
		details.Schedule = input.CronSchedule
	}

	if details == (client.CronJobDetailsPATCH{}) {
		return nil, nil
	}

	serviceDetails := &client.ServicePATCH_ServiceDetails{}
	if err := serviceDetails.FromCronJobDetailsPATCH(details); err != nil {
		return nil, err
	}
	return serviceDetails, nil
}

func buildUpdateEnvSpecificDetails(
	runtime *client.ServiceRuntime,
	input servicetypes.ServiceUpdateInput,
) (*client.EnvSpecificDetailsPATCH, error) {
	if runtime == nil {
		return nil, errors.New("current runtime is required to update runtime-specific fields")
	}

	switch {
	case *runtime == client.ServiceRuntimeDocker:
		return buildDockerUpdateEnvSpecificDetails(input)
	case *runtime == client.ServiceRuntimeImage:
		return buildImageUpdateEnvSpecificDetails(input)
	case isNativeServiceRuntime(*runtime):
		return buildNativeUpdateEnvSpecificDetails(input)
	default:
		return nil, fmt.Errorf("unsupported runtime %q", *runtime)
	}
}

func buildUpdateEnvSpecificDetailsForCronJob(
	runtime *client.ServiceRuntime,
	input servicetypes.ServiceUpdateInput,
) (*client.EnvSpecificDetailsPATCH, error) {
	if runtime == nil {
		return nil, errors.New("current runtime is required to update runtime-specific fields")
	}

	switch {
	case *runtime == client.ServiceRuntimeDocker:
		return buildDockerUpdateEnvSpecificDetailsForCronJob(input)
	case *runtime == client.ServiceRuntimeImage:
		return buildImageUpdateEnvSpecificDetailsForCronJob(input)
	case isNativeServiceRuntime(*runtime):
		return buildNativeUpdateEnvSpecificDetailsForCronJob(input)
	default:
		return nil, fmt.Errorf("unsupported runtime %q", *runtime)
	}
}

func buildNativeUpdateEnvSpecificDetails(input servicetypes.ServiceUpdateInput) (*client.EnvSpecificDetailsPATCH, error) {
	if input.RegistryCredential != nil {
		return nil, errors.New(registryCredentialUpdateIncompatibleRuntimeError)
	}
	if input.BuildCommand == nil && input.StartCommand == nil {
		return nil, nil
	}

	envSpecificDetails := &client.EnvSpecificDetailsPATCH{}
	if err := envSpecificDetails.FromNativeEnvironmentDetailsPATCH(client.NativeEnvironmentDetailsPATCH{
		BuildCommand: input.BuildCommand,
		StartCommand: input.StartCommand,
	}); err != nil {
		return nil, err
	}
	return envSpecificDetails, nil
}

func buildNativeUpdateEnvSpecificDetailsForCronJob(input servicetypes.ServiceUpdateInput) (*client.EnvSpecificDetailsPATCH, error) {
	if input.RegistryCredential != nil {
		return nil, errors.New(registryCredentialUpdateIncompatibleRuntimeError)
	}
	if input.BuildCommand == nil && input.CronCommand == nil {
		return nil, nil
	}

	envSpecificDetails := &client.EnvSpecificDetailsPATCH{}
	if err := envSpecificDetails.FromNativeEnvironmentDetailsPATCH(client.NativeEnvironmentDetailsPATCH{
		BuildCommand: input.BuildCommand,
		StartCommand: input.CronCommand,
	}); err != nil {
		return nil, err
	}
	return envSpecificDetails, nil
}

func buildImageUpdateEnvSpecificDetails(input servicetypes.ServiceUpdateInput) (*client.EnvSpecificDetailsPATCH, error) {
	if input.BuildCommand != nil || input.StartCommand != nil {
		return nil, errors.New("--build-command and --start-command are only supported for native runtimes")
	}
	if input.RegistryCredential != nil {
		return nil, errors.New(registryCredentialUpdateIncompatibleRuntimeError)
	}
	return nil, nil
}

func buildImageUpdateEnvSpecificDetailsForCronJob(input servicetypes.ServiceUpdateInput) (*client.EnvSpecificDetailsPATCH, error) {
	if input.BuildCommand != nil || input.CronCommand != nil {
		return nil, errors.New("--build-command and --cron-command are only supported for native runtimes")
	}
	if input.RegistryCredential != nil {
		return nil, errors.New(registryCredentialUpdateIncompatibleRuntimeError)
	}
	return nil, nil
}

func buildDockerUpdateEnvSpecificDetails(input servicetypes.ServiceUpdateInput) (*client.EnvSpecificDetailsPATCH, error) {
	if input.BuildCommand != nil || input.StartCommand != nil {
		return nil, errors.New("--build-command and --start-command are only supported for native runtimes")
	}
	if input.RegistryCredential == nil {
		return nil, nil
	}

	return dockerEnvSpecificDetailsFromInput(nil, input.RegistryCredential)
}

func buildDockerUpdateEnvSpecificDetailsForCronJob(input servicetypes.ServiceUpdateInput) (*client.EnvSpecificDetailsPATCH, error) {
	if input.BuildCommand != nil {
		return nil, errors.New("--build-command is only supported for native runtimes")
	}
	if input.CronCommand == nil && input.RegistryCredential == nil {
		return nil, nil
	}

	return dockerEnvSpecificDetailsFromInput(input.CronCommand, input.RegistryCredential)
}

func dockerEnvSpecificDetailsFromInput(dockerCommand *string, registryCredentialID *string) (*client.EnvSpecificDetailsPATCH, error) {
	envSpecificDetails := &client.EnvSpecificDetailsPATCH{}
	if err := envSpecificDetails.FromDockerDetailsPATCH(client.DockerDetailsPATCH{
		DockerCommand:        dockerCommand,
		RegistryCredentialId: registryCredentialID,
	}); err != nil {
		return nil, err
	}
	return envSpecificDetails, nil
}

func isNativeServiceRuntime(runtime client.ServiceRuntime) bool {
	return runtime != "" && runtime != client.ServiceRuntimeDocker && runtime != client.ServiceRuntimeImage
}

func buildStaticSiteUpdateDetails(input servicetypes.ServiceUpdateInput) (*client.ServicePATCH_ServiceDetails, error) {
	var details client.StaticSiteDetailsPATCH

	if input.RegistryCredential != nil {
		return nil, errors.New(registryCredentialUpdateIncompatibleRuntimeError)
	}
	if input.BuildCommand != nil {
		details.BuildCommand = input.BuildCommand
	}
	if input.PublishDirectory != nil {
		details.PublishPath = input.PublishDirectory
	}
	if input.Previews != nil {
		details.Previews = previewsFromInput(input.Previews)
	}
	if len(input.IPAllowList) > 0 {
		ipAllowList, err := ParseIPAllowListInputs(input.IPAllowList)
		if err != nil {
			return nil, err
		}
		details.IpAllowList = ipAllowList
	}

	if details == (client.StaticSiteDetailsPATCH{}) {
		return nil, nil
	}

	serviceDetails := &client.ServicePATCH_ServiceDetails{}
	if err := serviceDetails.FromStaticSiteDetailsPATCH(details); err != nil {
		return nil, err
	}
	return serviceDetails, nil
}
