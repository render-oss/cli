package postgres_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/postgres"
)

// legacyAndCustom are the plan values intentionally omitted from --plan help
// text: the "custom" sentinel and the deprecated bare plan names.
var legacyAndCustom = map[pgclient.PostgresPlans]bool{
	pgclient.Custom:   true,
	pgclient.Starter:  true,
	pgclient.Standard: true,
	pgclient.Pro:      true,
	pgclient.ProPlus:  true,
}

// specPlanName matches a spec-based plan name ({cpu}c-{ram}).
var specPlanName = regexp.MustCompile(`^[0-9.]+c-[0-9.]+(mb|g)$`)

// plansWithoutHighAvailability are the only plans that do not support high
// availability. Naming the exceptions rather than restating every plan's flag
// keeps this test independent of the metadata it checks.
var plansWithoutHighAvailability = []string{
	string(pgclient.Free),
	"0.1c-256mb",
	"0.5c-1g",
	string(pgclient.Basic256mb),
	string(pgclient.Basic1gb),
}

func supportsHighAvailability(plan string) bool {
	return !slices.Contains(plansWithoutHighAvailability, plan)
}

// TestModernPlansAdvertisesSpecPlanNames pins the rule that decides help text:
// every spec-based name in the schema is advertised, free is advertised because
// it has no spec-based name, and everything else is left out. A plan added to
// the schema is therefore covered without listing it here.
func TestModernPlansAdvertisesSpecPlanNames(t *testing.T) {
	for _, p := range pgclient.PostgresPlansValues() {
		advertised := !legacyAndCustom[p] &&
			(p == pgclient.Free || specPlanName.MatchString(string(p)))

		if advertised {
			assert.Contains(t, postgres.ModernPlans, string(p),
				"%q should appear in --plan help", p)
		} else {
			assert.NotContains(t, postgres.ModernPlans, string(p),
				"%q should be excluded from --plan help", p)
		}
	}
}

// TestModernPlansAreValidAndUnique ensures help text never advertises a name the
// API would reject, and lists nothing twice.
func TestModernPlansAreValidAndUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, name := range postgres.ModernPlans {
		assert.True(t, pgclient.PostgresPlans(name).Valid(),
			"%q in ModernPlans is not a valid PostgresPlans value", name)
		assert.False(t, seen[name], "duplicate plan %q in ModernPlans", name)
		seen[name] = true
	}
}

// TestModernPlansHaveMetadata guards the interactive picker: every advertised
// plan carries a label, and its HA eligibility matches haIneligible, so a newly
// added plan cannot ship unlabeled or silently treated as HA-ineligible.
func TestModernPlansHaveMetadata(t *testing.T) {
	for _, name := range postgres.ModernPlans {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, postgres.PlanLabel(name), "%q has no picker label", name)
			if name == string(pgclient.Free) {
				assert.Equal(t, "Free", postgres.PlanLabel(name))
			}
			if specPlanName.MatchString(name) {
				assert.Equal(t, name, postgres.PlanLabel(name),
					"a spec-based plan labels itself")
			}
			assert.Equal(t, supportsHighAvailability(name), postgres.IsHAEligible(name),
				"unexpected HA eligibility for %q", name)
		})
	}
}

// TestUnadvertisedPlansKeepTheirMetadata covers the older names: they no longer
// appear in help text or the picker, but a user can still pass one to --plan, so
// they keep the label the confirmations echo back and their own HA eligibility.
func TestUnadvertisedPlansKeepTheirMetadata(t *testing.T) {
	for _, p := range pgclient.PostgresPlansValues() {
		if legacyAndCustom[p] || p == pgclient.Free || specPlanName.MatchString(string(p)) {
			continue
		}

		t.Run(string(p), func(t *testing.T) {
			assert.NotContains(t, postgres.ModernPlans, string(p),
				"%q should be excluded from --plan help", p)
			assert.NotEqual(t, string(p), postgres.PlanLabel(string(p)),
				"%q should keep a human-facing label", p)
			assert.Equal(t, supportsHighAvailability(string(p)), postgres.IsHAEligible(string(p)),
				"unexpected HA eligibility for %q", p)
		})
	}
}
