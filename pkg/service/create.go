package service

import (
	"fmt"

	"github.com/render-oss/cli/v2/pkg/client"
	"github.com/render-oss/cli/v2/pkg/pointers"
	types "github.com/render-oss/cli/v2/pkg/types"
	servicetypes "github.com/render-oss/cli/v2/pkg/types/service"
)

// BuildCreateRequest maps validated CLI input into the API create-service request body.
func BuildCreateRequest(cliInput servicetypes.ServiceCreateInput, ownerID string) (client.CreateServiceJSONRequestBody, error) {
	serviceType, err := cliInput.OptionalServiceType()
	if err != nil {
		return client.CreateServiceJSONRequestBody{}, err
	}
	if serviceType == nil {
		return client.CreateServiceJSONRequestBody{}, fmt.Errorf("type is required")
	}

	runtime, err := cliInput.OptionalServiceRuntime()
	if err != nil {
		return client.CreateServiceJSONRequestBody{}, err
	}

	region, err := cliInput.OptionalRegion()
	if err != nil {
		return client.CreateServiceJSONRequestBody{}, err
	}

	typedServiceType := toClientType(serviceType)
	typedRuntime := toClientRuntime(runtime)
	typedRegion := toClientRegion(region)
	typedPlan := toClientPlan(cliInput.Plan)

	// When an image is provided without an explicit runtime, default to "image" runtime.
	// This allows users to omit --runtime when using --image.
	if cliInput.Image != nil && typedRuntime == nil {
		defaultRuntime := client.ServiceRuntimeImage
		typedRuntime = &defaultRuntime
	}

	envVars, err := parseEnvVarInputs(cliInput.EnvVars)
	if err != nil {
		return client.CreateServiceJSONRequestBody{}, err
	}

	secretFiles, err := ResolveSecretFileInputs(cliInput.SecretFiles)
	if err != nil {
		return client.CreateServiceJSONRequestBody{}, err
	}

	if serviceTypeRequiresRuntime(*typedServiceType) && typedRuntime == nil {
		return client.CreateServiceJSONRequestBody{}, fmt.Errorf("runtime is required")
	}

	body := client.CreateServiceJSONRequestBody{
		Name:    cliInput.Name,
		OwnerId: ownerID,
		Type:    *typedServiceType,
	}
	buildFilter := buildFilterFromInputs(cliInput.BuildFilterPaths, cliInput.BuildFilterIgnoredPaths)
	if buildFilter != nil {
		body.BuildFilter = buildFilter
	}

	if cliInput.Repo != nil {
		body.Repo = cliInput.Repo
	}
	if cliInput.Branch != nil {
		body.Branch = cliInput.Branch
	}
	if cliInput.Image != nil {
		body.Image = &client.Image{ImagePath: *cliInput.Image}
		if cliInput.RegistryCredential != nil {
			body.Image.RegistryCredentialId = cliInput.RegistryCredential
		}
	}
	if cliInput.EnvironmentID != nil {
		body.EnvironmentId = cliInput.EnvironmentID
	}
	if cliInput.RootDirectory != nil {
		body.RootDir = cliInput.RootDirectory
	}
	if cliInput.AutoDeploy != nil {
		autoDeploy := client.AutoDeployYes
		if !*cliInput.AutoDeploy {
			autoDeploy = client.AutoDeployNo
		}
		body.AutoDeploy = pointers.From(autoDeploy)
	}

	body.EnvVars = pointers.FromArray(envVars)
	body.SecretFiles = pointers.FromArray(secretFiles)

	serviceDetails, err := buildServiceDetails(cliInput, *typedServiceType, typedRuntime, typedRegion, typedPlan)
	if err != nil {
		return client.CreateServiceJSONRequestBody{}, err
	}
	body.ServiceDetails = serviceDetails

	return body, nil
}

func parseEnvVarInputs(values []string) ([]client.EnvVarInput, error) {
	if len(values) == 0 {
		return nil, nil
	}

	parsed := make([]client.EnvVarInput, 0, len(values))
	for _, raw := range values {
		envVarInput, err := types.ParseEnvVar(raw)
		if err != nil {
			return nil, err
		}
		var envVar client.EnvVarInput
		if err := envVar.FromEnvVarKeyValue(client.EnvVarKeyValue{
			Key:   envVarInput.Key,
			Value: envVarInput.Value,
		}); err != nil {
			return nil, err
		}
		parsed = append(parsed, envVar)
	}

	return parsed, nil
}

