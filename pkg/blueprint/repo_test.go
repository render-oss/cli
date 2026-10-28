package blueprint_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/blueprint"
	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func newBlueprintRepoFixture(t *testing.T) (*blueprint.Repo, *renderapi.Server, bptypes.CreateBlueprintRequest) {
	t.Helper()
	server := renderapi.NewServer(t)
	owner := server.Owners.Add(renderapi.NewOwner(client.Owner{}))
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return blueprint.NewRepo(c), server, bptypes.CreateBlueprintRequest{
		OwnerId: owner.Id,
		Name:    "app",
		Source:  bptypes.BlueprintSource{Repo: "https://github.com/acme/app"},
	}
}

func TestRepoCreateReturnsAPIResponse(t *testing.T) {
	repo, server, body := newBlueprintRepoFixture(t)
	response := bptypes.CreateBlueprintResponse{
		Blueprint:       *renderapi.NewBlueprint(bptypes.Blueprint{Name: "saved app"}),
		PlanningOutcome: bptypes.PlanningOutcomeFailed,
		Sync: &bptypes.SyncWithPlan{
			Id:           testids.SyncID("failed"),
			State:        bptypes.SyncStateError,
			DashboardUrl: "https://dashboard.render.com/review",
			RawPlan:      new(map[string]interface{}{"actions": []interface{}{"create service"}}),
			RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{{
				ServiceType: bptypes.ManualEnvVarTargetTypeWeb,
				ServiceName: "app",
				Key:         "API_KEY",
			}},
		},
		Notices: []bptypes.BlueprintNotice{{
			Severity: bptypes.BlueprintNoticeSeverityError,
			Message:  "invalid plan",
			Path:     new("services[0].plan"),
			Line:     new(7),
			Column:   new(9),
		}},
	}
	server.Blueprints.RespondWithBodyForCreate(response)

	result, err := repo.Create(context.Background(), body)
	require.NoError(t, err)
	require.Equal(t, &response, result)
}

func TestRepoCreateReturnsHTTPError(t *testing.T) {
	repo, server, body := newBlueprintRepoFixture(t)
	message := "Pass a different Blueprint name."
	server.Blueprints.RespondWithErrorForCreate(http.StatusConflict, message)

	result, err := repo.Create(context.Background(), body)
	require.ErrorContains(t, err, message)
	require.Nil(t, result)
}

func TestRepoCreatePropagatesCancellation(t *testing.T) {
	repo, server, body := newBlueprintRepoFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := repo.Create(ctx, body)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	require.Empty(t, server.Requests)
}
