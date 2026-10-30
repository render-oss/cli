package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
	types "github.com/render-oss/cli/pkg/types"
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
		input := servicetypes.ServiceCreateInput{
			Name: "clone-repo",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceTypeWebService, *input.Type)
		require.Equal(t, "https://github.com/renderinc/api", *input.Repo)
		require.Equal(t, "master", *input.Branch)
		require.Equal(t, "services/api", *input.RootDirectory)
		require.Equal(t, "evm-123", *input.EnvironmentID)
		require.Equal(t, servicetypes.ServiceRuntimeNode, *input.Runtime)
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
		input := servicetypes.ServiceCreateInput{
			Name: "clone-image",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "docker.io/org/app:latest", *input.Image)
		require.Equal(t, "rgc-123", *input.RegistryCredential)
		require.Equal(t, servicetypes.ServiceRuntimeImage, *input.Runtime)
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
		input := servicetypes.ServiceCreateInput{
			Name: "clone-docker-repo",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceRuntimeDocker, *input.Runtime)
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
		input := servicetypes.ServiceCreateInput{
			Name: "clone-worker",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceTypeBackgroundWorker, *input.Type)
		require.Equal(t, "https://github.com/renderinc/worker", *input.Repo)
		require.Equal(t, "main", *input.Branch)
		require.Equal(t, "workers/processor", *input.RootDirectory)
		require.Equal(t, "evm-456", *input.EnvironmentID)
		require.Equal(t, servicetypes.ServiceRuntimePython, *input.Runtime)
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
		input := servicetypes.ServiceCreateInput{
			Name: "clone-static",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceTypeStaticSite, *input.Type)
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
		input := servicetypes.ServiceCreateInput{
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
		input := servicetypes.ServiceCreateInput{
			Name:    "clone-explicit",
			From:    pointers.From("srv-source"),
			Type:    svcType(servicetypes.ServiceTypePrivateService),
			Repo:    pointers.From("https://github.com/org/custom"),
			Branch:  pointers.From("feature-x"),
			Runtime: svcRuntime(servicetypes.ServiceRuntimeDocker),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.ServiceTypePrivateService, *input.Type)
		require.Equal(t, "https://github.com/org/custom", *input.Repo)
		require.Equal(t, "feature-x", *input.Branch)
		require.Equal(t, servicetypes.ServiceRuntimeDocker, *input.Runtime)
	})

	t.Run("does not copy repo defaults when image is explicitly provided", func(t *testing.T) {
		source := &client.Service{
			Type:           client.WebService,
			Repo:           pointers.From("https://github.com/renderinc/api"),
			Branch:         pointers.From("master"),
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode, client.NativeEnvironmentDetails{}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:  "clone-explicit-image",
			From:  pointers.From("srv-source"),
			Image: pointers.From("docker.io/custom/image:latest"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "docker.io/custom/image:latest", *input.Image)
		require.Nil(t, input.Repo)
		require.Nil(t, input.Branch)
		require.Equal(t, servicetypes.ServiceRuntimeImage, *input.Runtime)
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
		input := servicetypes.ServiceCreateInput{
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
			Type:   client.WebService,
			Repo:   pointers.From("https://github.com/renderinc/api"),
			Branch: pointers.From("master"),
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode,
				client.NativeEnvironmentDetails{BuildCommand: "npm ci", StartCommand: "npm start"}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:  "clone-explicit-image",
			From:  pointers.From("srv-source"),
			Image: pointers.From("docker.io/custom/image:latest"),
		}

		input, body, err := cloneCreateRequest(t, input, source)

		require.NoError(t, err)
		// The source's native build and start commands do not apply to an image.
		require.Nil(t, input.BuildCommand)
		require.Nil(t, input.StartCommand)
		details, err := body.ServiceDetails.AsWebServiceDetailsPOST()
		require.NoError(t, err)
		require.Equal(t, client.ServiceRuntimeImage, details.Runtime)
		require.Nil(t, details.EnvSpecificDetails)
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
		input := servicetypes.ServiceCreateInput{
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
		input := servicetypes.NormalizeServiceCreateCLIInput(servicetypes.ServiceCreateInput{
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

	t.Run("hydrates web service with all new fields", func(t *testing.T) {
		gen := client.PreviewsGeneration("automatic")
		preDeployCommand := "npm run db:migrate"
		source := &client.Service{
			AutoDeploy:    client.AutoDeployNo,
			Type:          client.WebService,
			Repo:          pointers.From("https://github.com/renderinc/api"),
			Branch:        pointers.From("main"),
			RootDir:       ".",
			EnvironmentId: pointers.From("evm-123"),
			BuildFilter: &client.BuildFilter{
				Paths:        []string{"src/", "lib/"},
				IgnoredPaths: []string{"test/", "docs/"},
			},
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime:                 client.ServiceRuntimeNode,
				Region:                  client.Virginia,
				Plan:                    client.PlanStarter,
				HealthCheckPath:         "/healthz",
				EnvSpecificDetails:      mustEnvSpecific(t, client.NativeEnvironmentDetails{BuildCommand: "npm ci", StartCommand: "npm start", PreDeployCommand: &preDeployCommand}),
				NumInstances:            3,
				MaxShutdownDelaySeconds: pointers.From(30),
				Previews:                &client.Previews{Generation: &gen},
				MaintenanceMode:         &client.MaintenanceMode{Enabled: true, Uri: "/maintenance.html"},
				IpAllowList: &[]client.CidrBlockAndDescription{
					{CidrBlock: "10.0.0.0/8", Description: "Internal"},
					{CidrBlock: "192.168.1.0/24", Description: ""},
				},
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name: "clone-web-extended",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, []string{"src/", "lib/"}, input.BuildFilterPaths)
		require.Equal(t, []string{"test/", "docs/"}, input.BuildFilterIgnoredPaths)
		require.Equal(t, types.RegionVirginia, *input.Region)
		require.Equal(t, "starter", *input.Plan)
		require.Equal(t, "npm ci", *input.BuildCommand)
		require.Equal(t, "npm start", *input.StartCommand)
		require.Equal(t, "npm run db:migrate", *input.PreDeployCommand)
		require.Equal(t, "/healthz", *input.HealthCheckPath)
		require.Equal(t, false, *input.AutoDeploy)
		require.Equal(t, 3, *input.NumInstances)
		require.Equal(t, 30, *input.MaxShutdownDelay)
		require.Equal(t, servicetypes.PreviewsGenerationAutomatic, *input.Previews)
		require.Equal(t, true, *input.MaintenanceMode)
		require.Equal(t, "/maintenance.html", *input.MaintenanceModeURI)
		require.Equal(t, []string{"cidr=10.0.0.0/8,description=Internal", "cidr=192.168.1.0/24"}, input.IPAllowList)
	})

	t.Run("hydrates static site with previews and ip allow list only", func(t *testing.T) {
		gen := client.PreviewsGeneration("manual")
		source := &client.Service{
			Type:    client.StaticSite,
			Repo:    pointers.From("https://github.com/renderinc/docs"),
			Branch:  pointers.From("main"),
			RootDir: "website",
			ServiceDetails: mustStaticSiteDetailsExtended(t, client.StaticSiteDetails{
				BuildCommand: "npm run build",
				PublishPath:  "dist",
				Previews:     &client.Previews{Generation: &gen},
				IpAllowList: &[]client.CidrBlockAndDescription{
					{CidrBlock: "2001:db8::/32", Description: "IPv6 range"},
				},
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name: "clone-static-extended",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "npm run build", *input.BuildCommand)
		require.Equal(t, "dist", *input.PublishDirectory)
		require.Equal(t, servicetypes.PreviewsGenerationManual, *input.Previews)
		require.Equal(t, []string{"cidr=2001:db8::/32,description=IPv6 range"}, input.IPAllowList)
		require.Nil(t, input.NumInstances)
		require.Nil(t, input.MaxShutdownDelay)
		require.Nil(t, input.MaintenanceMode)
		require.Nil(t, input.MaintenanceModeURI)
	})

	t.Run("hydrates background worker with numInstances, maxShutdownDelay, previews", func(t *testing.T) {
		gen := client.PreviewsGeneration("off")
		preDeployCommand := "python migrate.py"
		source := &client.Service{
			AutoDeploy: client.AutoDeployYes,
			Type:       client.BackgroundWorker,
			Repo:       pointers.From("https://github.com/renderinc/worker"),
			Branch:     pointers.From("main"),
			RootDir:    ".",
			ServiceDetails: mustBackgroundWorkerDetailsExtended(t, client.BackgroundWorkerDetails{
				Runtime:                 client.ServiceRuntimePython,
				Region:                  client.Ohio,
				Plan:                    client.PlanPro,
				EnvSpecificDetails:      mustEnvSpecific(t, client.NativeEnvironmentDetails{BuildCommand: "pip install", StartCommand: "python worker.py", PreDeployCommand: &preDeployCommand}),
				NumInstances:            2,
				MaxShutdownDelaySeconds: pointers.From(15),
				Previews:                &client.Previews{Generation: &gen},
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name: "clone-worker-extended",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, types.RegionOhio, *input.Region)
		require.Equal(t, "pro", *input.Plan)
		require.Equal(t, "pip install", *input.BuildCommand)
		require.Equal(t, "python worker.py", *input.StartCommand)
		require.Equal(t, "python migrate.py", *input.PreDeployCommand)
		require.Equal(t, true, *input.AutoDeploy)
		require.Equal(t, 2, *input.NumInstances)
		require.Equal(t, 15, *input.MaxShutdownDelay)
		require.Equal(t, servicetypes.PreviewsGenerationOff, *input.Previews)
		require.Nil(t, input.MaintenanceMode)
		require.Nil(t, input.MaintenanceModeURI)
		require.Empty(t, input.IPAllowList)
	})

	t.Run("hydrates cron defaults and skips non-cron type-specific fields", func(t *testing.T) {
		var envSpecific client.EnvSpecificDetails
		require.NoError(t, envSpecific.FromNativeEnvironmentDetails(client.NativeEnvironmentDetails{
			BuildCommand: "go build ./cmd/cron",
			StartCommand: "./cron run",
		}))

		var serviceDetails client.Service_ServiceDetails
		require.NoError(t, serviceDetails.FromCronJobDetails(client.CronJobDetails{
			Runtime:            client.ServiceRuntimeGo,
			Region:             client.Oregon,
			Plan:               client.PlanStarter,
			Schedule:           "*/5 * * * *",
			EnvSpecificDetails: envSpecific,
		}))

		source := &client.Service{
			Type:           client.CronJob,
			ServiceDetails: serviceDetails,
		}
		input := servicetypes.ServiceCreateInput{
			Name: "clone-cron-extended",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, types.RegionOregon, *input.Region)
		require.Equal(t, "starter", *input.Plan)
		require.Equal(t, "go build ./cmd/cron", *input.BuildCommand)
		require.Equal(t, "./cron run", *input.CronCommand)
		require.Nil(t, input.NumInstances)
		require.Nil(t, input.MaxShutdownDelay)
		require.Nil(t, input.Previews)
		require.Nil(t, input.MaintenanceMode)
		require.Nil(t, input.MaintenanceModeURI)
		require.Empty(t, input.IPAllowList)
		require.Empty(t, input.BuildFilterPaths)
		require.Empty(t, input.BuildFilterIgnoredPaths)
	})

	t.Run("nil build filter leaves build filter fields empty", func(t *testing.T) {
		source := &client.Service{
			Type:           client.WebService,
			Repo:           pointers.From("https://github.com/renderinc/api"),
			Branch:         pointers.From("main"),
			RootDir:        ".",
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode, client.NativeEnvironmentDetails{}),
		}
		input := servicetypes.ServiceCreateInput{
			Name: "clone-no-build-filter",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Empty(t, input.BuildFilterPaths)
		require.Empty(t, input.BuildFilterIgnoredPaths)
	})

	t.Run("web service with maintenance mode disabled", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime:            client.ServiceRuntimeNode,
				EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{}),
				MaintenanceMode:    &client.MaintenanceMode{Enabled: false, Uri: ""},
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name: "clone-maint-disabled",
			From: pointers.From("srv-source"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, false, *input.MaintenanceMode)
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
		input := servicetypes.NormalizeServiceCreateCLIInput(servicetypes.ServiceCreateInput{
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

	t.Run("preserves pre-populated NumInstances override when source has different value", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime:      client.ServiceRuntimeNode,
				NumInstances: 5,
				EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{
					BuildCommand: "npm ci",
					StartCommand: "npm start",
				}),
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:         "clone-override-instances",
			From:         pointers.From("srv-source"),
			NumInstances: pointers.From(1),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, 1, *input.NumInstances)
	})

	t.Run("preserves pre-populated MaxShutdownDelay override when source has different value", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime:                 client.ServiceRuntimeNode,
				MaxShutdownDelaySeconds: pointers.From(60),
				EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{
					BuildCommand: "npm ci",
					StartCommand: "npm start",
				}),
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:             "clone-override-shutdown",
			From:             pointers.From("srv-source"),
			MaxShutdownDelay: pointers.From(30),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, 30, *input.MaxShutdownDelay)
	})

	t.Run("preserves pre-populated Previews override when source has different value", func(t *testing.T) {
		gen := client.PreviewsGeneration("automatic")
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime:  client.ServiceRuntimeNode,
				Previews: &client.Previews{Generation: &gen},
				EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{
					BuildCommand: "npm ci",
					StartCommand: "npm start",
				}),
			}),
		}
		manual := servicetypes.PreviewsGenerationManual
		input := servicetypes.ServiceCreateInput{
			Name:     "clone-override-previews",
			From:     pointers.From("srv-source"),
			Previews: &manual,
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, servicetypes.PreviewsGenerationManual, *input.Previews)
	})

	t.Run("preserves pre-populated MaintenanceMode override when source has different value", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime:         client.ServiceRuntimeNode,
				MaintenanceMode: &client.MaintenanceMode{Enabled: true, Uri: "/maint.html"},
				EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{
					BuildCommand: "npm ci",
					StartCommand: "npm start",
				}),
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:            "clone-override-maint",
			From:            pointers.From("srv-source"),
			MaintenanceMode: pointers.From(false),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, false, *input.MaintenanceMode)
	})

	t.Run("preserves pre-populated MaintenanceModeURI override when source has different value", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime:         client.ServiceRuntimeNode,
				MaintenanceMode: &client.MaintenanceMode{Enabled: true, Uri: "/default-maint.html"},
				EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{
					BuildCommand: "npm ci",
					StartCommand: "npm start",
				}),
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:               "clone-override-maint-uri",
			From:               pointers.From("srv-source"),
			MaintenanceModeURI: pointers.From("/custom-maint.html"),
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, "/custom-maint.html", *input.MaintenanceModeURI)
	})

	t.Run("preserves pre-populated BuildFilterPaths override when source has different value", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			BuildFilter: &client.BuildFilter{
				Paths:        []string{"src/", "lib/"},
				IgnoredPaths: []string{"test/", "docs/"},
			},
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode, client.NativeEnvironmentDetails{}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:             "clone-override-build-paths",
			From:             pointers.From("srv-source"),
			BuildFilterPaths: []string{"custom/path/"},
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, []string{"custom/path/"}, input.BuildFilterPaths)
	})

	t.Run("preserves pre-populated BuildFilterIgnoredPaths override when source has different value", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			BuildFilter: &client.BuildFilter{
				Paths:        []string{"src/", "lib/"},
				IgnoredPaths: []string{"test/", "docs/"},
			},
			ServiceDetails: mustWebServiceDetails(t, client.ServiceRuntimeNode, client.NativeEnvironmentDetails{}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:                    "clone-override-ignored-paths",
			From:                    pointers.From("srv-source"),
			BuildFilterIgnoredPaths: []string{"vendor/", "dist/"},
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, []string{"vendor/", "dist/"}, input.BuildFilterIgnoredPaths)
	})

	t.Run("preserves pre-populated IPAllowList override when source has different value", func(t *testing.T) {
		source := &client.Service{
			Type:    client.WebService,
			Repo:    pointers.From("https://github.com/renderinc/api"),
			Branch:  pointers.From("main"),
			RootDir: ".",
			ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
				Runtime: client.ServiceRuntimeNode,
				IpAllowList: &[]client.CidrBlockAndDescription{
					{CidrBlock: "10.0.0.0/8", Description: "Internal"},
				},
				EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{}),
			}),
		}
		input := servicetypes.ServiceCreateInput{
			Name:        "clone-override-ip-allowlist",
			From:        pointers.From("srv-source"),
			IPAllowList: []string{"cidr=192.168.1.0/24,description=Custom"},
		}

		ServiceFromAPI(&input, source)

		require.Equal(t, []string{"cidr=192.168.1.0/24,description=Custom"}, input.IPAllowList)
	})
}

