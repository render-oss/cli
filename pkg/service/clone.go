package service

import (
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

type sourceDefaults struct {
	serviceType          client.ServiceType
	rootDirectory        *string
	environmentID        *string
	repo                 *string
	branch               *string
	image                *string
	runtime              *client.ServiceRuntime
	registryCredentialID *string
	cronSchedule         *string
	cronCommand          *string
}

// ServiceFromAPI generates a Service type with values from an API response
func ServiceFromAPI(input *servicetypes.Service, source *client.Service) {
	if input == nil || source == nil {
		return
	}

	defaults := extractCloneSourceDefaults(source)

	applyBaseDefaults(input, defaults)
	applySourceDefaults(input, defaults)
	applyRuntimeDefaults(input, defaults)
	applyRegistryCredentialDefault(input, defaults)
	applyCronDefaults(input, defaults)
}

func extractCloneSourceDefaults(source *client.Service) sourceDefaults {
	defaults := sourceDefaults{
		serviceType:   source.Type,
		rootDirectory: pointers.From(source.RootDir),
		environmentID: source.EnvironmentId,
		repo:          source.Repo,
		branch:        source.Branch,
		image:         source.ImagePath,
	}

	if source.RegistryCredential != nil {
		defaults.registryCredentialID = pointers.From(source.RegistryCredential.Id)
	}

	runtime, envSpecificDetails, ok := runtimeAndEnvSpecificDetailsFromSource(source)
	if ok {
		defaults.runtime = &runtime
	}

	// Extract registry credential from docker env-specific details
	if defaults.registryCredentialID == nil && ok && runtime == client.ServiceRuntimeDocker {
		if id, ok := registryCredFromDockerDetails(envSpecificDetails); ok {
			defaults.registryCredentialID = pointers.From(id)
		}
	}

	if source.Type == client.CronJob {
		cronDetails, err := source.ServiceDetails.AsCronJobDetails()
		if err == nil {
			defaults.cronSchedule = pointers.From(cronDetails.Schedule)
			native, err := cronDetails.EnvSpecificDetails.AsNativeEnvironmentDetails()
			if err == nil {
				defaults.cronCommand = pointers.From(native.StartCommand)
			}
		}
	}

	return defaults
}

func applyBaseDefaults(input *servicetypes.Service, defaults sourceDefaults) {
	input.Type = withDefaultFromValue(input.Type, defaults.serviceType)
	input.RootDirectory = withDefault(input.RootDirectory, defaults.rootDirectory)
	input.EnvironmentID = withDefault(input.EnvironmentID, defaults.environmentID)
}

// applySourceDefaults fills source location defaults with precedence rules:
// - If image is explicitly provided, do not backfill repo/branch.
// - If repo is explicitly provided, do not backfill image/registry.
// - If neither is explicitly provided, prefer repo defaults first, then image fallback.
func applySourceDefaults(input *servicetypes.Service, defaults sourceDefaults) {
	if input.Image == nil {
		input.Repo = withDefault(input.Repo, defaults.repo)
		input.Branch = withDefault(input.Branch, defaults.branch)
	}

	if input.Repo == nil {
		input.Image = withDefault(input.Image, defaults.image)
	}

	if input.Repo == nil && input.Image != nil {
		input.RegistryCredential = withDefault(input.RegistryCredential, defaults.registryCredentialID)
	}
}

func applyRuntimeDefaults(input *servicetypes.Service, defaults sourceDefaults) {
	if input.Image != nil {
		input.Runtime = withDefaultFromValue(input.Runtime, servicetypes.ServiceRuntimeImage)
		return
	}

	if defaults.runtime == nil {
		return
	}
	if *defaults.runtime == client.ServiceRuntimeImage {
		return
	}

	input.Runtime = withDefaultFromValue(input.Runtime, *defaults.runtime)
}

func applyRegistryCredentialDefault(input *servicetypes.Service, defaults sourceDefaults) {
	if !input.SupportsRegistryCredentials() {
		return
	}

	input.RegistryCredential = withDefault(input.RegistryCredential, defaults.registryCredentialID)
}

func applyCronDefaults(input *servicetypes.Service, defaults sourceDefaults) {
	input.CronSchedule = withDefault(input.CronSchedule, defaults.cronSchedule)
	input.CronCommand = withDefault(input.CronCommand, defaults.cronCommand)
}

func withDefault(dst *string, src *string) *string {
	if src == nil || dst != nil {
		return dst
	}

	return pointers.From(*src)
}

func withDefaultFromValue[S ~string](dst *string, src S) *string {
	return withDefault(dst, pointers.From(string(src)))
}

// RuntimeFromSourceService extracts runtime from a service when that service type has a runtime field.
func RuntimeFromSourceService(source *client.Service) (client.ServiceRuntime, bool) {
	runtime, _, ok := runtimeAndEnvSpecificDetailsFromSource(source)
	return runtime, ok
}

// RegistryCredentialIDFromSourceService extracts a registry credential ID from
// the source service when one can be inferred from summary or docker details.
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
