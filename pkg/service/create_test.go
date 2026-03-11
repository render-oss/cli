package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	types "github.com/render-oss/cli/pkg/types"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCreateRequest_FromCLIInput(t *testing.T) {
	input := servicetypes.ServiceCreateInput{
		Name:         "my-service",
		Type:         svcType(servicetypes.ServiceTypeWebService),
		Repo:         pointers.From("https://github.com/org/repo"),
		Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
		Region:       svcRegion(types.RegionOregon),
		BuildCommand: pointers.From("npm ci"),
		StartCommand: pointers.From("npm run start"),
	}

	parsed, err := BuildCreateRequest(input, "tea-abc123")
	require.NoError(t, err)
	require.NotNil(t, parsed.ServiceDetails)
	require.Nil(t, parsed.EnvVars)

	details, err := parsed.ServiceDetails.AsWebServiceDetailsPOST()
	require.NoError(t, err)
	require.Equal(t, client.ServiceRuntimeNode, details.Runtime)
	require.Equal(t, client.Region("oregon"), *details.Region)
}

func TestBuildCreateRequest_FromCLIInput_DefaultRuntimeAndEnvVars(t *testing.T) {
	input := servicetypes.ServiceCreateInput{
		Name:    "my-image-service",
		Type:    svcType(servicetypes.ServiceTypeWebService),
		Image:   pointers.From("docker.io/org/app:latest"),
		EnvVars: []string{"FOO=bar"},
	}

	parsed, err := BuildCreateRequest(input, "tea-abc123")
	require.NoError(t, err)
	details, err := parsed.ServiceDetails.AsWebServiceDetailsPOST()
	require.NoError(t, err)
	require.Equal(t, client.ServiceRuntimeImage, details.Runtime)
	require.Len(t, *parsed.EnvVars, 1)
}

func TestBuildCreateRequest_FromCLIInput_WithNormalizedInputOmitsEmptyOptionals(t *testing.T) {
	empty := ""
	input := servicetypes.ServiceCreateInput{
		Name:               "my-service",
		Type:               svcType(servicetypes.ServiceTypeWebService),
		Repo:               pointers.From("https://github.com/org/repo"),
		Branch:             &empty,
		Image:              &empty,
		Plan:               &empty,
		Runtime:            svcRuntime(servicetypes.ServiceRuntimeNode),
		RootDirectory:      &empty,
		BuildCommand:       &empty,
		StartCommand:       &empty,
		HealthCheckPath:    &empty,
		PublishDirectory:   &empty,
		CronCommand:        &empty,
		CronSchedule:       &empty,
		EnvironmentID:      &empty,
		RegistryCredential: &empty,
		PreDeployCommand:   &empty,
	}

	normalized := servicetypes.NormalizeServiceCreateCLIInput(input)
	parsed, err := BuildCreateRequest(normalized, "tea-abc123")
	require.NoError(t, err)
	require.Equal(t, "https://github.com/org/repo", *parsed.Repo)
	require.Nil(t, parsed.Branch)
	require.Nil(t, parsed.Image)
	require.Nil(t, parsed.RootDir)
	require.Nil(t, parsed.EnvironmentId)
}

func TestBuildCreateRequest_FromCLIInput_ParsesSecretFiles(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "config.txt")
	require.NoError(t, os.WriteFile(secretPath, []byte("top-secret"), 0o600))

	input := servicetypes.ServiceCreateInput{
		Name:        "my-service",
		Type:        svcType(servicetypes.ServiceTypeWebService),
		Image:       pointers.From("docker.io/org/app:latest"),
		SecretFiles: []string{"app-secret:" + secretPath},
	}

	parsed, err := BuildCreateRequest(input, "tea-abc123")
	require.NoError(t, err)
	require.Len(t, *parsed.SecretFiles, 1)
	require.Equal(t, "app-secret", (*parsed.SecretFiles)[0].Name)
	require.Equal(t, "top-secret", (*parsed.SecretFiles)[0].Content)
}

func TestBuildCreateRequest_FromCLIInput_SecretFileReadError(t *testing.T) {
	input := servicetypes.ServiceCreateInput{
		Name: "my-service",

		Type:        svcType(servicetypes.ServiceTypeWebService),
		Image:       pointers.From("docker.io/org/app:latest"),
		SecretFiles: []string{"app-secret:/definitely/missing"},
	}

	_, err := BuildCreateRequest(input, "tea-abc123")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read --secret-file")
}