// A clone may change the service type or swap a native runtime for a prebuilt
// image. It inherits only the source settings the destination accepts, so
// validation never refuses a setting the caller did not supply; settings the
// caller does supply are validated as usual.
func TestServiceFromAPIInheritsOnlySettingsTheDestinationAccepts(t *testing.T) {
	cloneAs := func(serviceType servicetypes.ServiceType) servicetypes.ServiceCreateInput {
		return servicetypes.ServiceCreateInput{Name: "clone", From: pointers.From("srv-source"), Type: svcType(serviceType)}
	}
	computeTypes := []servicetypes.ServiceType{
		servicetypes.ServiceTypeWebService, servicetypes.ServiceTypePrivateService,
		servicetypes.ServiceTypeBackgroundWorker, servicetypes.ServiceTypeCronJob,
	}

	t.Run("static site to compute service", func(t *testing.T) {
		for _, serviceType := range computeTypes {
			t.Run(string(serviceType), func(t *testing.T) {
				input := cloneAs(serviceType)
				input.Runtime = svcRuntime(servicetypes.ServiceRuntimeNode)
				if serviceType == servicetypes.ServiceTypeCronJob {
					input.CronCommand = pointers.From("node job.js")
					input.CronSchedule = pointers.From("0 * * * *")
				} else {
					input.StartCommand = pointers.From("npm start")
				}

				got, body, err := cloneCreateRequest(t, input, cloneSourceStaticSite(t))

				require.NoError(t, err)
				require.Equal(t, client.ServiceType(serviceType), body.Type)
				require.Nil(t, got.PublishDirectory)
				require.Equal(t, "https://github.com/org/site", *got.Repo)
				require.Equal(t, "site", *got.RootDirectory)
				require.Equal(t, "npm run build", *got.BuildCommand)
				if serviceType == servicetypes.ServiceTypeCronJob {
					require.Nil(t, got.Previews)
				} else {
					require.Equal(t, servicetypes.PreviewsGenerationManual, *got.Previews)
				}
				if serviceType == servicetypes.ServiceTypeWebService {
					require.Equal(t, []string{"cidr=10.0.0.0/8,description=office"}, got.IPAllowList)
				} else {
					require.Empty(t, got.IPAllowList)
				}
			})
		}
	})

	t.Run("web service to static site", func(t *testing.T) {
		got, body, err := cloneCreateRequest(t, cloneAs(servicetypes.ServiceTypeStaticSite), cloneSourceWebService(t))

		require.NoError(t, err)
		require.Equal(t, client.StaticSite, body.Type)
		require.Nil(t, got.Runtime)
		require.Nil(t, got.Region)
		require.Nil(t, got.Plan)
		require.Nil(t, got.StartCommand)
		require.Nil(t, got.PreDeployCommand)
		require.Nil(t, got.HealthCheckPath)
		require.Nil(t, got.NumInstances)
		require.Nil(t, got.MaxShutdownDelay)
		require.Nil(t, got.MaintenanceMode)
		require.Equal(t, "npm ci", *got.BuildCommand)
		require.Equal(t, servicetypes.PreviewsGenerationAutomatic, *got.Previews)
		require.Equal(t, []string{"cidr=10.0.0.0/8,description=office"}, got.IPAllowList)
	})

	t.Run("web service to cron job", func(t *testing.T) {
		input := cloneAs(servicetypes.ServiceTypeCronJob)
		input.CronCommand = pointers.From("node job.js")
		input.CronSchedule = pointers.From("0 0 1 1 *")

		got, body, err := cloneCreateRequest(t, input, cloneSourceWebService(t))

		require.NoError(t, err)
		require.Nil(t, got.HealthCheckPath)
		require.Nil(t, got.StartCommand)
		require.Nil(t, got.PreDeployCommand)
		require.Nil(t, got.NumInstances)
		require.Nil(t, got.MaxShutdownDelay)
		require.Nil(t, got.Previews)
		require.Nil(t, got.MaintenanceMode)
		require.Nil(t, got.MaintenanceModeURI)
		require.Empty(t, got.IPAllowList)
		require.Equal(t, map[string]any{
			"runtime":  "node",
			"region":   "frankfurt",
			"plan":     "starter",
			"schedule": "0 0 1 1 *",
			"envSpecificDetails": map[string]any{
				"buildCommand": "npm ci",
				"startCommand": "node job.js",
			},
		}, testrequire.AsJSONMap(t, body.ServiceDetails))
	})

	for _, serviceType := range []servicetypes.ServiceType{servicetypes.ServiceTypePrivateService, servicetypes.ServiceTypeBackgroundWorker} {
		t.Run("web service to "+string(serviceType), func(t *testing.T) {
			got, body, err := cloneCreateRequest(t, cloneAs(serviceType), cloneSourceWebService(t))

			require.NoError(t, err)
			require.Nil(t, got.HealthCheckPath)
			require.Nil(t, got.MaintenanceMode)
			require.Nil(t, got.MaintenanceModeURI)
			require.Empty(t, got.IPAllowList)
			require.Equal(t, map[string]any{
				"runtime":                 "node",
				"region":                  "frankfurt",
				"plan":                    "starter",
				"preDeployCommand":        "npm run migrate",
				"numInstances":            float64(2),
				"maxShutdownDelaySeconds": float64(45),
				"previews":                map[string]any{"generation": "automatic"},
				"envSpecificDetails": map[string]any{
					"buildCommand": "npm ci",
					"startCommand": "npm start",
				},
			}, testrequire.AsJSONMap(t, body.ServiceDetails))
		})
	}

	t.Run("cron job to web service", func(t *testing.T) {
		input := cloneAs(servicetypes.ServiceTypeWebService)
		input.StartCommand = pointers.From("./server")

		got, _, err := cloneCreateRequest(t, input, cloneSourceCronJob(t))

		require.NoError(t, err)
		require.Nil(t, got.CronCommand)
		require.Nil(t, got.CronSchedule)
		require.Equal(t, servicetypes.ServiceRuntimeGo, *got.Runtime)
		require.Equal(t, "go build -o job .", *got.BuildCommand)
		require.Equal(t, "starter", *got.Plan)
	})

	nativeSources := map[servicetypes.ServiceType]func(*testing.T) *client.Service{
		servicetypes.ServiceTypeWebService: cloneSourceWebService,
		servicetypes.ServiceTypePrivateService: func(t *testing.T) *client.Service {
			return &client.Service{
				Type: client.PrivateService, Repo: pointers.From("https://github.com/org/app"), Branch: pointers.From("main"),
				ServiceDetails: mustPrivateServiceDetails(t, client.ServiceRuntimeNode,
					client.NativeEnvironmentDetails{BuildCommand: "npm ci", StartCommand: "npm start"}),
			}
		},
		servicetypes.ServiceTypeBackgroundWorker: func(t *testing.T) *client.Service {
			return &client.Service{
				Type: client.BackgroundWorker, Repo: pointers.From("https://github.com/org/app"), Branch: pointers.From("main"),
				ServiceDetails: mustBackgroundWorkerDetails(t, client.ServiceRuntimeNode,
					client.NativeEnvironmentDetails{BuildCommand: "npm ci", StartCommand: "npm start"}),
			}
		},
		servicetypes.ServiceTypeCronJob: cloneSourceCronJob,
	}
	for _, serviceType := range computeTypes {
		t.Run("native "+string(serviceType)+" to prebuilt image", func(t *testing.T) {
			input := servicetypes.ServiceCreateInput{
				Name: "clone", From: pointers.From("srv-source"), Image: pointers.From("docker.io/org/app:1.2.3"),
			}

			got, body, err := cloneCreateRequest(t, input, nativeSources[serviceType](t))

			require.NoError(t, err)
			require.Equal(t, client.ServiceType(serviceType), body.Type)
			require.Equal(t, servicetypes.ServiceRuntimeImage, *got.Runtime)
			require.Nil(t, got.BuildCommand)
			require.Nil(t, got.StartCommand)
			require.Nil(t, got.Repo)
			require.Nil(t, got.Branch)
			require.Equal(t, "docker.io/org/app:1.2.3", body.Image.ImagePath)
			if serviceType == servicetypes.ServiceTypeWebService {
				require.Equal(t, "/healthz", *got.HealthCheckPath)
				require.Equal(t, "npm run migrate", *got.PreDeployCommand)
				require.Equal(t, 2, *got.NumInstances)
			}
			if serviceType == servicetypes.ServiceTypeCronJob {
				require.Equal(t, "0 * * * *", *got.CronSchedule)
				require.Equal(t, "./job", *got.CronCommand)
			}
		})
	}

	t.Run("prebuilt image inherited from the source", func(t *testing.T) {
		// The clone's runtime is only known once the source's image is applied.
		source := &client.Service{
			Type:      client.PrivateService,
			ImagePath: pointers.From("docker.io/org/app:latest"),
			ServiceDetails: mustPrivateServiceDetails(t, client.ServiceRuntimeImage,
				client.NativeEnvironmentDetails{StartCommand: "run"}),
		}

		got, body, err := cloneCreateRequest(t, servicetypes.ServiceCreateInput{Name: "clone", From: pointers.From("srv-source")}, source)

		require.NoError(t, err)
		require.Nil(t, got.StartCommand)
		require.Equal(t, "docker.io/org/app:latest", body.Image.ImagePath)
	})

	t.Run("explicit unsupported settings are still refused", func(t *testing.T) {
		cases := []struct {
			name    string
			source  func(*testing.T) *client.Service
			input   func() servicetypes.ServiceCreateInput
			wantErr string
		}{
			{
				name:   "publish directory for a web service",
				source: cloneSourceStaticSite,
				input: func() servicetypes.ServiceCreateInput {
					input := cloneAs(servicetypes.ServiceTypeWebService)
					input.Runtime = svcRuntime(servicetypes.ServiceRuntimeNode)
					input.StartCommand = pointers.From("npm start")
					input.PublishDirectory = pointers.From("dist")
					return input
				},
				wantErr: "--publish-directory is not supported for web_service",
			},
			{
				name:   "health check path for a cron job",
				source: cloneSourceWebService,
				input: func() servicetypes.ServiceCreateInput {
					input := cloneAs(servicetypes.ServiceTypeCronJob)
					input.HealthCheckPath = pointers.From("/healthz")
					return input
				},
				wantErr: "--health-check-path is not supported for cron_job",
			},
			{
				name:   "start command for a cron job",
				source: cloneSourceWebService,
				input: func() servicetypes.ServiceCreateInput {
					input := cloneAs(servicetypes.ServiceTypeCronJob)
					input.StartCommand = pointers.From("npm start")
					return input
				},
				wantErr: "--start-command is not supported for cron_job",
			},
			{
				name:   "maintenance mode for a private service",
				source: cloneSourceWebService,
				input: func() servicetypes.ServiceCreateInput {
					input := cloneAs(servicetypes.ServiceTypePrivateService)
					input.MaintenanceMode = pointers.From(true)
					return input
				},
				wantErr: "--maintenance-mode is not supported for private_service",
			},
			{
				name:   "build command for a prebuilt image",
				source: cloneSourceWebService,
				input: func() servicetypes.ServiceCreateInput {
					return servicetypes.ServiceCreateInput{
						Name: "clone", From: pointers.From("srv-source"),
						Image: pointers.From("docker.io/org/app:1.2.3"), BuildCommand: pointers.From("npm ci"),
					}
				},
				wantErr: "--build-command and --start-command are only supported for native runtimes",
			},
			{
				name:   "start command for a prebuilt image",
				source: cloneSourceWebService,
				input: func() servicetypes.ServiceCreateInput {
					return servicetypes.ServiceCreateInput{
						Name: "clone", From: pointers.From("srv-source"),
						Image: pointers.From("docker.io/org/app:1.2.3"), StartCommand: pointers.From("npm start"),
					}
				},
				wantErr: "--build-command and --start-command are only supported for native runtimes",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, _, err := cloneCreateRequest(t, tc.input(), tc.source(t))
				require.EqualError(t, err, tc.wantErr)
			})
		}
	})

	t.Run("same type and runtime keeps every setting", func(t *testing.T) {
		web, body, err := cloneCreateRequest(t, servicetypes.ServiceCreateInput{Name: "clone", From: pointers.From("srv-source")}, cloneSourceWebService(t))
		require.NoError(t, err)
		require.Equal(t, "app", *web.RootDirectory)
		require.Equal(t, []string{"app/**"}, web.BuildFilterPaths)
		require.False(t, *web.AutoDeploy)
		require.Equal(t, map[string]any{
			"runtime":                 "node",
			"region":                  "frankfurt",
			"plan":                    "starter",
			"healthCheckPath":         "/healthz",
			"preDeployCommand":        "npm run migrate",
			"numInstances":            float64(2),
			"maxShutdownDelaySeconds": float64(45),
			"previews":                map[string]any{"generation": "automatic"},
			"maintenanceMode":         map[string]any{"enabled": false, "uri": ""},
			"ipAllowList":             []any{map[string]any{"cidrBlock": "10.0.0.0/8", "description": "office"}},
			"envSpecificDetails": map[string]any{
				"buildCommand": "npm ci",
				"startCommand": "npm start",
			},
		}, testrequire.AsJSONMap(t, body.ServiceDetails))

		static, body, err := cloneCreateRequest(t, servicetypes.ServiceCreateInput{Name: "clone", From: pointers.From("srv-source")}, cloneSourceStaticSite(t))
		require.NoError(t, err)
		require.Equal(t, "site", *static.RootDirectory)
		require.Equal(t, map[string]any{
			"buildCommand": "npm run build",
			"publishPath":  "dist",
			"previews":     map[string]any{"generation": "manual"},
			"ipAllowList":  []any{map[string]any{"cidrBlock": "10.0.0.0/8", "description": "office"}},
		}, testrequire.AsJSONMap(t, body.ServiceDetails))
	})

	t.Run("another native runtime keeps the commands", func(t *testing.T) {
		input := servicetypes.ServiceCreateInput{
			Name: "clone", From: pointers.From("srv-source"), Runtime: svcRuntime(servicetypes.ServiceRuntimePython),
		}

		got, _, err := cloneCreateRequest(t, input, cloneSourceWebService(t))

		require.NoError(t, err)
		require.Equal(t, servicetypes.ServiceRuntimePython, *got.Runtime)
		require.Equal(t, "npm ci", *got.BuildCommand)
		require.Equal(t, "npm start", *got.StartCommand)
		require.Equal(t, "npm run migrate", *got.PreDeployCommand)
		require.Equal(t, "/healthz", *got.HealthCheckPath)
	})
}

