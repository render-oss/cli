package service

import (
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	types "github.com/render-oss/cli/pkg/types"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

type sourceDefaults struct {
	serviceType             servicetypes.ServiceType
	rootDirectory           *string
	environmentID           *string
	repo                    *string
	branch                  *string
	image                   *string
	runtime                 *servicetypes.ServiceRuntime
	registryCredentialID    *string
	cronSchedule            *string
	cronCommand             *string
	region                  *types.Region
	plan                    *string
	buildCommand            *string
	startCommand            *string
	preDeployCommand        *string
	healthCheckPath         *string
	publishDirectory        *string
	autoDeploy              *bool
	buildFilterPaths        []string
	buildFilterIgnoredPaths []string
	numInstances            *int
	maxShutdownDelay        *int
	previews                *servicetypes.PreviewsGeneration
	maintenanceMode         *bool
	maintenanceModeURI      *string
	ipAllowList             []string
}

// ServiceFromAPI generates a Service type with values from an API response
func ServiceFromAPI(input *servicetypes.ServiceCreateInput, source *client.Service) {
	if input == nil || source == nil {
		return
	}
	extracted := extractCloneSourceDefaults(source)
	defaults := mapSourceDefaultsToServiceInput(extracted)

	applyBaseDefaults(input, defaults)
	applySourceDefaults(input, defaults)
	applyRuntimeDefaults(input, defaults)
	applyRegistryCredentialDefault(input, defaults)
	applyCronDefaults(input, defaults)
	applyAdditionalCloneDefaults(input, defaults)
	applyBuildFilterDefaults(input, defaults)
	applyServiceTypeDefaults(input, defaults)
	applyIPAllowListDefaults(input, defaults)
}

func mapSourceDefaultsToServiceInput(defaults sourceDefaults) servicetypes.ServiceCreateInput {
	mapped := servicetypes.ServiceCreateInput{
		Type:                    pointers.From(defaults.serviceType),
		RootDirectory:           defaults.rootDirectory,
		EnvironmentID:           defaults.environmentID,
		Repo:                    defaults.repo,
		Branch:                  defaults.branch,
		Image:                   defaults.image,
		RegistryCredential:      defaults.registryCredentialID,
		CronSchedule:            defaults.cronSchedule,
		CronCommand:             defaults.cronCommand,
		Region:                  defaults.region,
		Plan:                    defaults.plan,
		BuildCommand:            defaults.buildCommand,
		StartCommand:            defaults.startCommand,
		PreDeployCommand:        defaults.preDeployCommand,
		HealthCheckPath:         defaults.healthCheckPath,
		PublishDirectory:        defaults.publishDirectory,
		AutoDeploy:              defaults.autoDeploy,
		BuildFilterPaths:        append([]string(nil), defaults.buildFilterPaths...),
		BuildFilterIgnoredPaths: append([]string(nil), defaults.buildFilterIgnoredPaths...),
		NumInstances:            defaults.numInstances,
		MaxShutdownDelay:        defaults.maxShutdownDelay,
		Previews:                defaults.previews,
		MaintenanceMode:         defaults.maintenanceMode,
		MaintenanceModeURI:      defaults.maintenanceModeURI,
		IPAllowList:             append([]string(nil), defaults.ipAllowList...),
	}

	if defaults.runtime != nil {
		mapped.Runtime = pointers.From(*defaults.runtime)
	}

	return mapped
}

func extractCloneSourceDefaults(source *client.Service) sourceDefaults {
	defaults := sourceDefaults{
		serviceType:   servicetypes.ServiceType(source.Type),
		rootDirectory: pointers.From(source.RootDir),
		environmentID: source.EnvironmentId,
		repo:          source.Repo,
		branch:        source.Branch,
		image:         source.ImagePath,
	}
	switch source.AutoDeploy {
	case client.AutoDeployYes:
		defaults.autoDeploy = pointers.From(true)
	case client.AutoDeployNo:
		defaults.autoDeploy = pointers.From(false)
	}

	if source.RegistryCredential != nil {
		defaults.registryCredentialID = pointers.From(source.RegistryCredential.Id)
	}

	runtime, envSpecificDetails, ok := runtimeAndEnvSpecificDetailsFromSource(source)
	if ok {
		typedRuntime := servicetypes.ServiceRuntime(runtime)
		defaults.runtime = &typedRuntime
	}

	// Extract registry credential from docker env-specific details
	if defaults.registryCredentialID == nil && ok && runtime == client.ServiceRuntimeDocker {
		if id, ok := registryCredFromDockerDetails(envSpecificDetails); ok {
			defaults.registryCredentialID = pointers.From(id)
		}
	}

	if source.BuildFilter != nil {
		if len(source.BuildFilter.Paths) > 0 {
			defaults.buildFilterPaths = append([]string(nil), source.BuildFilter.Paths...)
		}
		if len(source.BuildFilter.IgnoredPaths) > 0 {
			defaults.buildFilterIgnoredPaths = append([]string(nil), source.BuildFilter.IgnoredPaths...)
		}
	}

	switch source.Type {
	case client.CronJob:
		details, err := source.ServiceDetails.AsCronJobDetails()
		if err == nil {
			defaults.region = pointers.From(types.Region(details.Region))
			defaults.plan = pointers.From(string(details.Plan))
			defaults.cronSchedule = pointers.From(details.Schedule)
			defaults.cronCommand = startCommandFromEnvDetails(details.EnvSpecificDetails)
			defaults.buildCommand = buildCommandFromEnvDetails(details.EnvSpecificDetails)
		}
	case client.WebService:
		details, err := source.ServiceDetails.AsWebServiceDetails()
		if err == nil {
			defaults.region = pointers.From(types.Region(details.Region))
			defaults.plan = pointers.From(string(details.Plan))
			defaults.healthCheckPath = pointers.From(details.HealthCheckPath)
			defaults.buildCommand = buildCommandFromEnvDetails(details.EnvSpecificDetails)
			defaults.startCommand = startCommandFromEnvDetails(details.EnvSpecificDetails)
			defaults.preDeployCommand = preDeployCommandFromEnvDetails(details.EnvSpecificDetails)
			// Only materialize NumInstances if it's > 0
			if details.NumInstances > 0 {
				defaults.numInstances = pointers.From(details.NumInstances)
			}
			// MaxShutdownDelaySeconds is already *int, only use if not nil
			defaults.maxShutdownDelay = details.MaxShutdownDelaySeconds
			defaults.previews = previewsGeneration(details.Previews)
			if details.MaintenanceMode != nil {
				defaults.maintenanceMode = pointers.From(details.MaintenanceMode.Enabled)
				defaults.maintenanceModeURI = pointers.From(details.MaintenanceMode.Uri)
			}
			if details.IpAllowList != nil {
				defaults.ipAllowList = formatIPAllowListEntries(*details.IpAllowList)
			}
		}
	case client.PrivateService:
		details, err := source.ServiceDetails.AsPrivateServiceDetails()
		if err == nil {
			defaults.region = pointers.From(types.Region(details.Region))
			defaults.plan = pointers.From(string(details.Plan))
			defaults.buildCommand = buildCommandFromEnvDetails(details.EnvSpecificDetails)
			defaults.startCommand = startCommandFromEnvDetails(details.EnvSpecificDetails)
			defaults.preDeployCommand = preDeployCommandFromEnvDetails(details.EnvSpecificDetails)
			// Only materialize NumInstances if it's > 0
			if details.NumInstances > 0 {
				defaults.numInstances = pointers.From(details.NumInstances)
			}
			// MaxShutdownDelaySeconds is already *int, only use if not nil
			defaults.maxShutdownDelay = details.MaxShutdownDelaySeconds
			defaults.previews = previewsGeneration(details.Previews)
		}
	case client.BackgroundWorker:
		details, err := source.ServiceDetails.AsBackgroundWorkerDetails()
		if err == nil {
			defaults.region = pointers.From(types.Region(details.Region))
			defaults.plan = pointers.From(string(details.Plan))
			defaults.buildCommand = buildCommandFromEnvDetails(details.EnvSpecificDetails)
			defaults.startCommand = startCommandFromEnvDetails(details.EnvSpecificDetails)
			defaults.preDeployCommand = preDeployCommandFromEnvDetails(details.EnvSpecificDetails)
			// Only materialize NumInstances if it's > 0
			if details.NumInstances > 0 {
				defaults.numInstances = pointers.From(details.NumInstances)
			}
			// MaxShutdownDelaySeconds is already *int, only use if not nil
			defaults.maxShutdownDelay = details.MaxShutdownDelaySeconds
			defaults.previews = previewsGeneration(details.Previews)
		}
	case client.StaticSite:
		details, err := source.ServiceDetails.AsStaticSiteDetails()
		if err == nil {
			defaults.buildCommand = pointers.From(details.BuildCommand)
			defaults.publishDirectory = pointers.From(details.PublishPath)
			defaults.previews = previewsGeneration(details.Previews)
			if details.IpAllowList != nil {
				defaults.ipAllowList = formatIPAllowListEntries(*details.IpAllowList)
			}
		}
	}

	return defaults
}

func applyBaseDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	input.Type = withDefaultAlias(input.Type, defaults.Type)
	input.RootDirectory = withDefault(input.RootDirectory, defaults.RootDirectory)
	input.EnvironmentID = withDefault(input.EnvironmentID, defaults.EnvironmentID)
}