func buildServiceDetails(
	cliInput servicetypes.ServiceCreateInput,
	serviceType client.ServiceType,
	runtime *client.ServiceRuntime,
	region *client.Region,
	plan *client.Plan,
) (*client.ServicePOST_ServiceDetails, error) {
	serviceDetails := &client.ServicePOST_ServiceDetails{}

	switch serviceType {
	case client.WebService:
		envSpecificDetails, err := buildRuntimeEnvSpecificDetails(
			runtime,
			cliInput.BuildCommand,
			cliInput.StartCommand,
			cliInput.RegistryCredential,
		)
		if err != nil {
			return nil, err
		}
		ipAllowList, err := ParseIPAllowListInputs(cliInput.IPAllowList)
		if err != nil {
			return nil, err
		}

		details := client.WebServiceDetailsPOST{
			Runtime:                 *runtime,
			Plan:                    plan,
			Region:                  region,
			EnvSpecificDetails:      envSpecificDetails,
			HealthCheckPath:         cliInput.HealthCheckPath,
			PreDeployCommand:        cliInput.PreDeployCommand,
			NumInstances:            cliInput.NumInstances,
			MaxShutdownDelaySeconds: maxShutdownDelayFromInput(cliInput.MaxShutdownDelay),
			Previews:                previewsFromInput(cliInput.Previews),
			MaintenanceMode:         maintenanceModeFromInput(cliInput.MaintenanceMode, cliInput.MaintenanceModeURI),
			IpAllowList:             ipAllowList,
		}
		if err := serviceDetails.FromWebServiceDetailsPOST(details); err != nil {
			return nil, err
		}
	case client.PrivateService:
		envSpecificDetails, err := buildRuntimeEnvSpecificDetails(
			runtime,
			cliInput.BuildCommand,
			cliInput.StartCommand,
			cliInput.RegistryCredential,
		)
		if err != nil {
			return nil, err
		}

		details := client.PrivateServiceDetailsPOST{
			Runtime:                 *runtime,
			Plan:                    paidPlan(plan),
			Region:                  region,
			EnvSpecificDetails:      envSpecificDetails,
			PreDeployCommand:        cliInput.PreDeployCommand,
			NumInstances:            cliInput.NumInstances,
			MaxShutdownDelaySeconds: maxShutdownDelayFromInput(cliInput.MaxShutdownDelay),
			Previews:                previewsFromInput(cliInput.Previews),
		}
		if err := serviceDetails.FromPrivateServiceDetailsPOST(details); err != nil {
			return nil, err
		}
	case client.BackgroundWorker:
		envSpecificDetails, err := buildRuntimeEnvSpecificDetails(
			runtime,
			cliInput.BuildCommand,
			cliInput.StartCommand,
			cliInput.RegistryCredential,
		)
		if err != nil {
			return nil, err
		}

		details := client.BackgroundWorkerDetailsPOST{
			Runtime:                 *runtime,
			Plan:                    paidPlan(plan),
			Region:                  region,
			EnvSpecificDetails:      envSpecificDetails,
			PreDeployCommand:        cliInput.PreDeployCommand,
			NumInstances:            cliInput.NumInstances,
			MaxShutdownDelaySeconds: maxShutdownDelayFromInput(cliInput.MaxShutdownDelay),
			Previews:                previewsFromInput(cliInput.Previews),
		}
		if err := serviceDetails.FromBackgroundWorkerDetailsPOST(details); err != nil {
			return nil, err
		}
	case client.CronJob:
		envSpecificDetails, err := buildCronEnvSpecificDetails(cliInput.BuildCommand, cliInput.CronCommand)
		if err != nil {
			return nil, err
		}

		details := client.CronJobDetailsPOST{
			Runtime:            *runtime,
			Schedule:           pointers.ValueOrDefault(cliInput.CronSchedule, ""),
			Plan:               paidPlan(plan),
			Region:             region,
			EnvSpecificDetails: envSpecificDetails,
		}
		if err := serviceDetails.FromCronJobDetailsPOST(details); err != nil {
			return nil, err
		}
	case client.StaticSite:
		ipAllowList, err := ParseIPAllowListInputs(cliInput.IPAllowList)
		if err != nil {
			return nil, err
		}
		details := client.StaticSiteDetailsPOST{
			BuildCommand: cliInput.BuildCommand,
			PublishPath:  cliInput.PublishDirectory,
			Previews:     previewsFromInput(cliInput.Previews),
			IpAllowList:  ipAllowList,
		}
		if err := serviceDetails.FromStaticSiteDetailsPOST(details); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported service type %q", serviceType)
	}

	return serviceDetails, nil
}

// paidPlan converts *client.Plan to *client.PaidPlan for API requests.
// PaidPlan is an alias for Plan in the API schema, but the generated Go types
// are distinct. This helper handles the conversion.
func paidPlan(plan *client.Plan) *client.PaidPlan {
	if plan == nil {
		return nil
	}
	value := client.PaidPlan(*plan)
	return &value
}

func serviceTypeRequiresRuntime(serviceType client.ServiceType) bool {
	return serviceType != client.StaticSite
}