// cloneCreateRequest runs a clone through the same steps as `services create
// --from`: normalize the flags, merge the source's defaults, then validate the
// result and build the create request from it.
func cloneCreateRequest(t *testing.T, input servicetypes.ServiceCreateInput, source *client.Service) (servicetypes.ServiceCreateInput, client.CreateServiceJSONRequestBody, error) {
	t.Helper()

	input = servicetypes.NormalizeServiceCreateCLIInput(input)
	ServiceFromAPI(&input, source)
	input, err := servicetypes.NormalizeAndValidateCreateInput(input, false)
	if err != nil {
		return input, client.CreateServiceJSONRequestBody{}, err
	}
	body, err := BuildCreateRequest(input, "tea-abc123")
	require.NoError(t, err)
	return input, body, nil
}

func cloneSourceStaticSite(t *testing.T) *client.Service {
	t.Helper()
	gen := client.PreviewsGeneration("manual")
	return &client.Service{
		Type:    client.StaticSite,
		Repo:    pointers.From("https://github.com/org/site"),
		Branch:  pointers.From("main"),
		RootDir: "site",
		ServiceDetails: mustStaticSiteDetailsExtended(t, client.StaticSiteDetails{
			BuildCommand: "npm run build",
			PublishPath:  "dist",
			Previews:     &client.Previews{Generation: &gen},
			IpAllowList:  &[]client.CidrBlockAndDescription{{CidrBlock: "10.0.0.0/8", Description: "office"}},
		}),
	}
}

