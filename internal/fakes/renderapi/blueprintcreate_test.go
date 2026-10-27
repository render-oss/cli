package renderapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/pointers"
)

func newBlueprintCreateFixture(t *testing.T) (*Server, *client.ClientWithResponses, bptypes.CreateBlueprintRequest) {
	t.Helper()
	server := NewServer(t)
	owner := server.Owners.Add(NewOwner(client.Owner{}))
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return server, c, bptypes.CreateBlueprintRequest{
		OwnerId: owner.Id, Name: "My App",
		Source: bptypes.BlueprintSource{Repo: "https://github.com/acme/app"},
	}
}

func TestFakeRenderServerBlueprintCreateEndpoint(t *testing.T) {
	t.Run("the fake POST /blueprints endpoint preserves supplied fields and defaults omitted fields", func(t *testing.T) {
		for _, tc := range []struct {
			name  string
			input bptypes.CreateBlueprintRequest
			want  bptypes.Blueprint
		}{
			{
				name: "request omits optional fields",
				input: bptypes.CreateBlueprintRequest{
					Name:   "My App",
					Source: bptypes.BlueprintSource{Repo: "https://github.com/acme/app"},
				},
				want: bptypes.Blueprint{
					Name:     "My App",
					Repo:     "https://github.com/acme/app",
					Branch:   "main",
					Path:     "render.yaml",
					AutoSync: true,
					Status:   bptypes.StatusCreated,
				},
			},
			{
				name: "request supplies all fields",
				input: bptypes.CreateBlueprintRequest{
					Name: "My App",
					Source: bptypes.BlueprintSource{
						Repo:   "https://github.com/acme/app",
						Branch: new("release"),
						Path:   new("infra/render.yaml"),
					},
					AutoSync: new(false),
				},
				want: bptypes.Blueprint{
					Name:     "My App",
					Repo:     "https://github.com/acme/app",
					Branch:   "release",
					Path:     "infra/render.yaml",
					AutoSync: false,
					Status:   bptypes.StatusCreated,
				},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				server, c, fixture := newBlueprintCreateFixture(t)
				input := tc.input
				input.OwnerId = fixture.OwnerId

				resp, err := c.CreateBlueprintWithResponse(context.Background(), input)
				require.NoError(t, err)
				require.Equal(t, http.StatusCreated, resp.StatusCode())

				result := resp.JSON201
				require.NotNil(t, result)
				require.Equal(t, bptypes.PlanningOutcomePlanned, result.PlanningOutcome)

				require.Equal(t, tc.want.Name, result.Blueprint.Name)
				require.Equal(t, tc.want.Repo, result.Blueprint.Repo)
				require.Equal(t, tc.want.Branch, result.Blueprint.Branch)
				require.Equal(t, tc.want.Path, result.Blueprint.Path)
				require.Equal(t, tc.want.AutoSync, result.Blueprint.AutoSync)
				require.Equal(t, tc.want.Status, result.Blueprint.Status)
				require.Equal(t, result.Blueprint, *server.Blueprints.Only(t))

				require.NotNil(t, result.Sync)
				require.Equal(t, bptypes.SyncStateCreated, result.Sync.State)
				require.Contains(t, result.Sync.DashboardUrl, result.Blueprint.Id)
				require.Contains(t, result.Sync.DashboardUrl, result.Sync.Id)
				require.NotNil(t, result.Sync.RawPlan)
				require.NotNil(t, result.Sync.RequiredManualEnvVars)

				require.NotNil(t, result.Notices)

				require.Len(t, server.Requests, 1)
				require.Equal(t, http.MethodPost, server.Requests[0].Method)
				require.Equal(t, "/blueprints", server.Requests[0].URI)

				var recorded bptypes.CreateBlueprintRequest
				require.NoError(t, json.Unmarshal(server.Requests[0].Body, &recorded))
				require.Equal(t, input, recorded)
			})
		}
	})
}

