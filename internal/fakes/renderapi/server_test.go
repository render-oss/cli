package renderapi

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
)

// newServerAndClient returns an empty fake server and an API client for it.
func newServerAndClient(t *testing.T) (*Server, *client.ClientWithResponses) {
	t.Helper()
	server := NewServer(t)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return server, c
}