func cloneSourceWebService(t *testing.T) *client.Service {
	t.Helper()
	gen := client.PreviewsGeneration("automatic")
	return &client.Service{
		Type:        client.WebService,
		Repo:        pointers.From("https://github.com/org/app"),
		Branch:      pointers.From("main"),
		RootDir:     "app",
		AutoDeploy:  client.AutoDeployNo,
		BuildFilter: &client.BuildFilter{Paths: []string{"app/**"}},
		ServiceDetails: mustWebServiceDetailsExtended(t, client.WebServiceDetails{
			Runtime:         client.ServiceRuntimeNode,
			Region:          client.Frankfurt,
			Plan:            client.PlanStarter,
			HealthCheckPath: "/healthz",
			EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{
				BuildCommand:     "npm ci",
				StartCommand:     "npm start",
				PreDeployCommand: pointers.From("npm run migrate"),
			}),
			NumInstances:            2,
			MaxShutdownDelaySeconds: pointers.From(45),
			Previews:                &client.Previews{Generation: &gen},
			// The API reports maintenance mode even while it is disabled.
			MaintenanceMode: &client.MaintenanceMode{Enabled: false, Uri: ""},
			IpAllowList:     &[]client.CidrBlockAndDescription{{CidrBlock: "10.0.0.0/8", Description: "office"}},
		}),
	}
}

