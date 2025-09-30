package text

import (
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"

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

func Version(workflowID string) func(wfv *wfclient.WorkflowVersion) string {
	return func(wfv *wfclient.WorkflowVersion) string {
		// TODO CAP-7490
		// https://linear.app/render-com/issue/CAP-7490/flesh-out-workflow-version-information-at-least-restgql-if-not-present

		return FormatStringF("Released version %s for workflow %s", wfv.Id, workflowID)
	}
}

func TaskRunDetails(taskRun *wfclient.TaskRunDetails) string {
	errorOrResults := ""
	if taskRun.Results != nil {
		errorOrResults = fmt.Sprintf("results: %v", taskRun.Results)
	} else if taskRun.Error != nil {
		errorOrResults = fmt.Sprintf("error: %s", *taskRun.Error)
	}

	return FormatStringF(
		"Task run details for %s, status %s, started at %s, completed at %s\n, %s",
		taskRun.Id, taskRun.Status, taskRun.StartedAt, taskRun.CompletedAt, errorOrResults)
}
