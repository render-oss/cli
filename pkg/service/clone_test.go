package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	servicetypes "github.com/render-oss/cli/pkg/types/service"
)

func TestCLIInputFromSource(t *testing.T) {
	t.Run("hydrates repo-backed defaults", func(t *testing.T) {
		source := &client.Service{
			Type:          client.WebService,
			Repo:          pointers.From("https://github.com/renderinc/api"),
			Branch:        pointers.From("master"),
			RootDir:       "services/api",
			EnvironmentId: pointers.From("evm-123"),
			ServiceDetails: mustWebServiceDetails(
				t,
				client.ServiceRuntimeNode,
				client.NativeEnvironmentDetails{BuildCommand: "npm ci", StartCommand: "npm run start"},
			),
		}
		input := servicetypes.Service{
			Name: "clone-repo",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceType("web_service"), *input.Type)
		require.Equal(t, "https://github.com/renderinc/api", *input.Repo)
		require.Equal(t, "master", *input.Branch)
		require.Equal(t, "services/api", *input.RootDirectory)
		require.Equal(t, "evm-123", *input.EnvironmentID)
		require.Equal(t, servicetypes.ServiceRuntime("node"), *input.Runtime)
	})

	t.Run("hydrates image-backed defaults", func(t *testing.T) {
		source := &client.Service{
			Type:      client.PrivateService,
			ImagePath: pointers.From("docker.io/org/app:latest"),
			RootDir:   ".",
			ServiceDetails: mustPrivateServiceDetails(
				t,
				client.ServiceRuntimeImage,
				client.NativeEnvironmentDetails{BuildCommand: "", StartCommand: "run"},
			),
			RegistryCredential: &client.RegistryCredentialSummary{Id: "rgc-123"},
		}
		input := servicetypes.Service{
			Name: "clone-image",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "docker.io/org/app:latest", *input.Image)
		require.Equal(t, "rgc-123", *input.RegistryCredential)
		require.Equal(t, servicetypes.ServiceRuntime("image"), *input.Runtime)
	})

	t.Run("hydrates docker runtime registry credential from docker details", func(t *testing.T) {
		var envSpecific client.EnvSpecificDetails
		require.NoError(t, envSpecific.FromDockerDetails(client.DockerDetails{
			RegistryCredential: &client.RegistryCredential{
				Id:        "rgc-456",
				Name:      "docker-cred",
				Registry:  client.DOCKER,
				UpdatedAt: time.Time{},
				Username:  "user",
			},
		}))

		var details client.Service_ServiceDetails
		require.NoError(t, details.FromWebServiceDetails(client.WebServiceDetails{
			Runtime:            client.ServiceRuntimeDocker,
			EnvSpecificDetails: envSpecific,
		}))

		source := &client.Service{
			Type:           client.WebService,
			Repo:           pointers.From("https://github.com/renderinc/api"),
			Branch:         pointers.From("master"),
			ServiceDetails: details,
		}
		input := servicetypes.Service{
			Name: "clone-docker-repo",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceRuntime("docker"), *input.Runtime)
		require.Equal(t, "rgc-456", *input.RegistryCredential)
	})

	t.Run("hydrates background worker defaults", func(t *testing.T) {
		source := &client.Service{
			Type:          client.BackgroundWorker,
			Repo:          pointers.From("https://github.com/renderinc/worker"),
			Branch:        pointers.From("main"),
			RootDir:       "workers/processor",
			EnvironmentId: pointers.From("evm-456"),
			ServiceDetails: mustBackgroundWorkerDetails(
				t,
				client.ServiceRuntimePython,
				client.NativeEnvironmentDetails{BuildCommand: "pip install -r requirements.txt", StartCommand: "python worker.py"},
			),
		}
		input := servicetypes.Service{
			Name: "clone-worker",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceType("background_worker"), *input.Type)
		require.Equal(t, "https://github.com/renderinc/worker", *input.Repo)
		require.Equal(t, "main", *input.Branch)
		require.Equal(t, "workers/processor", *input.RootDirectory)
		require.Equal(t, "evm-456", *input.EnvironmentID)
		require.Equal(t, servicetypes.ServiceRuntime("python"), *input.Runtime)
	})

	t.Run("hydrates static site defaults", func(t *testing.T) {
		source := &client.Service{
			Type:          client.StaticSite,
			Repo:          pointers.From("https://github.com/renderinc/docs"),
			Branch:        pointers.From("main"),
			RootDir:       "website",
			EnvironmentId: pointers.From("evm-789"),
			ServiceDetails: mustStaticSiteDetails(
				t,
				"npm run build",
				"dist",
			),
		}
		input := servicetypes.Service{
			Name: "clone-static",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceType("static_site"), *input.Type)
		require.Equal(t, "https://github.com/renderinc/docs", *input.Repo)
		require.Equal(t, "main", *input.Branch)
		require.Equal(t, "website", *input.RootDirectory)
		require.Equal(t, "evm-789", *input.EnvironmentID)
		// Static sites don't have a runtime field
		require.Nil(t, input.Runtime)
	})

	t.Run("hydrates cron schedule and command", func(t *testing.T) {
		source := &client.Service{
			Type:           client.CronJob,
			ServiceDetails: mustCronJobDetails(t, "*/5 * * * *", "echo hello"),
		}
		input := servicetypes.Service{
			Name: "clone-cron",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "*/5 * * * *", *input.CronSchedule)
		require.Equal(t, "echo hello", *input.CronCommand)
	})

	t.Run("does not override explicit fields", func(t *testing.T) {
		source := &client.Service{
			Type:           client.WebService,
			Repo:           pointers.From("https://github.com/renderinc/api"),
			Branch:         pointers.From("master"),
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode, client.NativeEnvironmentDetails{}),
		}
		input := servicetypes.Service{
			Name:    "clone-explicit",
			From:    pointers.From("srv-source"),
			Type:    svcTypeRaw("private_service"),
			Repo:    pointers.From("https://github.com/org/custom"),
			Branch:  pointers.From("feature-x"),
			Runtime: svcRuntime(servicetypes.ServiceRuntimeDocker),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceType("private_service"), *input.Type)
		require.Equal(t, "https://github.com/org/custom", *input.Repo)
		require.Equal(t, "feature-x", *input.Branch)
		require.Equal(t, servicetypes.ServiceRuntime("docker"), *input.Runtime)
	})

	t.Run("does not copy repo defaults when image is explicitly provided", func(t *testing.T) {
		source := &client.Service{
			Type:           client.WebService,
			Repo:           pointers.From("https://github.com/renderinc/api"),
			Branch:         pointers.From("master"),
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode, client.NativeEnvironmentDetails{}),
		}
		input := servicetypes.Service{
			Name:  "clone-explicit-image",
			From:  pointers.From("srv-source"),
			Image: pointers.From("docker.io/custom/image:latest"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "docker.io/custom/image:latest", *input.Image)
		require.Nil(t, input.Repo)
		require.Nil(t, input.Branch)
		require.Equal(t, servicetypes.ServiceRuntime("image"), *input.Runtime)
	})

	t.Run("does not copy image defaults when repo is explicitly provided", func(t *testing.T) {
		source := &client.Service{
			Type:      client.PrivateService,
			ImagePath: pointers.From("docker.io/org/app:latest"),
			ServiceDetails: mustPrivateServiceDetails(
				t,
				client.ServiceRuntimeImage,
				client.NativeEnvironmentDetails{},
			),
			RegistryCredential: &client.RegistryCredentialSummary{Id: "rgc-123"},
		}
		input := servicetypes.Service{
			Name: "clone-explicit-repo",
			From: pointers.From("srv-source"),
			Repo: pointers.From("https://github.com/org/custom"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "https://github.com/org/custom", *input.Repo)
		require.Nil(t, input.Image)
		require.Nil(t, input.RegistryCredential)
		require.Nil(t, input.Runtime)
	})

	t.Run("from image override defaults runtime to image and builds request", func(t *testing.T) {
		source := &client.Service{
			Type:           client.WebService,
			Repo:           pointers.From("https://github.com/renderinc/api"),
			Branch:         pointers.From("master"),
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode, client.NativeEnvironmentDetails{}),
		}
		input := servicetypes.Service{
			Name:  "clone-explicit-image",
			From:  pointers.From("srv-source"),
			Image: pointers.From("docker.io/custom/image:latest"),
		}

		ServiceFromAPI(&input, source)
		body, err := BuildCreateRequest(input, "tea-abc123")

		require.NoError(t, err)
		details, err := body.ServiceDetails.AsWebServiceDetailsPOST()
		require.NoError(t, err)
		require.Equal(t, client.ServiceRuntimeImage, details.Runtime)
	})

	t.Run("from repo override clears incompatible image runtime and fails build without explicit runtime", func(t *testing.T) {
		source := &client.Service{
			Type:      client.PrivateService,
			ImagePath: pointers.From("docker.io/org/app:latest"),
			ServiceDetails: mustPrivateServiceDetails(
				t,
				client.ServiceRuntimeImage,
				client.NativeEnvironmentDetails{},
			),
		}
		input := servicetypes.Service{
			Name: "clone-explicit-repo",
			From: pointers.From("srv-source"),
			Repo: pointers.From("https://github.com/org/custom"),
		}

		ServiceFromAPI(&input, source)
		_, err := BuildCreateRequest(input, "tea-abc123")

		require.Error(t, err)
		require.Contains(t, err.Error(), "runtime is required")
	})

	t.Run("applies normalized defaults from source values", func(t *testing.T) {
		source := &client.Service{
			Type:          client.WebService,
			Repo:          pointers.From("  https://github.com/renderinc/api  "),
			Branch:        pointers.From("  master  "),
			RootDir:       "  services/api  ",
			EnvironmentId: pointers.From("  evm-123  "),
			ServiceDetails: mustWebServiceDetails(
				t,
				client.ServiceRuntimeNode,
				client.NativeEnvironmentDetails{},
			),
		}
		input := servicetypes.NormalizeServiceCreateCLIInput(servicetypes.Service{
			Name: "clone-normalized",
			From: pointers.From("srv-source"),
		})

		ServiceFromAPI(&input, source)
		input = servicetypes.NormalizeServiceCreateCLIInput(input)

		require.Equal(t, "https://github.com/renderinc/api", *input.Repo)
		require.Equal(t, "master", *input.Branch)
		require.Equal(t, "services/api", *input.RootDirectory)
		require.Equal(t, "evm-123", *input.EnvironmentID)
	})

	t.Run("skips empty source defaults", func(t *testing.T) {
		source := &client.Service{
			Type:          client.WebService,
			Repo:          pointers.From("   "),
			Branch:        pointers.From(""),
			RootDir:       "",
			EnvironmentId: pointers.From("  "),
			ServiceDetails: mustWebServiceDetails(
				t,
				client.ServiceRuntimeNode,
				client.NativeEnvironmentDetails{},
			),
		}
		input := servicetypes.NormalizeServiceCreateCLIInput(servicetypes.Service{
			Name: "clone-empty-defaults",
			From: pointers.From("srv-source"),
		})

		ServiceFromAPI(&input, source)
		input = servicetypes.NormalizeServiceCreateCLIInput(input)

		require.Nil(t, input.Repo)
		require.Nil(t, input.Branch)
		require.Nil(t, input.RootDirectory)
		require.Nil(t, input.EnvironmentID)
	})
}