func cloneSourceCronJob(t *testing.T) *client.Service {
	t.Helper()
	var details client.Service_ServiceDetails
	require.NoError(t, details.FromCronJobDetails(client.CronJobDetails{
		Runtime:            client.ServiceRuntimeGo,
		Region:             client.Frankfurt,
		Plan:               client.PlanStarter,
		Schedule:           "0 * * * *",
		EnvSpecificDetails: mustEnvSpecific(t, client.NativeEnvironmentDetails{BuildCommand: "go build -o job .", StartCommand: "./job"}),
	}))
	return &client.Service{
		Type:           client.CronJob,
		Repo:           pointers.From("https://github.com/org/job"),
		Branch:         pointers.From("main"),
		ServiceDetails: details,
	}
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

func mustEnvSpecific(t *testing.T, native client.NativeEnvironmentDetails) client.EnvSpecificDetails {
	t.Helper()
	var envSpecific client.EnvSpecificDetails
	require.NoError(t, envSpecific.FromNativeEnvironmentDetails(native))
	return envSpecific
}

func mustWebServiceDetailsExtended(t *testing.T, details client.WebServiceDetails) client.Service_ServiceDetails {
	t.Helper()
	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromWebServiceDetails(details))
	return serviceDetails
}

func mustStaticSiteDetailsExtended(t *testing.T, details client.StaticSiteDetails) client.Service_ServiceDetails {
	t.Helper()
	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromStaticSiteDetails(details))
	return serviceDetails
}

func mustBackgroundWorkerDetailsExtended(t *testing.T, details client.BackgroundWorkerDetails) client.Service_ServiceDetails {
	t.Helper()
	var serviceDetails client.Service_ServiceDetails
	require.NoError(t, serviceDetails.FromBackgroundWorkerDetails(details))
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