func buildNativeRuntimeEnvSpecificDetails(buildCommand *string, startCommand *string) (*client.EnvSpecificDetailsPOST, error) {
	hasCommandOverrides := buildCommand != nil || startCommand != nil
	if !hasCommandOverrides {
		return nil, nil
	}

	envSpecificDetails := &client.EnvSpecificDetailsPOST{}
	if err := envSpecificDetails.FromNativeEnvironmentDetailsPOST(client.NativeEnvironmentDetailsPOST{
		BuildCommand: pointers.ValueOrDefault(buildCommand, ""),
		StartCommand: pointers.ValueOrDefault(startCommand, ""),
	}); err != nil {
		return nil, err
	}

	return envSpecificDetails, nil
}

func buildRuntimeEnvSpecificDetails(
	runtime *client.ServiceRuntime,
	buildCommand *string,
	startCommand *string,
	registryCredentialID *string,
) (*client.EnvSpecificDetailsPOST, error) {
	if runtime != nil && *runtime == client.ServiceRuntimeDocker {
		if registryCredentialID == nil {
			return nil, nil
		}

		envSpecificDetails := &client.EnvSpecificDetailsPOST{}
		if err := envSpecificDetails.FromDockerDetailsPOST(client.DockerDetailsPOST{
			RegistryCredentialId: registryCredentialID,
		}); err != nil {
			return nil, err
		}
		return envSpecificDetails, nil
	}

	return buildNativeRuntimeEnvSpecificDetails(buildCommand, startCommand)
}

// buildCronEnvSpecificDetails returns *EnvSpecificDetails (not the POST variant)
// because CronJobDetailsPOST.EnvSpecificDetails uses the non-POST type in the API schema.
// This differs from buildNativeRuntimeEnvSpecificDetails which returns *EnvSpecificDetailsPOST.
func buildCronEnvSpecificDetails(buildCommand *string, cronCommand *string) (*client.EnvSpecificDetails, error) {
	hasCommandOverrides := buildCommand != nil || cronCommand != nil
	if !hasCommandOverrides {
		return nil, nil
	}

	envSpecificDetails := &client.EnvSpecificDetails{}
	if err := envSpecificDetails.FromNativeEnvironmentDetails(client.NativeEnvironmentDetails{
		BuildCommand: pointers.ValueOrDefault(buildCommand, ""),
		StartCommand: pointers.ValueOrDefault(cronCommand, ""),
	}); err != nil {
		return nil, err
	}

	return envSpecificDetails, nil
}

func toClientType(value *servicetypes.ServiceType) *client.ServiceType {
	if value == nil {
		return nil
	}
	typed := client.ServiceType(*value)
	return &typed
}

func toClientRuntime(value *servicetypes.ServiceRuntime) *client.ServiceRuntime {
	if value == nil {
		return nil
	}
	typed := client.ServiceRuntime(*value)
	return &typed
}

func toClientRegion(value *types.Region) *client.Region {
	if value == nil {
		return nil
	}
	typed := client.Region(*value)
	return &typed
}

func toClientPlan(value *string) *client.Plan {
	if value == nil {
		return nil
	}
	typed := client.Plan(*value)
	return &typed
}

func buildFilterFromInputs(paths []string, ignoredPaths []string) *client.BuildFilter {
	if len(paths) == 0 && len(ignoredPaths) == 0 {
		return nil
	}
	return &client.BuildFilter{
		Paths:        append([]string(nil), paths...),
		IgnoredPaths: append([]string(nil), ignoredPaths...),
	}
}

func ParseIPAllowListInputs(raw []string) (*[]client.CidrBlockAndDescription, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	entries := make([]client.CidrBlockAndDescription, 0, len(raw))
	for _, entry := range raw {
		cidr, description, err := types.ParseIPAllowListEntry(entry)
		if err != nil {
			return nil, err
		}
		entries = append(entries, client.CidrBlockAndDescription{
			CidrBlock:   cidr,
			Description: description,
		})
	}
	return &entries, nil
}

func previewsFromInput(previews *servicetypes.PreviewsGeneration) *client.Previews {
	if previews == nil {
		return nil
	}
	gen := client.PreviewsGeneration(*previews)
	return &client.Previews{Generation: &gen}
}

func maxShutdownDelayFromInput(value *int) *client.MaxShutdownDelaySeconds {
	if value == nil {
		return nil
	}
	v := client.MaxShutdownDelaySeconds(*value)
	return &v
}

func maintenanceModeFromInput(enabled *bool, uri *string) *client.MaintenanceMode {
	if enabled == nil && uri == nil {
		return nil
	}
	return &client.MaintenanceMode{
		Enabled: pointers.ValueOrDefault(enabled, false),
		Uri:     pointers.ValueOrDefault(uri, ""),
	}
}
