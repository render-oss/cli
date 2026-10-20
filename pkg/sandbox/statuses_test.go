package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

func TestAllStatuses(t *testing.T) {
	assert.ElementsMatch(t, []sandboxclient.SandboxStatus{
		sandboxclient.SandboxStatusCreating,
		sandboxclient.SandboxStatusRunning,
		sandboxclient.SandboxStatusSuspending,
		sandboxclient.SandboxStatusSuspended,
		sandboxclient.SandboxStatusResuming,
		sandboxclient.SandboxStatusErrored,
		sandboxclient.SandboxStatusTerminated,
	}, AllStatuses())

	for _, s := range AllStatuses() {
		assert.True(t, s.Valid(), "%q should be a schema status", s)
	}
}

func TestAllStatuses_ReturnsCopy(t *testing.T) {
	AllStatuses()[0] = "mutated"
	assert.Equal(t, sandboxclient.SandboxStatusCreating, AllStatuses()[0])
}
