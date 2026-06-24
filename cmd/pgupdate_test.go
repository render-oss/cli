package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testassert"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/internal/testrequire"
	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/pointers"
)

type pgUpdateHarness struct {
	t      *testing.T
	server *renderapi.Server
}

// newPGUpdateHarness sets up a server fake and seeds it with an (active) workspace.
func newPGUpdateHarness(t *testing.T) pgUpdateHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: pgActiveWorkspaceID, Name: "Test Workspace"}))
	t.Setenv("RENDER_WORKSPACE", pgActiveWorkspaceID)

	return pgUpdateHarness{t: t, server: server}
}

// execute invokes the `render ea pg update` command, passing through all extraArgs.
func (h pgUpdateHarness) execute(extraArgs ...string) (CommandResult, error) {
	h.t.Helper()
	return executePGCommand(h.t, h.server, append([]string{"ea", "pg", "update"}, extraArgs...)...)
}

func TestPGUpdate_ByID_RendersDiff(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := seedPG(harness.server, "my-db")

	result, err := harness.execute(pg.Id, "--name", "renamed-db", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Updated Postgres database")
	testassert.ContainsInOrder(t, result.Stdout, "Changes:", "Name:", "my-db → renamed-db")
	assert.Equal(t, "renamed-db", pg.Name, "stored database should be renamed in place")
}

func TestPGUpdate_ByName(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := seedPG(harness.server, "by-name-db")
	// pg is a live pointer to the server's stored state, so it starts on the
	// seed default and reflects the update in place once the command runs.
	require.Equal(t, pgclient.Free, pg.Plan, "precondition: seeded database starts on the free plan")

	result, err := harness.execute("by-name-db", "--plan", "pro_4gb", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "Updated Postgres database")
	assert.Equal(t, pgclient.PostgresPlans("pro_4gb"), pg.Plan)
}

func TestPGUpdate_PartialUpdate_LeavesOtherFieldsUntouched(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := harness.server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:       "app-db",
		Owner:      client.Owner{Id: pgActiveWorkspaceID},
		Plan:       pgclient.PostgresPlans("pro_4gb"),
		DiskSizeGB: pointers.From(20),
	}))

	result, err := harness.execute(pg.Id, "--name", "app-db-renamed", "--output", "text")
	require.NoError(t, err)

	// Only the name changed; plan and disk are untouched server-side.
	assert.Equal(t, "app-db-renamed", pg.Name)
	assert.Equal(t, pgclient.PostgresPlans("pro_4gb"), pg.Plan)
	require.NotNil(t, pg.DiskSizeGB)
	assert.Equal(t, 20, *pg.DiskSizeGB)

	// The Changes section reflects only the field that changed (the full-state
	// block below it still lists every field, so scope the check to the diff).
	changes, _, _ := strings.Cut(result.Stdout, "Full details:")
	assert.Contains(t, changes, "Name:")
	assert.NotContains(t, changes, "Plan:")
	assert.NotContains(t, changes, "Disk size:")
}

func TestPGUpdate_NoEffectiveChange_SaysNoChanges(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := seedPG(harness.server, "same-name")

	// Renaming to the current name is a valid request but changes nothing.
	result, err := harness.execute(pg.Id, "--name", "same-name", "--output", "text")
	require.NoError(t, err)

	assert.Contains(t, result.Stdout, "No changes applied to Postgres database")
}

func TestPGUpdate_IPAllowListReplace(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := harness.server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:  "ip-db",
		Owner: client.Owner{Id: pgActiveWorkspaceID},
		IpAllowList: []client.CidrBlockAndDescription{
			{CidrBlock: "192.0.2.0/24", Description: "old"},
		},
	}))

	_, err := harness.execute(pg.Id,
		"--ip-allow-list", "cidr=10.0.0.0/8,description=internal",
		"--output", "text")
	require.NoError(t, err)

	require.Len(t, pg.IpAllowList, 1)
	assert.Equal(t, "10.0.0.0/8", pg.IpAllowList[0].CidrBlock)
	assert.Equal(t, "internal", pg.IpAllowList[0].Description)
}

