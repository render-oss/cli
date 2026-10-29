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
	out := CreateOut{Data: result}
	var next string
	//exhaustive:enforce
	switch result.PlanningOutcome {
	case bptypes.PlanningOutcomePlanned:
		next = fmt.Sprintf("render ea blueprints approve %s", result.Blueprint.Id)
		if manualEnvVarsPath != "" {
			next = fmt.Sprintf("%s lists these environment variables with empty values. Set any values you want, then run %s, which reads this file unless you pass --manual-env-vars-file.", manualEnvVarsPath, next)
		}
	case bptypes.PlanningOutcomeNoChanges:
		next = fmt.Sprintf("Nothing to approve. If this is unexpected, update %q and push your changes to the remote %q branch, then re-plan with render ea blueprints plan %s.", result.Blueprint.Path, result.Blueprint.Branch, result.Blueprint.Id)
	case bptypes.PlanningOutcomeFailed:
		next = fmt.Sprintf("fix the errors above, then run render ea blueprints plan %s", result.Blueprint.Id)
	default:
		return out
	}
	out.Meta.Next = &next
	return out
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
