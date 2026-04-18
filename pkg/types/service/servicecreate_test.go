package service_test

import (
	"testing"

	"github.com/render-oss/cli/v2/pkg/pointers"
	types "github.com/render-oss/cli/v2/pkg/types"
	servicetypes "github.com/render-oss/cli/v2/pkg/types/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceCreateCLIInputValidate(t *testing.T) {
	t.Run("non-interactive requires name", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Type: svcType(servicetypes.ServiceTypeWebService),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("non-interactive requires type when --from is not set", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-service",
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})

	t.Run("interactive allows empty type", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-service",
			Type: svcTypeRaw(""),
		}
		err := input.Validate(true)
		require.NoError(t, err)
	})

	t.Run("cannot specify both repo and image", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:  "my-service",
			Type:  svcType(servicetypes.ServiceTypeWebService),
			Repo:  pointers.From("https://github.com/org/repo"),
			Image: pointers.From("docker.io/image:tag"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot specify both")
	})

	t.Run("invalid type is rejected", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-service",
			Type: svcTypeRaw("invalid_type"),
			Repo: pointers.From("https://github.com/org/repo"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type must be one of")
	})

	t.Run("runtime is required when not image/from", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-service",
			Type: svcType(servicetypes.ServiceTypeWebService),
			Repo: pointers.From("https://github.com/org/repo"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--runtime is required")
	})

	t.Run("static sites do not require runtime", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-static-site",
			Type: svcType(servicetypes.ServiceTypeStaticSite),
			Repo: pointers.From("https://github.com/org/repo"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("valid non-interactive input", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-service",
			Type:         svcType(servicetypes.ServiceTypeWebService),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
			Region:       svcRegion(types.RegionOregon),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects malformed env var", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-service",
			Type:         svcType(servicetypes.ServiceTypeWebService),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
			EnvVars:      []string{"INVALID"},
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --env-var")
	})

	t.Run("rejects malformed secret file", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-service",
			Type:         svcType(servicetypes.ServiceTypeWebService),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
			SecretFiles:  []string{"INVALID_NO_COLON"},
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid --secret-file")
	})

	t.Run("allows registry credential with image source", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:               "my-service",
			Type:               svcType(servicetypes.ServiceTypeWebService),
			Image:              pointers.From("docker.io/org/image:latest"),
			RegistryCredential: pointers.From("my-cred"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("allows registry credential with docker runtime", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:               "my-service",
			Type:               svcType(servicetypes.ServiceTypeWebService),
			Repo:               pointers.From("https://github.com/org/repo"),
			Runtime:            svcRuntime(servicetypes.ServiceRuntimeDocker),
			RegistryCredential: pointers.From("my-cred"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects registry credential for native runtime", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:               "my-service",
			Type:               svcType(servicetypes.ServiceTypeWebService),
			Repo:               pointers.From("https://github.com/org/repo"),
			Runtime:            svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildCommand:       pointers.From("npm ci"),
			StartCommand:       pointers.From("npm run start"),
			RegistryCredential: pointers.From("my-cred"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--registry-credential is only supported with --image or --runtime docker/image")
	})

	t.Run("cron job requires cron-command and cron-schedule when not cloning", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-cron",
			Type:         svcType(servicetypes.ServiceTypeCronJob),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildCommand: pointers.From("npm ci"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cron-command and cron-schedule are required")
	})

	t.Run("cron job with --from skips initial cron-command requirement", func(t *testing.T) {
		// When cloning (--from), the initial cron field check is skipped because
		// the source service will provide those values. This test verifies that
		// using image runtime (which doesn't require build/start commands) passes.
		input := servicetypes.ServiceCreateInput{
			Name:  "my-cron",
			Type:  svcType(servicetypes.ServiceTypeCronJob),
			From:  pointers.From("crn-abc123"),
			Image: pointers.From("docker.io/myimage:latest"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("cron job with image runtime does not require build command", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-cron",
			Type:         svcType(servicetypes.ServiceTypeCronJob),
			Image:        pointers.From("docker.io/myimage:latest"),
			CronCommand:  pointers.From("echo hello"),
			CronSchedule: pointers.From("* * * * *"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("missing repo or image returns clear error before cron validation", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:    "my-cron",
			Type:    svcType(servicetypes.ServiceTypeCronJob),
			Runtime: svcRuntime(servicetypes.ServiceRuntimeNode),
		}
		err := input.Validate(false)
		require.Error(t, err)
		// Should get "repo or image required" not "cron-command required"
		assert.Contains(t, err.Error(), "either repo or image is required")
	})
}

func TestValidateFlagsForType(t *testing.T) {
	t.Run("rejects --cron-command for web service", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:        "my-service",
			Type:        svcType(servicetypes.ServiceTypeWebService),
			Image:       pointers.From("nginx:latest"),
			CronCommand: pointers.From("echo hello"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--cron-command is not supported for web_service")
	})

	t.Run("rejects --health-check-path for cron job", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:            "my-cron",
			Type:            svcType(servicetypes.ServiceTypeCronJob),
			Image:           pointers.From("nginx:latest"),
			CronCommand:     pointers.From("echo hello"),
			CronSchedule:    pointers.From("* * * * *"),
			HealthCheckPath: pointers.From("/health"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--health-check-path is not supported for cron_job")
	})

	t.Run("rejects --publish-directory for web service", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:             "my-service",
			Type:             svcType(servicetypes.ServiceTypeWebService),
			Image:            pointers.From("nginx:latest"),
			PublishDirectory: pointers.From("dist"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--publish-directory is not supported for web_service")
	})

	t.Run("rejects --start-command for static site", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-static",
			Type:         svcType(servicetypes.ServiceTypeStaticSite),
			Repo:         pointers.From("https://github.com/org/repo"),
			StartCommand: pointers.From("npm start"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--start-command is not supported for static_site")
	})

	t.Run("rejects --num-instances for static site", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-static",
			Type:         svcType(servicetypes.ServiceTypeStaticSite),
			Repo:         pointers.From("https://github.com/org/repo"),
			NumInstances: pointers.From(3),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--num-instances is not supported for static_site")
	})

	t.Run("rejects --maintenance-mode for private service", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:            "my-service",
			Type:            svcType(servicetypes.ServiceTypePrivateService),
			Image:           pointers.From("nginx:latest"),
			MaintenanceMode: pointers.From(true),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--maintenance-mode is not supported for private_service")
	})

	t.Run("rejects --ip-allow-list for background worker", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:        "my-worker",
			Type:        svcType(servicetypes.ServiceTypeBackgroundWorker),
			Image:       pointers.From("nginx:latest"),
			IPAllowList: []string{"cidr=10.0.0.0/8"},
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--ip-allow-list is not supported for background_worker")
	})

	t.Run("allows --cron-command for cron job", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-cron",
			Type:         svcType(servicetypes.ServiceTypeCronJob),
			Image:        pointers.From("nginx:latest"),
			CronCommand:  pointers.From("echo hello"),
			CronSchedule: pointers.From("* * * * *"),
		}
		err := input.Validate(false)
		require.NoError(t, err)
	})

	t.Run("rejects --runtime for static site", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:    "my-static",
			Type:    svcType(servicetypes.ServiceTypeStaticSite),
			Repo:    pointers.From("https://github.com/org/repo"),
			Runtime: svcRuntime(servicetypes.ServiceRuntimeNode),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--runtime is not supported for static_site")
	})

	t.Run("rejects --region for static site", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:   "my-static",
			Type:   svcType(servicetypes.ServiceTypeStaticSite),
			Repo:   pointers.From("https://github.com/org/repo"),
			Region: svcRegion(types.RegionOregon),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--region is not supported for static_site")
	})

	t.Run("rejects --plan for static site", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-static",
			Type: svcType(servicetypes.ServiceTypeStaticSite),
			Repo: pointers.From("https://github.com/org/repo"),
			Plan: pointers.From("starter"),
		}
		err := input.Validate(false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--plan is not supported for static_site")
	})

	t.Run("skipped in interactive mode", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:        "my-service",
			Type:        svcType(servicetypes.ServiceTypeWebService),
			CronCommand: pointers.From("echo hello"),
		}
		err := input.Validate(true)
		require.NoError(t, err)
	})
}

