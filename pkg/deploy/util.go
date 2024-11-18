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

