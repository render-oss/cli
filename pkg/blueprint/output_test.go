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
	out := CreateOut{Data: &result, Meta: CreateOutMeta{Next: new("fix the errors, then run render ea blueprints plan exs-example")}}
	encoded, err := json.Marshal(out)
	require.NoError(t, err)
	require.JSONEq(t, `{
  "data": {
    "blueprint": {"id":"exs-example","name":"app","status":"created","repo":"https://github.com/acme/app","branch":"main","path":"render.yaml","autoSync":true},
    "planningOutcome": "failed",
    "sync": {"id":"exe-example","state":"error","commit":{"id":"abc123"},"dashboardUrl":"https://dashboard.render.com/review","rawPlan":{"actions":["create service"]},"requiredManualEnvVars":[]},
    "notices": [{"severity":"error","message":"invalid plan"}]
  },
  "meta": {"next":"fix the errors, then run render ea blueprints plan exs-example"}
}`, string(encoded))
}
