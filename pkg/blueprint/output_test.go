package blueprint

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

func TestCreateOutPreservesPlanningFailureData(t *testing.T) {
	result := bptypes.CreateBlueprintResponse{
		Blueprint: bptypes.Blueprint{
			Id: "exs-example", Name: "app", Status: bptypes.StatusCreated,
			Repo: "https://github.com/acme/app", Branch: "main", Path: "render.yaml", AutoSync: true,
		},
		PlanningOutcome: bptypes.PlanningOutcomeFailed,
		Sync: &bptypes.SyncWithPlan{
			Id: "exe-example", State: bptypes.SyncStateError,
			Commit: bptypes.CommitRef{Id: "abc123"}, DashboardUrl: "https://dashboard.render.com/review",
			RawPlan:               new(map[string]interface{}{"actions": []interface{}{"create service"}}),
			RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{},
		},
		Notices: []bptypes.BlueprintNotice{{Severity: bptypes.BlueprintNoticeSeverityError, Message: "invalid plan"}},
	}
	out := NewCreateOut(&result)
	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	require.JSONEq(t, `{
  "data": {
    "blueprint": {"id":"exs-example","name":"app","status":"created","repo":"https://github.com/acme/app","branch":"main","path":"render.yaml","autoSync":true},
    "planningOutcome": "failed",
    "sync": {"id":"exe-example","state":"error","commit":{"id":"abc123"},"dashboardUrl":"https://dashboard.render.com/review","rawPlan":{"actions":["create service"]},"requiredManualEnvVars":[]},
    "notices": [{"severity":"error","message":"invalid plan"}]
  },
  "meta": {"next":"fix the errors above, then run render ea blueprints plan exs-example"}
}`, string(encoded))
}

// TestCreateOutUnknownPlanningOutcome verifies that an unknown PlanningOutcome
// enum value is preserved in the API result, with no next-step guidance.
func TestCreateOutUnknownPlanningOutcome(t *testing.T) {
	result := &bptypes.CreateBlueprintResponse{PlanningOutcome: bptypes.PlanningOutcome("future_outcome")}
	out := NewCreateOut(result)
	require.Same(t, result, out.Data)
	require.Nil(t, out.Meta.Next)
}

func TestNewCreateOutNext(t *testing.T) {
	t.Run("chooses follow-up guidance from the server planning outcome", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			response bptypes.CreateBlueprintResponse
			next     string
		}{
			{
				name: "planned",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomePlanned,
					Sync:            &bptypes.SyncWithPlan{State: bptypes.SyncStateCreated},
				},
				next: "render ea blueprints approve exs-example",
			},
			{
				name: "planned with manual env vars",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomePlanned,
					Sync: &bptypes.SyncWithPlan{
						State: bptypes.SyncStateCreated,
						RequiredManualEnvVars: []bptypes.RequiredManualEnvVar{{
							ServiceType: bptypes.ManualEnvVarTargetTypeWeb,
							ServiceName: "app",
							Key:         "API_KEY",
						}},
					},
				},
				next: "Create and populate manual-env-vars.json with the required environment variable values, then run render ea blueprints approve exs-example --manual-env-vars-file manual-env-vars.json.",
			},
			{
				name: "no changes",
				response: bptypes.CreateBlueprintResponse{
					Blueprint:       bptypes.Blueprint{Path: "infra/render.yaml", Branch: "release"},
					PlanningOutcome: bptypes.PlanningOutcomeNoChanges,
				},
				next: "Nothing to approve. If this is unexpected, update \"infra/render.yaml\" and push your changes to the remote \"release\" branch, then re-plan with render ea blueprints plan exs-example.",
			},
			{
				name: "planning failed despite a saved sync",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomeFailed,
					Sync:            &bptypes.SyncWithPlan{State: bptypes.SyncStateError},
				},
				next: "fix the errors above, then run render ea blueprints plan exs-example",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				result := tc.response
				result.Blueprint.Id = "exs-example"

				out := NewCreateOut(&result)
				require.Equal(t, new(tc.next), out.Meta.Next)
			})
		}
	})
}
