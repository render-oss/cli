package deploy

import (
	"slices"

	"github.com/renderinc/cli/pkg/client"
)

var cancellableStatuses = []client.DeployStatus{
	client.DeployStatusCreated,
	client.DeployStatusBuildInProgress,
	client.DeployStatusUpdateInProgress,
	client.DeployStatusPreDeployInProgress,
}

func IsCancellable(status *client.DeployStatus) bool {
	return status == nil || slices.Contains(cancellableStatuses, *status)
}

func IsComplete(status *client.DeployStatus) bool {
	if status == nil {
		return false
	}
	switch *status {
	case client.DeployStatusBuildFailed,
		client.DeployStatusCanceled,
		client.DeployStatusDeactivated,
		client.DeployStatusLive,
		client.DeployStatusPreDeployFailed,
		client.DeployStatusUpdateFailed:
		return true
	case client.DeployStatusBuildInProgress,
		client.DeployStatusCreated,
		client.DeployStatusPreDeployInProgress,
		client.DeployStatusUpdateInProgress:
		return false
	default:
		return false
	}
}

func IsSuccessful(status *client.DeployStatus) bool {
	if status == nil {
		return false
	}

	return *status == client.DeployStatusLive || *status == client.DeployStatusDeactivated
}
