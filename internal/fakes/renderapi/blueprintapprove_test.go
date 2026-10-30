package renderapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testids"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func TestFakeRenderServerBlueprintApprove(t *testing.T) {
	t.Run("approves a pending sync for a known Blueprint", func(t *testing.T) {
		server, c := newServerAndClient(t)
		blueprint := server.Blueprints.Add(NewBlueprint(bptypes.Blueprint{}))

		resp, err := c.ApproveBlueprintWithResponse(context.Background(), blueprint.Id, bptypes.ApproveBlueprintRequest{})

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.NotNil(t, resp.JSON200)
		require.Equal(t, bptypes.SyncStatePending, resp.JSON200.State)
		require.Contains(t, resp.JSON200.DashboardUrl, blueprint.Id)
	})

	t.Run("returns 404 for an unknown Blueprint", func(t *testing.T) {
		_, c := newServerAndClient(t)

		resp, err := c.ApproveBlueprintWithResponse(context.Background(), testids.BlueprintID("unknown"), bptypes.ApproveBlueprintRequest{})

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}
