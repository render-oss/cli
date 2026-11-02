package renderapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testids"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func TestFakeRenderServerBlueprintUpdate(t *testing.T) {
	t.Run("updates only the supplied fields", func(t *testing.T) {
		server, c := newServerAndClient(t)
		blueprint := server.Blueprints.Add(NewBlueprint(bptypes.Blueprint{Name: "app", AutoSync: true}))
		want := *blueprint
		want.Branch = "release"
		want.AutoSync = false

		resp, err := c.UpdateBlueprintWithResponse(context.Background(), blueprint.Id, bptypes.BlueprintPATCH{
			Branch:   new("release"),
			AutoSync: new(false),
		})

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode())
		require.Equal(t, &want, resp.JSON200)
		require.Equal(t, &want, server.Blueprints.Only(t))
	})

	t.Run("returns 404 for an unknown Blueprint", func(t *testing.T) {
		_, c := newServerAndClient(t)

		resp, err := c.UpdateBlueprintWithResponse(context.Background(), testids.BlueprintID("unknown"), bptypes.BlueprintPATCH{Name: new("app")})

		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, resp.StatusCode())
	})
}
