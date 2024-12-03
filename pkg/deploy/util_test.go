package deploy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/deploy"
)

func TestIsComplete(t *testing.T) {
	t.Run("handles nil status", func(t *testing.T) {
		assert.False(t, deploy.IsComplete(nil))
	})

	tests := map[client.DeployStatus]bool{
		client.DeployStatusBuildFailed:         true,
		client.DeployStatusBuildInProgress:     false,
		client.DeployStatusCanceled:            true,
		client.DeployStatusCreated:             false,
		client.DeployStatusDeactivated:         true,
		client.DeployStatusLive:                true,
		client.DeployStatusPreDeployFailed:     true,
		client.DeployStatusPreDeployInProgress: false,
		client.DeployStatusUpdateFailed:        true,
		client.DeployStatusUpdateInProgress:    false,
	}

	for status, expected := range tests {
		t.Run(string(status), func(t *testing.T) {
			actual := deploy.IsComplete(&status)
			assert.Equal(t, expected, actual)
		})
	}
}
