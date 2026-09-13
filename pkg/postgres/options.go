package postgres

import (
	"regexp"
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

// specPlanName matches the spec-based Postgres plan names, e.g. "0.1c-256mb" or "4c-16g".
var specPlanName = regexp.MustCompile(`^[0-9.]+c-[0-9.]+(mb|g)$`)

// ModernPlans lists the plan names suggested in --plan help text and offered in
// the interactive picker. It is derived from the generated
// pgclient.PostgresPlansValues() so newly added plans appear automatically,
// narrowed to the spec-based names (plus "free") and with the
// "custom" sentinel and the legacy plans filtered out. Names the CLI does not
// advertise remain valid --plan input that passes through to the API unchanged.
// The API also accepts account-specific plan names (custom plans); this list is
// for documentation only, not validation.
var ModernPlans = modernPlanNames()

func modernPlanNames() []string {
	plans := slices.DeleteFunc(pgclient.PostgresPlansValues(), func(p pgclient.PostgresPlans) bool {
		if p == pgclient.Custom || slices.Contains(legacyPostgresPlans, p) {
			return true
		}
		return !specPlanName.MatchString(string(p)) && p != pgclient.Free
	})
	out := make([]string, len(plans))
	for i, p := range plans {
		out[i] = string(p)
	}
	return out
}
