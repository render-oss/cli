package renderapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func newBlueprintApproveFixture(t *testing.T) (*Server, *client.ClientWithResponses) {
	t.Helper()
	server := NewServer(t)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return server, c
}

func TestFakeRenderServerBlueprintApprove(t *testing.T) {
	t.Run("approves a pending sync for a known Blueprint", func(t *testing.T) {
		server, c := newBlueprintApproveFixture(t)
		blueprint := server.Blueprints.Add(NewBlueprint(bptypes.Blueprint{}))

		resp, err := c.ApproveBlueprintWithResponse(context.Background(), blueprint.Id, bptypes.ApproveBlueprintRequest{})

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, bptypes.SyncStatePending, resp.JSON200.State)
		require.Contains(t, resp.JSON200.DashboardUrl, blueprint.Id)
	})

	t.Run("returns 404 for an unknown Blueprint", func(t *testing.T) {
		_, c := newBlueprintApproveFixture(t)

		resp, err := c.ApproveBlueprintWithResponse(context.Background(), testids.BlueprintID("unknown"), bptypes.ApproveBlueprintRequest{})

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}
