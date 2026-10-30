package renderapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testids"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func TestFakeRenderServerBlueprintPlan(t *testing.T) {
	t.Run("plans a sync for a known Blueprint", func(t *testing.T) {
		server, c := newServerAndClient(t)
		blueprint := server.Blueprints.Add(NewBlueprint(bptypes.Blueprint{}))

		resp, err := c.CreateBlueprintSyncWithResponse(context.Background(), blueprint.Id)

		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode())
		require.NotNil(t, resp.JSON201)
		require.Equal(t, bptypes.PlanningOutcomePlanned, resp.JSON201.PlanningOutcome)
		require.NotNil(t, resp.JSON201.Sync)
		require.Contains(t, resp.JSON201.Sync.DashboardUrl, blueprint.Id)
	})

	t.Run("returns 404 for an unknown Blueprint", func(t *testing.T) {
		_, c := newServerAndClient(t)

		resp, err := c.CreateBlueprintSyncWithResponse(context.Background(), testids.BlueprintID("unknown"))

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode())
	})

	t.Run("returns a queued no changes result as 200 for an unseeded Blueprint", func(t *testing.T) {
		server, c := newServerAndClient(t)
		noChanges := bptypes.CreateBlueprintSyncResponse{
			PlanningOutcome: bptypes.PlanningOutcomeNoChanges,
			Notices:         []bptypes.BlueprintNotice{},
		}
		server.Blueprints.RespondWithBodyForPlan(noChanges)

		resp, err := c.CreateBlueprintSyncWithResponse(context.Background(), testids.BlueprintID("unseeded"))

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.Equal(t, &noChanges, resp.JSON200)
	})
}
