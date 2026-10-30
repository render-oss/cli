package blueprint

import (
	"fmt"

	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

// CreateOut preserves the REST create result, including a saved Blueprint when
// planning fails.
type CreateOut struct {
	Data *bptypes.CreateBlueprintResponse `json:"data"`
	Meta CreateOutMeta                    `json:"meta"`
}

type CreateOutMeta struct {
	Next *string `json:"next"`
}

// UpdateOut preserves the REST update result.
type UpdateOut struct {
	Data *bptypes.Blueprint `json:"data"`
	Meta UpdateOutMeta      `json:"meta"`
}

type UpdateOutMeta struct {
	Next *string `json:"next"`
}

// PlanOut preserves the REST plan result, including a failed planning attempt.
type PlanOut struct {
	Data *bptypes.CreateBlueprintSyncResponse `json:"data"`
	Meta PlanOutMeta                          `json:"meta"`
}

type PlanOutMeta struct {
	Next *string `json:"next"`
}

// NewCreateOut adds follow-up guidance without changing the REST response.
// manualEnvVarsPath is the prepared manual env var file, or empty if none was
// written. Callers handle file preparation errors before constructing output.
func NewCreateOut(result *bptypes.CreateBlueprintResponse, manualEnvVarsPath string) CreateOut {
	bp := result.Blueprint
	noChangesNext := fmt.Sprintf("Nothing to approve. If this is unexpected, update %q and push your changes to the remote %q branch, then re-plan with \"render ea blueprints plan %s\".", bp.Path, bp.Branch, bp.Id)
	return CreateOut{
		Data: result,
		Meta: CreateOutMeta{Next: planningNext(bp.Id, result.PlanningOutcome, manualEnvVarsPath, noChangesNext)},
	}
}

// NewPlanOut adds follow-up guidance without changing the REST response. The
// plan response does not include the Blueprint, so blueprintID comes from the
// command input. manualEnvVarsPath is as for NewCreateOut.
func NewPlanOut(blueprintID string, result *bptypes.CreateBlueprintSyncResponse, manualEnvVarsPath string) PlanOut {
	noChangesNext := fmt.Sprintf("Nothing to approve. If this is unexpected, push your Blueprint file changes to the Blueprint's branch, then re-plan with \"render ea blueprints plan %s\".", blueprintID)
	return PlanOut{
		Data: result,
		Meta: PlanOutMeta{Next: planningNext(blueprintID, result.PlanningOutcome, manualEnvVarsPath, noChangesNext)},
	}
}

// planningNext returns the next step after planning a sync, or nil for an
// unknown planning outcome. noChangesNext is the next step when no sync is needed.
func planningNext(blueprintID string, outcome bptypes.PlanningOutcome, manualEnvVarsPath, noChangesNext string) *string {
	var next string
	//exhaustive:enforce
	switch outcome {
	case bptypes.PlanningOutcomePlanned:
		next = fmt.Sprintf("render ea blueprints approve %s", blueprintID)
		if manualEnvVarsPath != "" {
			next = fmt.Sprintf("%s lists these environment variables with empty values. Set any values you want, then run \"%s\", which reads this file unless you pass --manual-env-vars-file.", manualEnvVarsPath, next)
		}
	case bptypes.PlanningOutcomeNoChanges:
		next = noChangesNext
	case bptypes.PlanningOutcomeFailed:
		next = fmt.Sprintf("fix the errors above, then run \"render ea blueprints plan %s\"", blueprintID)
	default:
		return nil
	}
	return &next
}

// ApproveOut preserves the REST approve result: the newly approved sync, or
// the pending or running sync it left unchanged.
type ApproveOut struct {
	Data *bptypes.ApprovedSync `json:"data"`
	Meta ApproveOutMeta        `json:"meta"`
}

type ApproveOutMeta struct {
	Next *string `json:"next"`
}

// NewApproveOut adds follow-up guidance without changing the REST response.
// Approve does not wait for the sync, so the next step is to check on the
// resources it changes.
func NewApproveOut(sync *bptypes.ApprovedSync) ApproveOut {
	next := "Render will start applying your planned changes. Monitor progress at the provided dashboard URL, or check the affected services with `render services` and `render deploys` commands."
	return ApproveOut{Data: sync, Meta: ApproveOutMeta{Next: &next}}
}