// applySourceDefaults fills source location defaults with precedence rules:
// - If image is explicitly provided, do not backfill repo/branch.
// - If repo is explicitly provided, do not backfill image/registry.
// - If neither is explicitly provided, prefer repo defaults first, then image fallback.
func applySourceDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	if input.Image == nil {
		input.Repo = withDefault(input.Repo, defaults.Repo)
		input.Branch = withDefault(input.Branch, defaults.Branch)
	}

	if input.Repo == nil {
		input.Image = withDefault(input.Image, defaults.Image)
	}

	if input.Repo == nil && input.Image != nil {
		input.RegistryCredential = withDefault(input.RegistryCredential, defaults.RegistryCredential)
	}
}

func applyRuntimeDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	if input.Image != nil {
		input.Runtime = withDefaultAliasFromValue(input.Runtime, servicetypes.ServiceRuntimeImage)
		return
	}

	if defaults.Runtime == nil {
		return
	}
	if *defaults.Runtime == servicetypes.ServiceRuntimeImage {
		return
	}

	input.Runtime = withDefaultAlias(input.Runtime, defaults.Runtime)
}

func applyRegistryCredentialDefault(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	if !input.SupportsRegistryCredentials() {
		return
	}

	input.RegistryCredential = withDefault(input.RegistryCredential, defaults.RegistryCredential)
}

func applyCronDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	input.CronSchedule = withDefault(input.CronSchedule, defaults.CronSchedule)
	input.CronCommand = withDefault(input.CronCommand, defaults.CronCommand)
}

func applyAdditionalCloneDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	input.Region = withDefaultAlias(input.Region, defaults.Region)
	input.Plan = withDefault(input.Plan, defaults.Plan)
	input.BuildCommand = withDefault(input.BuildCommand, defaults.BuildCommand)
	input.StartCommand = withDefault(input.StartCommand, defaults.StartCommand)
	input.PreDeployCommand = withDefault(input.PreDeployCommand, defaults.PreDeployCommand)
	input.HealthCheckPath = withDefault(input.HealthCheckPath, defaults.HealthCheckPath)
	input.PublishDirectory = withDefault(input.PublishDirectory, defaults.PublishDirectory)
	input.AutoDeploy = withDefaultBool(input.AutoDeploy, defaults.AutoDeploy)
}

func applyBuildFilterDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	if len(input.BuildFilterPaths) == 0 && len(defaults.BuildFilterPaths) > 0 {
		input.BuildFilterPaths = append([]string(nil), defaults.BuildFilterPaths...)
	}
	if len(input.BuildFilterIgnoredPaths) == 0 && len(defaults.BuildFilterIgnoredPaths) > 0 {
		input.BuildFilterIgnoredPaths = append([]string(nil), defaults.BuildFilterIgnoredPaths...)
	}
}

func applyServiceTypeDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	input.NumInstances = withDefaultInt(input.NumInstances, defaults.NumInstances)
	input.MaxShutdownDelay = withDefaultInt(input.MaxShutdownDelay, defaults.MaxShutdownDelay)
	if input.Previews == nil && defaults.Previews != nil {
		input.Previews = defaults.Previews
	}
	input.MaintenanceMode = withDefaultBool(input.MaintenanceMode, defaults.MaintenanceMode)
	input.MaintenanceModeURI = withDefault(input.MaintenanceModeURI, defaults.MaintenanceModeURI)
}

func applyIPAllowListDefaults(input *servicetypes.ServiceCreateInput, defaults servicetypes.ServiceCreateInput) {
	entries := defaults.IPAllowList
	if len(input.IPAllowList) > 0 || len(entries) == 0 {
		return
	}
	input.IPAllowList = append([]string(nil), entries...)
}

