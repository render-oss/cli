package text_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/blueprint"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/text"
)

func TestBlueprintCreatePrintsPlanBeforeSummary(t *testing.T) {
	result := bptypes.CreateBlueprintResponse{
		Blueprint: bptypes.Blueprint{
			Id:     "exs-example",
			Name:   "app",
			Repo:   "https://github.com/acme/app",
			Branch: "main",
			Path:   "render.yaml",
		},
		PlanningOutcome: bptypes.PlanningOutcomePlanned,
		Sync: &bptypes.SyncWithPlan{
			Id:           "exe-example",
			State:        bptypes.SyncStateCreated,
			DashboardUrl: "https://dashboard.render.com/review",
			RawPlan:      new(map[string]interface{}{"actions": []interface{}{"create service with 100% rollout"}}),
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

	out := text.BlueprintCreate(blueprint.NewCreateOut(&result, "/home/user/.render/state/manual-env-vars/exs-example.json"))
	require.Equal(t, `Plan
These are the changes that will be applied if this sync is approved.

{
  "actions": [
    "create service with 100% rollout"
  ]
}

Blueprint app (exs-example)
Source: https://github.com/acme/app
Branch: main
Path: render.yaml
Sync: exe-example

Planned, waiting for approval

  Warning: review the plan (services[0].plan, line 7, column 9)

Environment variables required at approval:
  web/app: API_KEY

Review in Dashboard (optional): https://dashboard.render.com/review

Next:
  /home/user/.render/state/manual-env-vars/exs-example.json lists these environment variables with empty values. Set any values you want, then run render ea blueprints approve exs-example, which reads this file unless you pass --manual-env-vars-file.
`, out)
}

func TestBlueprintCreateWithoutPlan(t *testing.T) {
	t.Run("summarizes outcomes that have no raw plan to print", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			response bptypes.CreateBlueprintResponse
			want     string
		}{
			{
				name: "no changes",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomeNoChanges,
				},
				want: "\nNo changes: the Blueprint already matches its configuration\n\nNext:\n  Nothing to approve. If this is unexpected, update \"render.yaml\" and push your changes to the remote \"main\" branch, then re-plan with render ea blueprints plan exs-example.\n",
			},
			{
				name: "unknown planning outcome preserves details without a next step",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcome("future_outcome"),
					Sync: &bptypes.SyncWithPlan{
						Id:           "exe-example",
						DashboardUrl: "https://dashboard.render.com/review",
					},
					Notices: []bptypes.BlueprintNotice{{
						Severity: bptypes.BlueprintNoticeSeverityWarning,
						Message:  "review the sync",
					}},
				},
				want: "Sync: exe-example\n\nUnknown planning outcome: \"future_outcome\"\nThe Blueprint was created, but this CLI cannot determine the next step.\n\n  Warning: review the sync\n\nReview in Dashboard (optional): https://dashboard.render.com/review\n",
			},
			{
				name: "planning failed without a saved sync",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomeFailed,
					Notices: []bptypes.BlueprintNotice{{
						Severity: bptypes.BlueprintNoticeSeverityError,
						Message:  "invalid plan",
					}},
				},
				want: "\nPlanning failed\n\n  Error: invalid plan\n\nNext:\n  fix the errors above, then run render ea blueprints plan exs-example\n",
			},
			{
				name: "planning failed with a saved sync",
				response: bptypes.CreateBlueprintResponse{
					PlanningOutcome: bptypes.PlanningOutcomeFailed,
					Sync: &bptypes.SyncWithPlan{
						Id:           "exe-failed",
						State:        bptypes.SyncStateError,
						DashboardUrl: "https://dashboard.render.com/review",
					},
					Notices: []bptypes.BlueprintNotice{{
						Severity: bptypes.BlueprintNoticeSeverityError,
						Message:  "invalid plan",
					}},
				},
				want: "Sync: exe-failed\n\nPlanning failed\n\n  Error: invalid plan\n\nReview in Dashboard (optional): https://dashboard.render.com/review\n\nNext:\n  fix the errors above, then run render ea blueprints plan exs-example\n",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				result := tc.response
				result.Blueprint = bptypes.Blueprint{
					Id:     "exs-example",
					Name:   "app",
					Repo:   "https://github.com/acme/app",
					Branch: "main",
					Path:   "render.yaml",
				}

				out := text.BlueprintCreate(blueprint.NewCreateOut(&result, ""))
				require.Equal(t, "Blueprint app (exs-example)\nSource: https://github.com/acme/app\nBranch: main\nPath: render.yaml\n"+tc.want, out)
			})
		}
	})
}

// TestBlueprintNoticePositions verifies that zero and negative source positions
// are omitted from notices without hiding a valid line or column alongside them.
func TestBlueprintNoticePositions(t *testing.T) {
	for _, tc := range []struct {
		name   string
		line   int
		column int
		want   string
	}{
		{name: "zero positions", want: "Error: invalid plan\n"},
		{name: "negative positions", line: -1, column: -2, want: "Error: invalid plan\n"},
		{name: "positive line with zero column", line: 7, want: "Error: invalid plan (line 7)\n"},
		{name: "positive column with zero line", column: 9, want: "Error: invalid plan (column 9)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := bptypes.CreateBlueprintResponse{
				PlanningOutcome: bptypes.PlanningOutcomeFailed,
				Notices: []bptypes.BlueprintNotice{{
					Severity: bptypes.BlueprintNoticeSeverityError,
					Message:  "invalid plan",
					Line:     &tc.line,
					Column:   &tc.column,
				}},
			}
			out := text.BlueprintCreate(blueprint.NewCreateOut(&result, ""))
			require.Contains(t, out, tc.want)
		})
	}
}

func TestBlueprintApprove(t *testing.T) {
	next := "Render will start applying your planned changes. Monitor progress at the provided dashboard URL, or check the affected services with `render services` and `render deploys` commands."
	for _, tc := range []struct {
		name string
		sync bptypes.ApprovedSync
		want string
	}{
		{
			name: "pending sync with a Dashboard URL",
			sync: bptypes.ApprovedSync{
				Id:           "exe-example",
				Commit:       bptypes.CommitRef{Id: "abc123"},
				State:        bptypes.SyncStatePending,
				DashboardUrl: "https://dashboard.render.com/sync",
			},
			want: "Sync exe-example\nCommit: abc123\nState: pending\n\nApproved\n\nTrack in Dashboard: https://dashboard.render.com/sync\n\nNext:\n  " + next + "\n",
		},
		{
			name: "running sync shows its state",
			sync: bptypes.ApprovedSync{
				Id:           "exe-example",
				Commit:       bptypes.CommitRef{Id: "abc123"},
				State:        bptypes.SyncStateRunning,
				DashboardUrl: "https://dashboard.render.com/sync",
			},
			want: "Sync exe-example\nCommit: abc123\nState: running\n\nApproved\n\nTrack in Dashboard: https://dashboard.render.com/sync\n\nNext:\n  " + next + "\n",
		},
		{
			name: "no Dashboard URL omits the Dashboard line",
			sync: bptypes.ApprovedSync{
				Id:     "exe-example",
				Commit: bptypes.CommitRef{Id: "abc123"},
				State:  bptypes.SyncStatePending,
			},
			want: "Sync exe-example\nCommit: abc123\nState: pending\n\nApproved\n\nNext:\n  " + next + "\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := text.BlueprintApprove(blueprint.NewApproveOut(&tc.sync))

			require.Equal(t, tc.want, out)
		})
	}
}
