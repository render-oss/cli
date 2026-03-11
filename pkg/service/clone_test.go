package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

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