func formatIPAllowListEntries(entries []client.CidrBlockAndDescription) []string {
	if len(entries) == 0 {
		return nil
	}
	parsed := make([]string, 0, len(entries))
	for _, entry := range entries {
		parsed = append(parsed, types.FormatIPAllowListEntry(entry.CidrBlock, entry.Description))
	}
	return parsed
}

func buildCommandFromEnvDetails(details client.EnvSpecificDetails) *string {
	native, err := details.AsNativeEnvironmentDetails()
	if err != nil {
		return nil
	}
	return pointers.From(native.BuildCommand)
}

func startCommandFromEnvDetails(details client.EnvSpecificDetails) *string {
	native, err := details.AsNativeEnvironmentDetails()
	if err != nil {
		return nil
	}
	return pointers.From(native.StartCommand)
}

func preDeployCommandFromEnvDetails(details client.EnvSpecificDetails) *string {
	native, err := details.AsNativeEnvironmentDetails()
	if err == nil {
		return native.PreDeployCommand
	}

	docker, err := details.AsDockerDetails()
	if err != nil {
		return nil
	}
	return docker.PreDeployCommand
}

// RuntimeFromSourceService extracts runtime from a service when that service type has a runtime field.
func RuntimeFromSourceService(source *client.Service) (client.ServiceRuntime, bool) {
	runtime, _, ok := runtimeAndEnvSpecificDetailsFromSource(source)
	return runtime, ok
}

// RegistryCredentialIDFromSourceService extracts a registry credential ID from
// the source service when one can be inferred from source or docker details.
func RegistryCredentialIDFromSourceService(source *client.Service) (string, bool) {
	if source == nil {
		return "", false
	}
	if source.RegistryCredential != nil {
		return source.RegistryCredential.Id, true
	}

	runtime, envSpecificDetails, ok := runtimeAndEnvSpecificDetailsFromSource(source)
	if !ok || string(runtime) != string(servicetypes.ServiceRuntimeDocker) {
		return "", false
	}

	return registryCredFromDockerDetails(envSpecificDetails)
}

func runtimeAndEnvSpecificDetailsFromSource(source *client.Service) (client.ServiceRuntime, client.EnvSpecificDetails, bool) {
	if source == nil {
		return "", client.EnvSpecificDetails{}, false
	}

	switch source.Type {
	case client.WebService:
		details, err := source.ServiceDetails.AsWebServiceDetails()
		if err != nil {
			return "", client.EnvSpecificDetails{}, false
		}
		return details.Runtime, details.EnvSpecificDetails, true
	case client.PrivateService:
		details, err := source.ServiceDetails.AsPrivateServiceDetails()
		if err != nil {
			return "", client.EnvSpecificDetails{}, false
		}
		return details.Runtime, details.EnvSpecificDetails, true
	case client.BackgroundWorker:
		details, err := source.ServiceDetails.AsBackgroundWorkerDetails()
		if err != nil {
			return "", client.EnvSpecificDetails{}, false
		}
		return details.Runtime, details.EnvSpecificDetails, true
	case client.CronJob:
		details, err := source.ServiceDetails.AsCronJobDetails()
		if err != nil {
			return "", client.EnvSpecificDetails{}, false
		}
		return details.Runtime, details.EnvSpecificDetails, true
	default:
		return "", client.EnvSpecificDetails{}, false
	}
}

func registryCredFromDockerDetails(details client.EnvSpecificDetails) (string, bool) {
	dockerDetails, err := details.AsDockerDetails()
	if err != nil || dockerDetails.RegistryCredential == nil {
		return "", false
	}

	return dockerDetails.RegistryCredential.Id, true
}

func previewsGeneration(p *client.Previews) *servicetypes.PreviewsGeneration {
	if p == nil || p.Generation == nil {
		return nil
	}
	return pointers.From(servicetypes.PreviewsGeneration(*p.Generation))
}

func withDefault(dst *string, src *string) *string {
	if src == nil || dst != nil {
		return dst
	}

	return pointers.From(*src)
}

func withDefaultAlias[T ~string](dst *T, src *T) *T {
	if src == nil || dst != nil {
		return dst
	}

	return pointers.From(*src)
}

func withDefaultAliasFromValue[T ~string](dst *T, src T) *T {
	return withDefaultAlias(dst, pointers.From(src))
}

func withDefaultInt(dst *int, src *int) *int {
	if src == nil || dst != nil {
		return dst
	}
	return pointers.From(*src)
}

func withDefaultBool(dst *bool, src *bool) *bool {
	if src == nil || dst != nil {
		return dst
	}
	return pointers.From(*src)
}