func TestBuildCreateRequest(t *testing.T) {
	t.Run("non-static service includes serviceDetails", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:    "my-service",
			Type:    svcType(servicetypes.ServiceTypeWebService),
			Image:   pointers.From("nginx:latest"),
			Runtime: svcRuntime(servicetypes.ServiceRuntimeImage),
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPOST()
		require.NoError(t, err)
		assert.Equal(t, client.ServiceRuntimeImage, details.Runtime)
	})

	t.Run("static sites do not require runtime", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:             "my-static-site",
			Type:             svcType(servicetypes.ServiceTypeStaticSite),
			Repo:             pointers.From("https://github.com/org/site"),
			BuildCommand:     pointers.From("npm run build"),
			PublishDirectory: pointers.From("dist"),
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsStaticSiteDetailsPOST()
		require.NoError(t, err)
		require.Equal(t, "npm run build", pointers.ValueOrDefault(details.BuildCommand, ""))
		require.Equal(t, "dist", pointers.ValueOrDefault(details.PublishPath, ""))
	})

	t.Run("explicit runtime is applied to serviceDetails", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:    "my-service",
			Type:    svcType(servicetypes.ServiceTypeWebService),
			Image:   pointers.From("nginx:latest"),
			Runtime: svcRuntime(servicetypes.ServiceRuntimeNode),
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPOST()
		require.NoError(t, err)
		assert.Equal(t, client.ServiceRuntimeNode, details.Runtime)
	})

	t.Run("web service maps build and start commands to native env specific details", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-service",
			Type:         svcType(servicetypes.ServiceTypeWebService),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPOST()
		require.NoError(t, err)
		require.NotNil(t, details.EnvSpecificDetails)

		native, err := details.EnvSpecificDetails.AsNativeEnvironmentDetailsPOST()
		require.NoError(t, err)
		assert.Equal(t, "npm ci", native.BuildCommand)
		assert.Equal(t, "npm run start", native.StartCommand)
	})

	t.Run("web service with docker runtime maps registry credential to docker details", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:               "my-service",
			Type:               svcType(servicetypes.ServiceTypeWebService),
			Repo:               pointers.From("https://github.com/org/repo"),
			Runtime:            svcRuntime(servicetypes.ServiceRuntimeDocker),
			RegistryCredential: pointers.From("rgc-123"),
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsWebServiceDetailsPOST()
		require.NoError(t, err)
		require.NotNil(t, details.EnvSpecificDetails)

		dockerDetails, err := details.EnvSpecificDetails.AsDockerDetailsPOST()
		require.NoError(t, err)
		require.Equal(t, "rgc-123", pointers.ValueOrDefault(dockerDetails.RegistryCredentialId, ""))
	})

	t.Run("cron job maps cron-command to native env specific start command", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-cron",
			Type:         svcType(servicetypes.ServiceTypeCronJob),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeRuby),
			CronCommand:  pointers.From("echo hello"),
			CronSchedule: pointers.From("*/5 * * * *"),
			BuildCommand: pointers.From("npm ci"),
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)
		require.NotNil(t, body.ServiceDetails)

		details, err := body.ServiceDetails.AsCronJobDetailsPOST()
		require.NoError(t, err)
		require.NotNil(t, details.EnvSpecificDetails)

		native, err := details.EnvSpecificDetails.AsNativeEnvironmentDetails()
		require.NoError(t, err)
		assert.Equal(t, "npm ci", native.BuildCommand)
		assert.Equal(t, "echo hello", native.StartCommand)
	})

	t.Run("empty optional fields are omitted", func(t *testing.T) {
		cliInput := servicetypes.ServiceCreateInput{
			Name:          "my-service",
			Type:          svcType(servicetypes.ServiceTypeWebService),
			Repo:          pointers.From("https://github.com/org/repo"),
			Runtime:       svcRuntime(servicetypes.ServiceRuntimeNode),
			Branch:        pointers.From(""),
			Image:         pointers.From(""),
			EnvironmentID: pointers.From(""),
			RootDirectory: pointers.From(""),
		}
		normalized := servicetypes.NormalizeServiceCreateCLIInput(cliInput)
		body, err := BuildCreateRequest(normalized, "tea-abc123")
		require.NoError(t, err)
		require.Nil(t, body.Branch)
		require.Nil(t, body.Image)
		require.Nil(t, body.EnvironmentId)
		require.Nil(t, body.RootDir)
	})

	t.Run("returns error for unsupported service type", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:    "my-service",
			Type:    svcTypeRaw("unsupported"),
			Repo:    pointers.From("https://github.com/org/repo"),
			Runtime: svcRuntime(servicetypes.ServiceRuntimeNode),
		}

		_, err := BuildCreateRequest(input, "tea-abc123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be one of")
	})

	t.Run("maps extended web fields and build filter", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:                    "my-service",
			Type:                    svcType(servicetypes.ServiceTypeWebService),
			Repo:                    pointers.From("https://github.com/org/repo"),
			Runtime:                 svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildFilterPaths:        []string{"src/", "lib/"},
			BuildFilterIgnoredPaths: []string{"test/"},
			NumInstances:            pointers.From(3),
			MaxShutdownDelay:        pointers.From(45),
			Previews:                pointers.From(servicetypes.PreviewsGenerationAutomatic),
			MaintenanceMode:         pointers.From(true),
			MaintenanceModeURI:      pointers.From("/maintenance.html"),
			IPAllowList:             []string{"cidr=10.0.0.0/8,description=Internal"},
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)
		require.NotNil(t, body.BuildFilter)
		require.Equal(t, []string{"src/", "lib/"}, body.BuildFilter.Paths)
		require.Equal(t, []string{"test/"}, body.BuildFilter.IgnoredPaths)

		details, err := body.ServiceDetails.AsWebServiceDetailsPOST()
		require.NoError(t, err)
		require.Equal(t, 3, pointers.ValueOrDefault(details.NumInstances, 0))
		require.Equal(t, 45, pointers.ValueOrDefault(details.MaxShutdownDelaySeconds, 0))
		require.NotNil(t, details.Previews)
		require.Equal(t, client.PreviewsGeneration("automatic"), *details.Previews.Generation)
		require.NotNil(t, details.MaintenanceMode)
		require.Equal(t, true, details.MaintenanceMode.Enabled)
		require.Equal(t, "/maintenance.html", details.MaintenanceMode.Uri)
		require.NotNil(t, details.IpAllowList)
		require.Len(t, *details.IpAllowList, 1)
		require.Equal(t, "10.0.0.0/8", (*details.IpAllowList)[0].CidrBlock)
		require.Equal(t, "Internal", (*details.IpAllowList)[0].Description)
	})

	t.Run("maps extended private service fields", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:             "my-private-service",
			Type:             svcType(servicetypes.ServiceTypePrivateService),
			Repo:             pointers.From("https://github.com/org/repo"),
			Runtime:          svcRuntime(servicetypes.ServiceRuntimeNode),
			NumInstances:     pointers.From(2),
			MaxShutdownDelay: pointers.From(20),
			Previews:         pointers.From(servicetypes.PreviewsGenerationManual),
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)

		details, err := body.ServiceDetails.AsPrivateServiceDetailsPOST()
		require.NoError(t, err)
		require.Equal(t, 2, pointers.ValueOrDefault(details.NumInstances, 0))
		require.Equal(t, 20, pointers.ValueOrDefault(details.MaxShutdownDelaySeconds, 0))
		require.NotNil(t, details.Previews)
		require.Equal(t, client.PreviewsGeneration("manual"), *details.Previews.Generation)
	})

	t.Run("maps static site previews and ip allow list", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:             "my-static-site",
			Type:             svcType(servicetypes.ServiceTypeStaticSite),
			Repo:             pointers.From("https://github.com/org/site"),
			BuildCommand:     pointers.From("npm run build"),
			PublishDirectory: pointers.From("dist"),
			Previews:         pointers.From(servicetypes.PreviewsGenerationOff),
			IPAllowList:      []string{"cidr=2001:db8::/32,description=IPv6 range"},
		}

		body, err := BuildCreateRequest(input, "tea-abc123")
		require.NoError(t, err)

		details, err := body.ServiceDetails.AsStaticSiteDetailsPOST()
		require.NoError(t, err)
		require.NotNil(t, details.Previews)
		require.Equal(t, client.PreviewsGeneration("off"), *details.Previews.Generation)
		require.NotNil(t, details.IpAllowList)
		require.Len(t, *details.IpAllowList, 1)
		require.Equal(t, "2001:db8::/32", (*details.IpAllowList)[0].CidrBlock)
		require.Equal(t, "IPv6 range", (*details.IpAllowList)[0].Description)
	})

	t.Run("returns error for invalid ip allow list format", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:        "my-service",
			Type:        svcType(servicetypes.ServiceTypeWebService),
			Repo:        pointers.From("https://github.com/org/repo"),
			Runtime:     svcRuntime(servicetypes.ServiceRuntimeNode),
			IPAllowList: []string{"malformed"},
		}

		_, err := BuildCreateRequest(input, "tea-abc123")
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid --ip-allow-list")
	})
}

func svcType(value servicetypes.ServiceType) *servicetypes.ServiceType {
	v := value
	return &v
}

func svcTypeRaw(value string) *servicetypes.ServiceType {
	v := servicetypes.ServiceType(value)
	return &v
}

func svcRuntime(value servicetypes.ServiceRuntime) *servicetypes.ServiceRuntime {
	v := value
	return &v
}

func svcRegion(value types.Region) *types.Region {
	v := value
	return &v
}
