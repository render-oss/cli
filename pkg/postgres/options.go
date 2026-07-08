package postgres

import (
	"slices"

	pgclient "github.com/render-oss/cli/pkg/client/postgres"
)

// legacyPostgresPlans are the deprecated plan names that predate the
// basic_*/pro_*gb/accelerated_* naming. They remain valid in the API schema
// (so PostgresPlansValues() still returns them) but are intentionally
// omitted from --plan help text.
var legacyPostgresPlans = []pgclient.PostgresPlans{
	pgclient.Starter,
	pgclient.Standard,
	pgclient.Pro,
	pgclient.ProPlus,
}

// ModernPlans lists the plan names suggested in --plan help text. It is derived
// from the generated pgclient.PostgresPlansValues() so newly added plans
// appear automatically, with the "custom" sentinel and the legacy plans filtered
// out. The API accepts additional account-specific plan names (custom plans); this
// list is for documentation only, not validation.
var ModernPlans = modernPlanNames()

func modernPlanNames() []string {
	plans := slices.DeleteFunc(pgclient.PostgresPlansValues(), func(p pgclient.PostgresPlans) bool {
		return p == pgclient.Custom || slices.Contains(legacyPostgresPlans, p)
	})
	out := make([]string, len(plans))
	for i, p := range plans {
		out[i] = string(p)
	}
	return out
}
