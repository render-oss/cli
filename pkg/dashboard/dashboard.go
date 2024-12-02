package dashboard

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/renderinc/cli/pkg/config"
	"github.com/renderinc/cli/pkg/postgres"
	"github.com/renderinc/cli/pkg/redis"
	"github.com/renderinc/cli/pkg/service"
)

func Open(url string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return nil
	}
}

func OpenResource(id, resourceType string) error {
	return Open(resourceURL(id, resourceType))
}

func resourceURL(id string, resourceType string) string {
	dashURL := config.DashboardURL()

	resourceTypePathSegment := pathSegmentFromResourceType(resourceType)

	dashURL += fmt.Sprintf("/%s/%s", resourceTypePathSegment, id)
	return dashURL
}

func OpenDeploy(resourceID, resourceType, deployID string) error {
	dashURL := resourceURL(resourceID, resourceType)

	dashURL = fmt.Sprintf("%s/deploys/%s", dashURL, deployID)

	return Open(dashURL)
}

func pathSegmentFromResourceType(resourceType string) string {
	switch resourceType {
	case service.WebServiceResourceType:
		return "web"
	case service.BackgroundWorkerResourceType:
		return "worker"
	case service.PrivateServiceResourceType:
		return "pserv"
	case service.StaticSiteResourceType:
		return "static"
	case service.CronJobResourceType:
		return "cron"
	case redis.RedisType:
		return "r"
	case postgres.PostgresType:
		return "d"
	default:
		return "web"
	}
}
