package service

import (
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildUpdateRequest(t *testing.T) {
	nativeRuntime := client.ServiceRuntimeNode
	dockerRuntime := client.ServiceRuntimeDocker
	imageRuntime := client.ServiceRuntimeImage
	nativeWebService := serviceBefore(t, client.WebService, &nativeRuntime)
	dockerWebService := serviceBefore(t, client.WebService, &dockerRuntime)
	imageWebService := serviceBefore(t, client.WebService, &imageRuntime)
	nativePrivateService := serviceBefore(t, client.PrivateService, &nativeRuntime)
	nativeBackgroundWorker := serviceBefore(t, client.BackgroundWorker, &nativeRuntime)
	nativeCronJob := serviceBefore(t, client.CronJob, &nativeRuntime)
	dockerCronJob := serviceBefore(t, client.CronJob, &dockerRuntime)
	staticSite := serviceBefore(t, client.StaticSite, nil)

	t.Run("maps top-level fields", func(t *testing.T) {
		input := servicetypes.ServiceUpdateInput{
			Name:          "new-service",
			Repo:          pointers.From("https://github.com/render-examples/flask-hello-world"),
			Branch:        pointers.From("main"),
			RootDirectory: pointers.From("api"),
		}

		body, err := BuildUpdateRequest(nativeWebService, input)
		require.NoError(t, err)

		assert.Equal(t, pointers.From("new-service"), body.Name)
		assert.Equal(t, pointers.From("https://github.com/render-examples/flask-hello-world"), body.Repo)
		assert.Equal(t, pointers.From("main"), body.Branch)
		assert.Equal(t, pointers.From("api"), body.RootDir)
		assert.Nil(t, body.ServiceDetails)
	})

	t.Run("maps auto deploy", func(t *testing.T) {
		body, err := BuildUpdateRequest(nativeWebService, servicetypes.ServiceUpdateInput{
			AutoDeploy: pointers.From(false),
		})
		require.NoError(t, err)

		assert.Equal(t, pointers.From(client.AutoDeployNo), body.AutoDeploy)
	})

	t.Run("maps build filter", func(t *testing.T) {
		body, err := BuildUpdateRequest(nativeWebService, servicetypes.ServiceUpdateInput{
			BuildFilterPaths:        []string{"src/**", "go.mod"},
			BuildFilterIgnoredPaths: []string{"docs/**"},
		})
		require.NoError(t, err)

		require.NotNil(t, body.BuildFilter)
		assert.Equal(t, []string{"src/**", "go.mod"}, body.BuildFilter.Paths)
		assert.Equal(t, []string{"docs/**"}, body.BuildFilter.IgnoredPaths)
	})

	t.Run("omits unset fields", func(t *testing.T) {
		body, err := BuildUpdateRequest(nativeWebService, servicetypes.ServiceUpdateInput{})
		require.NoError(t, err)

		assert.Nil(t, body.Name)
		assert.Nil(t, body.Repo)
		assert.Nil(t, body.Branch)
		assert.Nil(t, body.RootDir)
		assert.Nil(t, body.AutoDeploy)
		assert.Nil(t, body.BuildFilter)
		assert.Nil(t, body.ServiceDetails)
	})

	t.Run("maps image", func(t *testing.T) {
		body, err := BuildUpdateRequest(imageWebService, servicetypes.ServiceUpdateInput{
			Image:              pointers.From("docker.io/render-examples/web:latest"),
			RegistryCredential: pointers.From("rc-123"),
		})
		require.NoError(t, err)

		require.NotNil(t, body.Image)
		assert.Equal(t, "docker.io/render-examples/web:latest", body.Image.ImagePath)
		assert.Equal(t, pointers.From("rc-123"), body.Image.RegistryCredentialId)
		assert.Nil(t, body.ServiceDetails)
	})

	t.Run("rejects runtime update", func(t *testing.T) {
		_, err := BuildUpdateRequest(nativeWebService, servicetypes.ServiceUpdateInput{
			Runtime: svcRuntime(servicetypes.ServiceRuntimeGo),
		})
		require.ErrorIs(t, err, servicetypes.ErrRuntimeUpdateNotSupported)
	})

	t.Run("maps web service details", func(t *testing.T) {
		previews := servicetypes.PreviewsGenerationManual
		body, err := BuildUpdateRequest(nativeWebService, servicetypes.ServiceUpdateInput{
			Plan:               pointers.From("starter"),
			HealthCheckPath:    pointers.From("/ready"),
			BuildCommand:       pointers.From("npm ci"),
			StartCommand:       pointers.From("npm start"),
			PreDeployCommand:   pointers.From("bin/migrate"),
			MaxShutdownDelay:   pointers.From(42),
			Previews:           &previews,
			MaintenanceMode:    pointers.From(true),
			MaintenanceModeURI: pointers.From("https://status.example.com"),
			IPAllowList:        []string{"cidr=203.0.113.5/32,description=office"},
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From(client.Plan("starter")), details.Plan)
		assert.Equal(t, pointers.From("/ready"), details.HealthCheckPath)
		require.NotNil(t, details.EnvSpecificDetails)
		envDetails, err := details.EnvSpecificDetails.AsNativeEnvironmentDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From("npm ci"), envDetails.BuildCommand)
		assert.Equal(t, pointers.From("npm start"), envDetails.StartCommand)
		assert.Equal(t, pointers.From("bin/migrate"), details.PreDeployCommand)
		assert.Equal(t, pointers.From(client.MaxShutdownDelaySeconds(42)), details.MaxShutdownDelaySeconds)
		require.NotNil(t, details.Previews)
		assert.Equal(t, pointers.From(client.PreviewsGenerationManual), details.Previews.Generation)
		require.NotNil(t, details.MaintenanceMode)
		assert.True(t, details.MaintenanceMode.Enabled)
		assert.Equal(t, "https://status.example.com", details.MaintenanceMode.Uri)
		require.NotNil(t, details.IpAllowList)
		require.Len(t, *details.IpAllowList, 1)
		assert.Equal(t, "203.0.113.5/32", (*details.IpAllowList)[0].CidrBlock)
	})

	t.Run("preserves existing maintenance mode uri when updating enabled", func(t *testing.T) {
		service := webServiceBefore(t, client.WebServiceDetails{
			Runtime: nativeRuntime,
			MaintenanceMode: &client.MaintenanceMode{
				Enabled: false,
				Uri:     "https://status.example.com",
			},
		})

		body, err := BuildUpdateRequest(service, servicetypes.ServiceUpdateInput{
			MaintenanceMode: pointers.From(true),
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPATCH()
		require.NoError(t, err)
		require.NotNil(t, details.MaintenanceMode)
		assert.True(t, details.MaintenanceMode.Enabled)
		assert.Equal(t, "https://status.example.com", details.MaintenanceMode.Uri)
	})

	t.Run("preserves existing maintenance mode enabled value when updating uri", func(t *testing.T) {
		service := webServiceBefore(t, client.WebServiceDetails{
			Runtime: nativeRuntime,
			MaintenanceMode: &client.MaintenanceMode{
				Enabled: true,
				Uri:     "https://status.example.com",
			},
		})

		body, err := BuildUpdateRequest(service, servicetypes.ServiceUpdateInput{
			MaintenanceModeURI: pointers.From("https://maintenance.example.com"),
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPATCH()
		require.NoError(t, err)
		require.NotNil(t, details.MaintenanceMode)
		assert.True(t, details.MaintenanceMode.Enabled)
		assert.Equal(t, "https://maintenance.example.com", details.MaintenanceMode.Uri)
	})

	t.Run("maps docker registry credential to env-specific details", func(t *testing.T) {
		body, err := BuildUpdateRequest(dockerWebService, servicetypes.ServiceUpdateInput{
			RegistryCredential: pointers.From("rc-456"),
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPATCH()
		require.NoError(t, err)
		require.NotNil(t, details.EnvSpecificDetails)
		envDetails, err := details.EnvSpecificDetails.AsDockerDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From("rc-456"), envDetails.RegistryCredentialId)
	})

	t.Run("rejects incompatible runtime-specific fields", func(t *testing.T) {
		tests := []struct {
			name        string
			before      client.Service
			input       servicetypes.ServiceUpdateInput
			expectedErr string
		}{
			{
				name:   "registry credential on a static site",
				before: staticSite,
				input: servicetypes.ServiceUpdateInput{
					RegistryCredential: pointers.From("rc-456"),
				},
				expectedErr: registryCredentialUpdateIncompatibleRuntimeError,
			},
			{
				name:   "native commands for docker services",
				before: dockerWebService,
				input: servicetypes.ServiceUpdateInput{
					BuildCommand: pointers.From("npm ci"),
				},
				expectedErr: "--build-command and --start-command are only supported for native runtimes",
			},
			{
				name:   "build command for docker cron jobs",
				before: dockerCronJob,
				input: servicetypes.ServiceUpdateInput{
					BuildCommand: pointers.From("npm ci"),
				},
				expectedErr: "--build-command is only supported for native runtimes",
			},
			{
				name:   "native commands for image runtime services",
				before: imageWebService,
				input: servicetypes.ServiceUpdateInput{
					StartCommand: pointers.From("npm start"),
				},
				expectedErr: "--build-command and --start-command are only supported for native runtimes",
			},
			{
				name:   "registry credential for image runtime services unless image is updated",
				before: imageWebService,
				input: servicetypes.ServiceUpdateInput{
					RegistryCredential: pointers.From("rc-789"),
				},
				expectedErr: registryCredentialUpdateIncompatibleRuntimeError,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := BuildUpdateRequest(tt.before, tt.input)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			})
		}
	})

	t.Run("maps private service paid plan details", func(t *testing.T) {
		body, err := BuildUpdateRequest(nativePrivateService, servicetypes.ServiceUpdateInput{
			Plan: pointers.From("starter"),
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsPrivateServiceDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From(client.PaidPlan("starter")), details.Plan)
	})

	t.Run("maps background worker details", func(t *testing.T) {
		body, err := BuildUpdateRequest(nativeBackgroundWorker, servicetypes.ServiceUpdateInput{
			PreDeployCommand: pointers.From("bin/setup"),
			MaxShutdownDelay: pointers.From(30),
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsBackgroundWorkerDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From("bin/setup"), details.PreDeployCommand)
		assert.Equal(t, pointers.From(client.MaxShutdownDelaySeconds(30)), details.MaxShutdownDelaySeconds)
	})

	t.Run("maps cron job details", func(t *testing.T) {
		body, err := BuildUpdateRequest(nativeCronJob, servicetypes.ServiceUpdateInput{
			BuildCommand: pointers.From("bundle install"),
			CronCommand:  pointers.From("bundle exec rake nightly"),
			Plan:         pointers.From("starter"),
			CronSchedule: pointers.From("0 12 * * *"),
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsCronJobDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From(client.PaidPlan("starter")), details.Plan)
		require.NotNil(t, details.EnvSpecificDetails)
		envDetails, err := details.EnvSpecificDetails.AsNativeEnvironmentDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From("bundle install"), envDetails.BuildCommand)
		assert.Equal(t, pointers.From("bundle exec rake nightly"), envDetails.StartCommand)
		assert.Equal(t, pointers.From("0 12 * * *"), details.Schedule)
	})

	t.Run("maps docker cron job details", func(t *testing.T) {
		body, err := BuildUpdateRequest(dockerCronJob, servicetypes.ServiceUpdateInput{
			CronCommand:        pointers.From("bundle exec rake nightly"),
			RegistryCredential: pointers.From("rc-789"),
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsCronJobDetailsPATCH()
		require.NoError(t, err)
		require.NotNil(t, details.EnvSpecificDetails)
		envDetails, err := details.EnvSpecificDetails.AsDockerDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From("bundle exec rake nightly"), envDetails.DockerCommand)
		assert.Equal(t, pointers.From("rc-789"), envDetails.RegistryCredentialId)
	})

	t.Run("maps static site details", func(t *testing.T) {
		previews := servicetypes.PreviewsGenerationAutomatic
		body, err := BuildUpdateRequest(staticSite, servicetypes.ServiceUpdateInput{
			BuildCommand:     pointers.From("npm run build"),
			PublishDirectory: pointers.From("dist"),
			Previews:         &previews,
			IPAllowList:      []string{"cidr=198.51.100.0/24,description=office"},
		})
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsStaticSiteDetailsPATCH()
		require.NoError(t, err)
		assert.Equal(t, pointers.From("npm run build"), details.BuildCommand)
		assert.Equal(t, pointers.From("dist"), details.PublishPath)
		require.NotNil(t, details.Previews)
		assert.Equal(t, pointers.From(client.PreviewsGenerationAutomatic), details.Previews.Generation)
		require.NotNil(t, details.IpAllowList)
		require.Len(t, *details.IpAllowList, 1)
		assert.Equal(t, "198.51.100.0/24", (*details.IpAllowList)[0].CidrBlock)
	})
}

// serviceBefore builds the minimal pre-update service shape needed for tests
// that only care about service type and runtime.
func serviceBefore(t *testing.T, serviceType client.ServiceType, runtime *client.ServiceRuntime) client.Service {
	t.Helper()

	var details client.Service_ServiceDetails
	switch serviceType {
	case client.WebService:
		require.NotNil(t, runtime)
		require.NoError(t, details.FromWebServiceDetails(client.WebServiceDetails{Runtime: *runtime}))
	case client.PrivateService:
		require.NotNil(t, runtime)
		require.NoError(t, details.FromPrivateServiceDetails(client.PrivateServiceDetails{Runtime: *runtime}))
	case client.BackgroundWorker:
		require.NotNil(t, runtime)
		require.NoError(t, details.FromBackgroundWorkerDetails(client.BackgroundWorkerDetails{Runtime: *runtime}))
	case client.CronJob:
		require.NotNil(t, runtime)
		require.NoError(t, details.FromCronJobDetails(client.CronJobDetails{Runtime: *runtime}))
	case client.StaticSite:
		require.NoError(t, details.FromStaticSiteDetails(client.StaticSiteDetails{}))
	default:
		t.Fatalf("unsupported service type %q", serviceType)
	}

	return client.Service{
		Type:           serviceType,
		ServiceDetails: details,
	}
}

// webServiceBefore builds a pre-update web service with explicit service
// details, used by tests that need existing nested state.
func webServiceBefore(t *testing.T, webDetails client.WebServiceDetails) client.Service {
	t.Helper()

	var details client.Service_ServiceDetails
	require.NoError(t, details.FromWebServiceDetails(webDetails))

	return client.Service{
		Type:           client.WebService,
		ServiceDetails: details,
	}
}