func TestFakeRenderServerBlueprintCreateQueuedResponses(t *testing.T) {
	t.Run("the fake POST /blueprints endpoint returns the queued response body and saves a new blueprint", func(t *testing.T) {
		server, c, body := newBlueprintCreateFixture(t)
		response := bptypes.CreateBlueprintResponse{
			Blueprint: *NewBlueprint(bptypes.Blueprint{
				Name: body.Name,
				Repo: body.Source.Repo,
			}),
			PlanningOutcome: bptypes.PlanningOutcomePlanned,
			Sync: &bptypes.SyncWithPlan{
				Id:      testids.SyncID("planned"),
				State:   bptypes.SyncStateCreated,
				RawPlan: new(map[string]interface{}{"actions": []interface{}{"create service"}}),
				RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{{
					ServiceType: bptypes.ManualEnvVarTargetTypeWeb,
					ServiceName: "app",
					Key:         "API_KEY",
				}},
			},
			Notices: []bptypes.BlueprintNotice{{
				Severity: bptypes.BlueprintNoticeSeverityWarning,
				Message:  "review the plan",
				Path:     new("services[0].plan"),
				Line:     new(7),
				Column:   new(9),
			}},
		}
		server.Blueprints.RespondWithBodyForCreate(response)

		resp, err := c.CreateBlueprintWithResponse(context.Background(), body)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, resp.StatusCode())

		require.Equal(t, &response, resp.JSON201)

		require.Equal(t, response.Blueprint, *server.Blueprints.Only(t))
	})

	t.Run("the fake POST /blueprints endpoint consumes queued responses in order before generating a default response", func(t *testing.T) {
		server, c, body := newBlueprintCreateFixture(t)
		responses := []bptypes.CreateBlueprintResponse{
			{
				Blueprint:       *NewBlueprint(bptypes.Blueprint{Name: "First Blueprint"}),
				PlanningOutcome: bptypes.PlanningOutcomeFailed,
			},
			{
				Blueprint:       *NewBlueprint(bptypes.Blueprint{Name: "Second Blueprint"}),
				PlanningOutcome: bptypes.PlanningOutcomeNoChanges,
			},
		}
		for _, response := range responses {
			server.Blueprints.RespondWithBodyForCreate(response)
		}

		for _, response := range responses {
			resp, err := c.CreateBlueprintWithResponse(context.Background(), body)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp.StatusCode())
			require.Equal(t, &response, resp.JSON201)
		}

		next, err := c.CreateBlueprintWithResponse(context.Background(), body)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, next.StatusCode())
		require.NotNil(t, next.JSON201)

		require.Equal(t, bptypes.PlanningOutcomePlanned, next.JSON201.PlanningOutcome)
		for _, response := range responses {
			require.NotEqual(t, response.Blueprint.Id, next.JSON201.Blueprint.Id)
		}

		require.Len(t, server.Blueprints.Instances, 3)
	})

	t.Run("the fake POST /blueprints endpoint ignores decoded request fields while responses are queued", func(t *testing.T) {
		server, c, body := newBlueprintCreateFixture(t)
		body.OwnerId = testids.WorkspaceID("missing")
		body.Name = ""
		body.Source.Repo = ""
		body.ExistingResources = new(bptypes.ExistingResourceMode("invalid"))
		response := bptypes.CreateBlueprintResponse{
			Blueprint:       *NewBlueprint(bptypes.Blueprint{Name: "Queued Blueprint"}),
			PlanningOutcome: bptypes.PlanningOutcomePlanned,
		}
		server.Blueprints.RespondWithBodyForCreate(response)
		server.Blueprints.RespondWithErrorForCreate(http.StatusConflict, "queued conflict")

		first, err := c.CreateBlueprintWithResponse(context.Background(), body)
		require.NoError(t, err)
		require.Equal(t, http.StatusCreated, first.StatusCode())
		require.Equal(t, &response, first.JSON201)
		require.Equal(t, response.Blueprint, *server.Blueprints.Only(t))

		second, err := c.CreateBlueprintWithResponse(context.Background(), body)
		require.NoError(t, err)
		require.Equal(t, http.StatusConflict, second.StatusCode())
		require.NotNil(t, second.JSON409)
		require.Equal(t, "queued conflict", pointers.StringValue(second.JSON409.Message))

		third, err := c.CreateBlueprintWithResponse(context.Background(), body)
		require.NoError(t, err)
		require.Equal(t, http.StatusNotFound, third.StatusCode())
		require.Equal(t, response.Blueprint, *server.Blueprints.Only(t))
	})
}

