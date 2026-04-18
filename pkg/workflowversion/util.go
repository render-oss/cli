package workflowversion

import (
	wfclient "github.com/render-oss/cli/v2/pkg/client/workflows"
)

func IsComplete(status wfclient.WorkflowVersionStatus) bool {
	switch status {
	case wfclient.Ready,
		wfclient.BuildFailed,
		wfclient.RegistrationFailed:
		return true
	case wfclient.Created,
		wfclient.Building,
		wfclient.Registering:
		return false
	default:
		return false
	}
}

func IsSuccessful(status wfclient.WorkflowVersionStatus) bool {
	return status == wfclient.Ready
}
