package text

import (
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/deploy"
)

func FormatString(s string) string {
	return FormatStringF(s)
}

func FormatStringF(s string, a ...any) string {
	return fmt.Sprintf(s+"\n", a...)
}

func Deploy(serviceID string) func(dep *client.Deploy) string {
	return func(dep *client.Deploy) string {
		if deploy.IsSuccessful(dep.Status) {
			return FormatStringF("Deploy %s succeeded for service %s", dep.Id, serviceID)
		} else if deploy.IsComplete(dep.Status) {
			switch *dep.Status {
			case client.DeployStatusBuildFailed:
				return FormatStringF("Build failed for deploy %s", dep.Id)
			case client.DeployStatusPreDeployFailed:
				return FormatStringF("Pre Deploy failed for deploy %s", dep.Id)
			default:
				return FormatStringF("Deploy %s failed for service %s", dep.Id, serviceID)
			}
		}

		return FormatStringF("Created deploy %s for service %s", dep.Id, serviceID)
	}
}
