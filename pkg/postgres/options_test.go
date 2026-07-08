package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/postgres"
)

// legacyAndCustom are the plan values intentionally omitted from --plan help
// text: the "custom" sentinel and the deprecated bare-tier names.
var legacyAndCustom = map[pgclient.PostgresPlans]bool{
	pgclient.Custom:   true,
	pgclient.Starter:  true,
	pgclient.Standard: true,
	pgclient.Pro:      true,
	pgclient.ProPlus:  true,
}

// TestModernPlansMatchesSchema is a completeness guard: every modern plan in the
// generated schema must appear in ModernPlans (so a newly added plan can't
// silently drop out of help text), and every excluded value (custom + legacy
// tiers) must stay out.
func TestModernPlansMatchesSchema(t *testing.T) {
	for _, p := range pgclient.PostgresPlansValues() {
		if legacyAndCustom[p] {
			assert.NotContains(t, postgres.ModernPlans, string(p),
				"%q is legacy/custom and should be excluded from --plan help", p)
		} else {
			assert.Contains(t, postgres.ModernPlans, string(p),
				"%q is a modern plan and should appear in --plan help", p)
		}
	}
}

// TestModernPlansAreValidSchemaValues ensures help text never advertises a name
// the API would reject, and that ModernPlans has no duplicates.
func TestModernPlansAreValidSchemaValues(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range postgres.ModernPlans {
		assert.True(t, pgclient.PostgresPlans(name).Valid(),
			"%q in ModernPlans is not a valid PostgresPlans value", name)
		assert.False(t, seen[name], "duplicate plan %q in ModernPlans", name)
		seen[name] = true
	}
}

// TestModernPlansHaveMetadata is a completeness guard: every plan offered in the
// interactive picker (ModernPlans) must carry a display label in the metadata
// table, so a newly added plan can't silently ship without one. HA eligibility
// has no "missing" state (the zero value is a valid "not eligible"), so it can't
// be guarded the same way.
func TestModernPlansHaveMetadata(t *testing.T) {
	for _, plan := range postgres.ModernPlans {
		assert.NotEqual(t, plan, postgres.PlanLabel(plan),
			"plan %q has no metadata entry (PlanLabel falls back to the raw value)", plan)
	}
}