func TestNormalizeAndValidateCreateInput(t *testing.T) {
	t.Run("normalizes and validates", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         " my-service ",
			Type:         svcType(servicetypes.ServiceTypeWebService),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm run start"),
		}

		got, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.NoError(t, err)
		require.Equal(t, "my-service", got.Name)
	})

	t.Run("returns validation error", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-service",
			Type: svcType(servicetypes.ServiceTypeWebService),
			Repo: pointers.From("https://github.com/org/repo"),
		}

		_, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--runtime is required")
	})

	t.Run("whitespace-only required pointer fields normalize to nil and fail validation", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-service",
			Type: svcTypeRaw("   "),
			Repo: pointers.From("https://github.com/org/repo"),
		}

		_, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type is required")
	})

	t.Run("whitespace-only repo is treated as unset", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "my-service",
			Type: svcType(servicetypes.ServiceTypeWebService),
			Repo: pointers.From("   "),
		}

		_, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "either repo or image is required")
	})

	t.Run("valid IPAllowList entry passes validation", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name:         "my-service",
			Type:         svcType(servicetypes.ServiceTypeWebService),
			Repo:         pointers.From("https://github.com/org/repo"),
			Runtime:      svcRuntime(servicetypes.ServiceRuntimeNode),
			BuildCommand: pointers.From("npm ci"),
			StartCommand: pointers.From("npm start"),
			IPAllowList:  []string{"cidr=10.0.0.0/8,description=Internal"},
		}
		err := input.Validate(false)
		require.NoError(t, err)
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