func mustWebServiceDetails(t *testing.T, runtime client.ServiceRuntime, native client.NativeEnvironmentDetails) client.Service_ServiceDetails {
	t.Helper()

	var envSpecific client.EnvSpecificDetails
	require.NoError(t, envSpecific.FromNativeEnvironmentDetails(native))

	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromWebServiceDetails(client.WebServiceDetails{
		Runtime:            runtime,
		EnvSpecificDetails: envSpecific,
	}))

	return serviceDetails
}

func mustPrivateServiceDetails(t *testing.T, runtime client.ServiceRuntime, native client.NativeEnvironmentDetails) client.Service_ServiceDetails {
	t.Helper()

	var envSpecific client.EnvSpecificDetails
	require.NoError(t, envSpecific.FromNativeEnvironmentDetails(native))

	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromPrivateServiceDetails(client.PrivateServiceDetails{
		Runtime:            runtime,
		EnvSpecificDetails: envSpecific,
	}))

	return serviceDetails
}

func mustBackgroundWorkerDetails(t *testing.T, runtime client.ServiceRuntime, native client.NativeEnvironmentDetails) client.Service_ServiceDetails {
	t.Helper()

	var envSpecific client.EnvSpecificDetails
	require.NoError(t, envSpecific.FromNativeEnvironmentDetails(native))

	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromBackgroundWorkerDetails(client.BackgroundWorkerDetails{
		Runtime:            runtime,
		EnvSpecificDetails: envSpecific,
	}))

	return serviceDetails
}

func mustStaticSiteDetails(t *testing.T, buildCommand string, publishPath string) client.Service_ServiceDetails {
	t.Helper()

	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromStaticSiteDetails(client.StaticSiteDetails{
		BuildCommand: buildCommand,
		PublishPath:  publishPath,
	}))

	return serviceDetails
}

func mustCronJobDetails(t *testing.T, schedule string, command string) client.Service_ServiceDetails {
	t.Helper()

	var envSpecific client.EnvSpecificDetails
	require.NoError(t, envSpecific.FromNativeEnvironmentDetails(client.NativeEnvironmentDetails{
		BuildCommand: "",
		StartCommand: command,
	}))

	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromCronJobDetails(client.CronJobDetails{
		Runtime:            client.ServiceRuntimeDocker,
		Schedule:           schedule,
		EnvSpecificDetails: envSpecific,
	}))

	return serviceDetails
}
