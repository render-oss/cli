package renderapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
	"github.com/stretchr/testify/require"
)

// TestCliTelemetryEndpointCollectsEvents verifies the fake records posted CLI
// telemetry events on server.CliTelemetry so consumer tests can assert on the
// analytics a command emitted.
func TestCliTelemetryEndpointCollectsEvents(t *testing.T) {
	server := NewServer(t)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)

	resp, err := c.CreateCliTelemetryEvent(context.Background(), client.CreateCliTelemetryEventJSONRequestBody{
		Command:        "render services list",
		CompletionKind: telemetryclient.Success,
		ExitCode:       0,
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	event := server.CliTelemetry.Only(t)
	require.Equal(t, "render services list", event.Command)
	require.Equal(t, telemetryclient.Success, event.CompletionKind)
}