func TestFakeRenderServerBlueprintCreateError(t *testing.T) {
	server, c, body := newBlueprintCreateFixture(t)
	message := "A Blueprint already uses this name. Pass a different name."
	server.Blueprints.RespondWithErrorForCreate(http.StatusConflict, message)

	resp, err := c.CreateBlueprintWithResponse(context.Background(), body)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp.StatusCode())

	require.NotNil(t, resp.JSON409)
	require.Equal(t, message, pointers.StringValue(resp.JSON409.Message))

	require.Empty(t, server.Blueprints.Instances)
}

func TestFakeRenderServerBlueprintCreateRejectsInvalidRequests(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   bptypes.CreateBlueprintRequest
		message string
	}{
		{
			name: "missing name",
			input: bptypes.CreateBlueprintRequest{
				Source: bptypes.BlueprintSource{Repo: "https://github.com/acme/app"},
			},
			message: "name is required",
		},
		{
			name:    "missing repository",
			input:   bptypes.CreateBlueprintRequest{Name: "My App"},
			message: "source.repo is required",
		},
		{
			name: "CLI spelling of existing resources mode",
			input: bptypes.CreateBlueprintRequest{
				Name:              "My App",
				Source:            bptypes.BlueprintSource{Repo: "https://github.com/acme/app"},
				ExistingResources: new(bptypes.ExistingResourceMode("create-new")),
			},
			message: "existingResources must be adopt or create_new",
		},
		{
			name: "explicit empty existing resources mode",
			input: bptypes.CreateBlueprintRequest{
				Name:              "My App",
				Source:            bptypes.BlueprintSource{Repo: "https://github.com/acme/app"},
				ExistingResources: new(bptypes.ExistingResourceMode("")),
			},
			message: "existingResources must be adopt or create_new",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, c, fixture := newBlueprintCreateFixture(t)
			input := tc.input
			input.OwnerId = fixture.OwnerId

			resp, err := c.CreateBlueprintWithResponse(context.Background(), input)
			require.NoError(t, err)
			require.Equal(t, http.StatusBadRequest, resp.StatusCode())
			require.NotNil(t, resp.JSON400)
			require.Equal(t, tc.message, pointers.StringValue(resp.JSON400.Message))
			require.Empty(t, server.Blueprints.Instances)
		})
	}
}

func TestFakeRenderServerBlueprintCreateRejectsUnknownOwner(t *testing.T) {
	server, c, body := newBlueprintCreateFixture(t)
	body.OwnerId = testids.WorkspaceID("missing")

	resp, err := c.CreateBlueprintWithResponse(context.Background(), body)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode())
	require.NotNil(t, resp.JSON404)
	require.Equal(t, "workspace not found", pointers.StringValue(resp.JSON404.Message))
	require.Empty(t, server.Blueprints.Instances)
}

func TestFakeRenderServerBlueprintCreateAcceptsExistingResourceModes(t *testing.T) {
	for _, mode := range []bptypes.ExistingResourceMode{bptypes.AdoptExisting, bptypes.CreateNew} {
		t.Run(string(mode), func(t *testing.T) {
			server, c, body := newBlueprintCreateFixture(t)
			body.ExistingResources = &mode

			resp, err := c.CreateBlueprintWithResponse(context.Background(), body)
			require.NoError(t, err)
			require.Equal(t, http.StatusCreated, resp.StatusCode())
			require.NotNil(t, resp.JSON201)
			require.Equal(t, resp.JSON201.Blueprint, *server.Blueprints.Only(t))
		})
	}
}