func TestPGUpdate_ClearIPAllowList(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := harness.server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:  "ip-db",
		Owner: client.Owner{Id: pgActiveWorkspaceID},
		IpAllowList: []client.CidrBlockAndDescription{
			{CidrBlock: "10.0.0.0/8", Description: "internal"},
		},
	}))

	_, err := harness.execute(pg.Id, "--clear-ip-allow-list", "--output", "text")
	require.NoError(t, err)

	assert.Empty(t, pg.IpAllowList, "clear should remove all allow-list entries")
}

func TestPGUpdate_EnvironmentDisambiguation(t *testing.T) {
	harness := newPGUpdateHarness(t)
	project := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
		renderapi.EnvAttrs{Name: "staging"},
	)
	prodPG := seedPGInEnv(harness.server, "not-very-unique", project.Env("production").Id)
	stagingPG := seedPGInEnv(harness.server, "not-very-unique", project.Env("staging").Id)

	_, err := harness.execute("not-very-unique", "--environment", "production", "--name", "so-special", "--output", "text")
	require.NoError(t, err)

	assert.Equal(t, "so-special", prodPG.Name, "the production database should be the one updated")
	assert.Equal(t, "not-very-unique", stagingPG.Name, "the staging database must be untouched")
}

func TestPGUpdate_NameCollision_Errors(t *testing.T) {
	harness := newPGUpdateHarness(t)
	seedPG(harness.server, "not-unique")
	seedPG(harness.server, "not-unique")

	_, err := harness.execute("not-unique", "--name", "renamed", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Multiple Postgres databases")
}

func TestPGUpdate_JSONOutput_ReturnsDataAndDiff(t *testing.T) {
	harness := newPGUpdateHarness(t)
	project := harness.server.CreateProject(
		renderapi.ProjectAttrs{Name: "My Project", OwnerId: pgActiveWorkspaceID},
		renderapi.EnvAttrs{Name: "production"},
	)
	pg := seedPGInEnv(harness.server, "json-db", project.Env("production").Id)

	result, err := harness.execute(pg.Id, "--name", "json-renamed", "--output", "json")
	require.NoError(t, err)

	body := unmarshalPGJSONOutput(t, result.Stdout)
	data := testrequire.SubMap(t, body, "data")
	diff := testrequire.SubMap(t, body, "diff")
	nameDiff := testrequire.SubMap(t, diff, "name")
	assert.Equal(t, pg.Id, data["id"])
	assert.Equal(t, "json-renamed", data["name"])
	assert.Equal(t, project.Project.Id, data["projectId"])
	assert.Equal(t, project.Env("production").Id, data["environmentId"])
	assert.Equal(t, "json-db", nameDiff["before"])
	assert.Equal(t, "json-renamed", nameDiff["after"])
}

func TestPGUpdate_UnknownID_Errors(t *testing.T) {
	harness := newPGUpdateHarness(t)
	missing := testids.PostgresID("missing")

	_, err := harness.execute(missing, "--name", "whatever", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), missing)
}

func TestPGUpdate_NoMutationFields_Errors(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := seedPG(harness.server, "my-db")

	_, err := harness.execute(pg.Id, "--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one field")
}

func TestPGUpdate_ParameterOverrideFlagIsUnknown(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := seedPG(harness.server, "my-db")

	result, err := harness.execute(pg.Id, "--parameter-override", "max_connections=111", "--output", "text")
	require.Error(t, err)
	assert.Contains(t, result.Stderr, "unknown flag: --parameter-override")
}

func TestPGUpdate_BothIPAllowListAndClearFlags_Errors(t *testing.T) {
	harness := newPGUpdateHarness(t)
	pg := seedPG(harness.server, "my-db")

	_, err := harness.execute(pg.Id,
		"--ip-allow-list", "cidr=10.0.0.0/8,description=internal",
		"--clear-ip-allow-list",
		"--output", "text")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}
