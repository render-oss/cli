package renderapi

import (
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/require"
)

// TestServiceConstructors documents how to build each service type and guards
// the one thing the compiler can't: that every constructor populates the
// matching ServiceDetails union variant. A copy-pasted FromXDetails call would
// type-check fine and only blow up when a test reads back the wrong variant.
func TestServiceConstructors(t *testing.T) {
	t.Run("web service", func(t *testing.T) {
		svc := NewWebService(WebServiceAttrs{Service: CommonServiceAttrs{Name: "web"}})

		assertServiceResource(t, svc, client.WebService, "srv-")
		_, err := svc.ServiceDetails.AsWebServiceDetails()
		require.NoError(t, err)
	})

	t.Run("background worker", func(t *testing.T) {
		svc := NewBackgroundWorker(BackgroundWorkerAttrs{Service: CommonServiceAttrs{Name: "worker"}})

		assertServiceResource(t, svc, client.BackgroundWorker, "srv-")
		_, err := svc.ServiceDetails.AsBackgroundWorkerDetails()
		require.NoError(t, err)
	})

	t.Run("private service", func(t *testing.T) {
		svc := NewPrivateService(PrivateServiceAttrs{Service: CommonServiceAttrs{Name: "private"}})

		assertServiceResource(t, svc, client.PrivateService, "srv-")
		_, err := svc.ServiceDetails.AsPrivateServiceDetails()
		require.NoError(t, err)
	})

	t.Run("static site", func(t *testing.T) {
		svc := NewStaticSite(StaticSiteAttrs{Service: CommonServiceAttrs{Name: "static"}})

		assertServiceResource(t, svc, client.StaticSite, "srv-")
		_, err := svc.ServiceDetails.AsStaticSiteDetails()
		require.NoError(t, err)
	})

	t.Run("cron job", func(t *testing.T) {
		svc := NewCronJob(CronJobAttrs{Service: CommonServiceAttrs{Name: "cron"}})

		// Cron jobs get a crn- ID; every other service type gets srv-.
		assertServiceResource(t, svc, client.CronJob, "crn-")
		_, err := svc.ServiceDetails.AsCronJobDetails()
		require.NoError(t, err)
	})
}

func assertServiceResource(t *testing.T, svc *client.Service, wantType client.ServiceType, wantPrefix string) {
	t.Helper()
	require.Equal(t, wantType, svc.Type)
	require.True(t, strings.HasPrefix(svc.Id, wantPrefix))
}

// TestRuntimeDetailsConstructors documents how to attach each runtime kind to a
// service and guards the runtime->env mapping (and, for images, that the path
// is stamped onto the service).
func TestRuntimeDetailsConstructors(t *testing.T) {
	t.Run("native runtime", func(t *testing.T) {
		svc := NewWebService(WebServiceAttrs{
			Service: CommonServiceAttrs{Name: "native-runtime"},
			Details: WebServiceDetailsAttrs{
				RuntimeDetails: NewNativeRuntimeDetails(NativeRuntimeAttrs{
					Runtime: client.ServiceRuntimeRuby,
				}),
			},
		})

		web, err := svc.ServiceDetails.AsWebServiceDetails()
		require.NoError(t, err)
		require.Equal(t, client.ServiceRuntimeRuby, web.Runtime)
		require.Equal(t, client.ServiceEnvRuby, web.Env)
	})

	t.Run("docker runtime", func(t *testing.T) {
		svc := NewWebService(WebServiceAttrs{
			Service: CommonServiceAttrs{Name: "docker-runtime"},
			Details: WebServiceDetailsAttrs{
				RuntimeDetails: NewDockerRuntimeDetails(DockerRuntimeAttrs{
					DockerCommand:  "bin/web",
					DockerContext:  "./app",
					DockerfilePath: "Dockerfile.web",
				}),
			},
		})

		web, err := svc.ServiceDetails.AsWebServiceDetails()
		require.NoError(t, err)
		require.Equal(t, client.ServiceRuntimeDocker, web.Runtime)
		require.Equal(t, client.ServiceEnvDocker, web.Env)
		_, err = web.EnvSpecificDetails.AsDockerDetails()
		require.NoError(t, err)
	})

	t.Run("image runtime", func(t *testing.T) {
		svc := NewWebService(WebServiceAttrs{
			Service: CommonServiceAttrs{Name: "image-runtime"},
			Details: WebServiceDetailsAttrs{
				RuntimeDetails: NewImageRuntimeDetails(ImageRuntimeAttrs{ImagePath: "docker.io/render/example:latest"}),
			},
		})

		require.NotNil(t, svc.ImagePath)
		require.Equal(t, "docker.io/render/example:latest", *svc.ImagePath)
		web, err := svc.ServiceDetails.AsWebServiceDetails()
		require.NoError(t, err)
		require.Equal(t, client.ServiceRuntimeImage, web.Runtime)
		require.Equal(t, client.ServiceEnvImage, web.Env)
	})
}
