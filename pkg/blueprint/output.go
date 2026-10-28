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

// NewCreateOut adds follow-up guidance without changing the REST response.
func NewCreateOut(result *bptypes.CreateBlueprintResponse) CreateOut {
	out := CreateOut{Data: result}
	var next string
	//exhaustive:enforce
	switch result.PlanningOutcome {
	case bptypes.PlanningOutcomePlanned:
		next = fmt.Sprintf("render ea blueprints approve %s", result.Blueprint.Id)
		if result.Sync != nil && len(result.Sync.RequiredManualEnvVars) > 0 {
			next = fmt.Sprintf("Create and populate manual-env-vars.json with the required environment variable values, then run %s --manual-env-vars-file manual-env-vars.json.", next)
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
