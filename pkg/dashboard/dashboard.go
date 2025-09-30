package dashboard

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/service"
	"github.com/render-oss/cli/pkg/workflow"
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

func OpenVersion(workflowID, versionID string) error {
	// TODO TBD on path naming/visibility down the line
	dashURL := resourceURL(workflowID, workflow.WorkflowType)
	return Open(fmt.Sprintf("%s/versions/%s", dashURL, versionID))
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
	case keyvalue.KeyValueType:
		return "r"
	case postgres.PostgresType:
		return "d"
	case workflow.WorkflowType:
		return "wf"
	default:
		return "web"
	}
}
