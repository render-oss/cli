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

// UpdateOut combines the REST update result with the result of planning a
// sync, when one was requested.
type UpdateOut struct {
	Data *UpdateResult `json:"data"`
	Meta UpdateOutMeta `json:"meta"`
}

// UpdateResult is the updated Blueprint and, when the update requested a sync,
// the planning result. PlanningOutcome is nil when no sync was requested; Sync
// is then nil and Notices is empty, which does not mean there were no changes.
type UpdateResult struct {
	Blueprint       bptypes.Blueprint         `json:"blueprint"`
	PlanningOutcome *bptypes.PlanningOutcome  `json:"planningOutcome,omitempty"`
	Sync            *bptypes.SyncWithPlan     `json:"sync"`
	Notices         []bptypes.BlueprintNotice `json:"notices"`
}

type UpdateOutMeta struct {
	Next *string `json:"next"`
	// SyncRequested reports whether the update asked Render to plan a sync
	// (--branch or --path was supplied). It is true even when planning failed or
	// found no changes; Data.PlanningOutcome holds the result.
	SyncRequested bool `json:"syncRequested"`
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
	next := planningNext(blueprintID, result.PlanningOutcome, manualEnvVarsPath, replanNoChangesNext(blueprintID))
	if result.PlanningOutcome == bptypes.PlanningOutcomeFailed && result.Sync == nil {
		// A failed plan without a saved sync leaves any older planned sync as the
		// newest one, so approve could still apply it. A saved failed sync is the
		// newest one instead, and approve refuses it.
		next = new(*next + ". Do not approve until re-planning succeeds. If you previously planned changes that are still waiting for approval, approving now could apply that earlier plan.")
	}
	return PlanOut{
		Data: result,
		Meta: PlanOutMeta{Next: next},
	}
}

// NewUpdateOut adds follow-up guidance without changing the update result.
// There is no next step when no sync was requested. manualEnvVarsPath is as
// for NewCreateOut.
func NewUpdateOut(result *UpdateResult, manualEnvVarsPath string) UpdateOut {
	if result.PlanningOutcome == nil {
		return UpdateOut{Data: result}
	}
	bp := result.Blueprint
	next := planningNext(bp.Id, *result.PlanningOutcome, manualEnvVarsPath, replanNoChangesNext(bp.Id))
	if *result.PlanningOutcome == bptypes.PlanningOutcomeFailed && result.Sync == nil {
		// Updating does not change a sync that is waiting for approval, so
		// approve could still apply a sync planned from the previous branch or
		// path. A failed sync saved by this re-plan is the newest one instead,
		// and approve refuses it, so the warning applies only when none was saved.
		next = new(*next + ". Do not approve until re-planning succeeds. If you previously planned changes that are still waiting for approval, approving now could apply that earlier plan.")
	}
	return UpdateOut{
		Data: result,
		Meta: UpdateOutMeta{Next: next, SyncRequested: true},
	}
}

// replanNoChangesNext is the next step when re-planning an existing Blueprint
// finds no changes. A plan with no changes is not saved, so an older planned
// sync can still be the newest one, and approve would apply it.
func replanNoChangesNext(blueprintID string) string {
	return fmt.Sprintf("No changes to approve. No further action is needed for this result. If you previously planned changes that are still waiting for approval, running \"render ea blueprints approve %s\" now will approve that earlier plan. If you expected changes from this run, push your Blueprint file changes to the Blueprint's branch, then re-plan with \"render ea blueprints plan %s\".", blueprintID, blueprintID)
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
