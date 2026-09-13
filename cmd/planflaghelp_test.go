package cmd

import (
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/keyvalue"
	"github.com/render-oss/cli/pkg/postgres"
)

// plansInPlanFlagHelp returns the plan names a command actually offers in its
// --plan help text, parsed back out of the rendered usage string. Reading the
// real flag usage keeps these assertions about what a user sees when they run
// `--help`, rather than about the slice the usage string is built from.
func plansInPlanFlagHelp(t *testing.T, cmd *cobra.Command) []string {
	t.Helper()

	flag := cmd.Flags().Lookup("plan")
	require.NotNil(t, flag, "%s has no --plan flag", cmd.Name())

	_, list, found := strings.Cut(flag.Usage, "one of: ")
	require.True(t, found, "unexpected --plan usage text: %q", flag.Usage)
	list, _, found = strings.Cut(list, ". Custom")
	require.True(t, found, "unexpected --plan usage text: %q", flag.Usage)

	return strings.Split(list, " | ")
}

// advertisedFor returns the advertised plan list backing a command's --plan help.
func advertisedFor(cmdName string) []string {
	if strings.HasPrefix(cmdName, "kv ") {
		return keyvalue.PlanValues()
	}
	return postgres.ModernPlans
}

func planFlagCommands() map[string]*cobra.Command {
	return map[string]*cobra.Command{
		"pg create": newPgCreateCmd(nil),
		"pg update": newPgUpdateCmd(nil),
		"kv create": newKVCreateCmd(nil),
		"kv update": newKVUpdateCmd(nil),
	}
}

// TestPlanFlagHelpOffersTheAdvertisedPlans checks the help a user actually sees
// against the advertised plan lists, so help text cannot drift from them (by
// being hardcoded, say). Which plans belong on those lists is covered by the
// postgres and keyvalue package tests.
func TestPlanFlagHelpOffersTheAdvertisedPlans(t *testing.T) {
	for name, cmd := range planFlagCommands() {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, advertisedFor(name), plansInPlanFlagHelp(t, cmd))
		})
	}
}

// TestPlanFlagHelpOmitsUnadvertisedPlans checks that no plan value the CLI does
// not advertise leaks into help text. Derived from the generated enums so a plan
// added to the schema is covered without being listed here.
func TestPlanFlagHelpOmitsUnadvertisedPlans(t *testing.T) {
	cmds := planFlagCommands()

	for name, cmd := range cmds {
		offered := map[string]bool{}
		for _, p := range plansInPlanFlagHelp(t, cmd) {
			offered[p] = true
		}

		schemaValues := func() []string {
			if strings.HasPrefix(name, "kv ") {
				out := make([]string, 0, len(client.KeyValuePlanValues()))
				for _, p := range client.KeyValuePlanValues() {
					out = append(out, string(p))
				}
				return out
			}
			out := make([]string, 0, len(pgclient.PostgresPlansValues()))
			for _, p := range pgclient.PostgresPlansValues() {
				out = append(out, string(p))
			}
			return out
		}()

		t.Run(name, func(t *testing.T) {
			for _, p := range schemaValues {
				assert.Equal(t, offered[p], slices.Contains(advertisedFor(name), p),
					"%q appears in %s --plan help but is not advertised (or vice versa)", p, name)
			}
		})
	}
}
