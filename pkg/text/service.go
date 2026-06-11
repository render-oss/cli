package text

import (
	"fmt"
	"strings"

	"github.com/render-oss/cli/pkg/service"
	rstrings "github.com/render-oss/cli/pkg/strings"
)

func ServiceDetail(svc *service.ServiceOut) string {
	if svc == nil {
		return ""
	}
	data := svc.Service
	lines := []string{
		fmt.Sprintf("Name: %s", data.Name),
		fmt.Sprintf("ID: %s", data.Id),
		fmt.Sprintf("Type: %s", string(data.Type)),
		fmt.Sprintf("Owner ID: %s", data.OwnerId),
	}
	projectID := ""
	if svc.ProjectID != nil {
		projectID = *svc.ProjectID
	}
	if label := rstrings.ResourceLabel(svc.ProjectName, projectID); label != "" {
		lines = append(lines, fmt.Sprintf("Project: %s", label))
	}

	environmentID := ""
	if data.EnvironmentId != nil {
		environmentID = *data.EnvironmentId
	}
	if label := rstrings.ResourceLabel(svc.EnvironmentName, environmentID); label != "" {
		lines = append(lines, fmt.Sprintf("Environment: %s", label))
	}
	if data.DashboardUrl != "" {
		lines = append(lines, fmt.Sprintf("Dashboard: %s", data.DashboardUrl))
	}
	return strings.Join(lines, "\n")
}
