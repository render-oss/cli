package postgres_test

import (
	"context"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/postgres"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resolve"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
)

type testHarness struct {
	t           testing.TB
	server      *renderapi.Server
	service     *postgres.Service
	workspaceID string
}

func newHarness(t *testing.T) testHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	workspaceID := testids.WorkspaceID("active")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: workspaceID, Name: "Active Workspace"}))
	t.Setenv("RENDER_WORKSPACE", workspaceID)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)

	ownerRepo := owner.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	projectRepo := project.NewRepo(c)
	svc := postgres.NewService(
		postgres.NewRepo(c),
		environmentRepo,
		projectRepo,
		resolve.New(ownerRepo, projectRepo, environmentRepo),
	)

	return testHarness{
		t:           t,
		server:      server,
		service:     svc,
		workspaceID: workspaceID,
	}
}

// addPostgres creates a Postgres that is owned by the seeded Active Workspace directly (no environment)
func (h testHarness) addPostgres(name string) *client.PostgresDetail {
	return h.server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:  name,
		Owner: client.Owner{Id: h.workspaceID},
	}))
}

// addPostgresInEnvironment creates a Postgres that is owned by the provided environment
// Use only if the provided environment lives in the Active Workspace
// Otherwise, use addPostgresInWorkspaceEnvironment
func (h testHarness) addPostgresInEnvironment(name string, environmentID string) *client.PostgresDetail {
	return h.addPostgresInWorkspaceEnvironment(name, h.workspaceID, environmentID)
}

func (h testHarness) addPostgresInWorkspaceEnvironment(name string, workspaceID string, environmentID string) *client.PostgresDetail {
	h.requireOwner(workspaceID)
	env := h.requireEnvironment(environmentID)
	project := h.requireProject(env.ProjectId)
	require.Equal(h.t, workspaceID, project.Owner.Id, "test setup: environment %q belongs to project %q in workspace %q, not workspace %q", environmentID, project.Id, project.Owner.Id, workspaceID)

	return h.server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:          name,
		Owner:         client.Owner{Id: workspaceID},
		EnvironmentId: pointers.From(environmentID),
	}))
}

func (h testHarness) addProjectAndEnvironment(workspaceID string, projectName string, environmentName string) client.Environment {
	h.requireOwner(workspaceID)

	project := renderapi.NewProject(renderapi.ProjectAttrs{
		Name:    projectName,
		OwnerId: workspaceID,
	})
	env := renderapi.NewEnvironment(client.Environment{
		Name:      environmentName,
		ProjectId: project.Id,
	})
	project.EnvironmentIds = []string{env.Id}
	h.server.Projects.Add(project)
	h.server.Environments.Add(env)
	return *env
}

func (h testHarness) requireOwner(workspaceID string) {
	h.t.Helper()

	idx := slices.IndexFunc(h.server.Owners.Instances, func(owner *client.Owner) bool {
		return owner.Id == workspaceID
	})
	require.NotEqual(h.t, -1, idx, "test setup: owner %q must be registered before use", workspaceID)
}

func (h testHarness) requireEnvironment(environmentID string) client.Environment {
	h.t.Helper()

	idx := slices.IndexFunc(h.server.Environments.Instances, func(env *client.Environment) bool {
		return env.Id == environmentID
	})
	require.NotEqual(h.t, -1, idx, "test setup: environment %q must be registered before use", environmentID)
	return *h.server.Environments.Instances[idx]
}

func (h testHarness) requireProject(projectID string) client.Project {
	h.t.Helper()

	idx := slices.IndexFunc(h.server.Projects.Instances, func(project *client.Project) bool {
		return project.Id == projectID
	})
	require.NotEqual(h.t, -1, idx, "test setup: project %q must be registered before use", projectID)
	return *h.server.Projects.Instances[idx]
}

func TestServiceCreate_UsesResolvedWorkspaceAndDefaults(t *testing.T) {
	harness := newHarness(t)

	created, err := harness.service.Create(context.Background(), pgtypes.CreatePostgresInput{})
	require.NoError(t, err)

	assert.NotEmpty(t, created.Postgres.Id)
	assert.NotEmpty(t, created.Postgres.Name)
	assert.Equal(t, harness.workspaceID, created.Postgres.Owner.Id)
	assert.Equal(t, "free", string(created.Postgres.Plan))
	assert.Equal(t, "18", string(created.Postgres.Version))
	assert.Len(t, harness.server.Postgres.Instances, 1)
}

// When --project is given without --environment, Service.Create auto-selects
// the project's sole environment. This pins down a subtle behavior of the
// resolver call chain (ResolveScope does not auto-select; ResolveEnvironmentID does).
func TestServiceCreate_AutoSelectsSingleEnvironmentWhenOnlyProjectGiven(t *testing.T) {
	harness := newHarness(t)

	proj := renderapi.NewProject(renderapi.ProjectAttrs{
		Name:    "my-project",
		OwnerId: harness.workspaceID,
	})
	harness.server.Projects.Add(proj)

	env := renderapi.NewEnvironment(client.Environment{
		Name:      "prod",
		ProjectId: proj.Id,
	})
	harness.server.Environments.Add(env)

	created, err := harness.service.Create(context.Background(), pgtypes.CreatePostgresInput{
		ProjectIDOrName: pointers.From(proj.Id),
	})
	require.NoError(t, err)

	require.NotNil(t, created.Postgres.EnvironmentId)
	assert.Equal(t, env.Id, *created.Postgres.EnvironmentId)
	require.NotNil(t, created.Environment)
	assert.Equal(t, env.Id, created.Environment.Id)
	require.NotNil(t, created.Project)
	assert.Equal(t, proj.Id, created.Project.Id)
}

func TestServiceDelete_DeletesByID(t *testing.T) {
	harness := newHarness(t)
	pg := harness.addPostgres("my-db")

	err := harness.service.Delete(context.Background(), pg.Id)
	require.NoError(t, err)

	assert.Empty(t, harness.server.Postgres.Instances)
}

func TestServiceList_FilterByProject(t *testing.T) {
	harness := newHarness(t)

	projAProdEnv := harness.addProjectAndEnvironment(harness.workspaceID, "Project A", "production")
	projBProdEnv := harness.addProjectAndEnvironment(harness.workspaceID, "Project B", "production")
	projAPostgres := harness.addPostgresInEnvironment("project-a-db", projAProdEnv.Id)
	projBPostgres := harness.addPostgresInEnvironment("project-b-db", projBProdEnv.Id)

	result, err := harness.service.List(context.Background(), pgtypes.ListPostgresInput{
		ProjectIDOrName: pointers.From("Project A"),
	})
	require.NoError(t, err)

	require.Len(t, result, 1)
	assert.Equal(t, projAPostgres.Id, result[0].ID())
	assert.NotEqual(t, projBPostgres.Id, result[0].ID())
}

func TestServiceList_ProjectWithNoEnvironmentsReturnsEmptyList(t *testing.T) {
	harness := newHarness(t)
	project := renderapi.NewProject(renderapi.ProjectAttrs{
		Name:    "Empty Project",
		OwnerId: harness.workspaceID,
	})
	harness.server.Projects.Add(project)

	result, err := harness.service.List(context.Background(), pgtypes.ListPostgresInput{
		ProjectIDOrName: pointers.From("Empty Project"),
	})
	require.NoError(t, err)

	assert.Empty(t, result)
	assert.NotNil(t, result)
	assert.False(t, harness.server.HasRequest("GET", "/postgres"))
}

func TestServiceList_EnvironmentLookupStaysInActiveWorkspace(t *testing.T) {
	harness := newHarness(t)
	otherWorkspaceID := testids.WorkspaceID("other")
	harness.server.Owners.Add(renderapi.NewOwner(client.Owner{Id: otherWorkspaceID, Name: "Other Workspace"}))

	activeProduction := harness.addProjectAndEnvironment(harness.workspaceID, "Active Project", "production")
	otherProduction := harness.addProjectAndEnvironment(otherWorkspaceID, "Other Project", "production")
	activeWorkspacePG := harness.addPostgresInEnvironment("active-db", activeProduction.Id)
	otherWorkspacePG := harness.addPostgresInWorkspaceEnvironment("other-db", otherWorkspaceID, otherProduction.Id)

	result, err := harness.service.List(context.Background(), pgtypes.ListPostgresInput{
		EnvironmentIDOrName: pointers.From("production"),
	})
	require.NoError(t, err)

	require.Len(t, result, 1)
	assert.NotEqual(t, otherWorkspacePG.Id, result[0].ID())
	assert.Equal(t, activeWorkspacePG.Id, result[0].ID())
}

// Given 2 environments in different workspaces each named "production" each with a database named "my-pg",
// Ensure that we resolve the Postgres instance relative to the active workspace
func TestServiceResolve_EnvironmentLookupStaysInActiveWorkspace(t *testing.T) {
	harness := newHarness(t)
	otherWorkspaceID := testids.WorkspaceID("other")
	harness.server.Owners.Add(renderapi.NewOwner(client.Owner{Id: otherWorkspaceID, Name: "Other Workspace"}))

	activeProduction := harness.addProjectAndEnvironment(harness.workspaceID, "Active Project", "production")
	otherProduction := harness.addProjectAndEnvironment(otherWorkspaceID, "Other Project", "production")
	activeWorkspacePG := harness.addPostgresInEnvironment("my-db", activeProduction.Id)
	otherWorkspacePG := harness.addPostgresInWorkspaceEnvironment("my-db", otherWorkspaceID, otherProduction.Id)

	result, err := harness.service.Resolve(context.Background(), postgres.ResolveInput{
		IDOrName:            "my-db",
		EnvironmentIDOrName: pointers.From("production"),
	})
	require.NoError(t, err)

	require.NotNil(t, result.Postgres)
	assert.NotEqual(t, otherWorkspacePG.Id, result.Postgres.Id)
	assert.Equal(t, activeWorkspacePG.Id, result.Postgres.Id)
}

func TestServiceResolve_EnrichesProjectAndEnvironmentForIDLookup(t *testing.T) {
	harness := newHarness(t)
	env := harness.addProjectAndEnvironment(harness.workspaceID, "Active Project", "production")
	pg := harness.addPostgresInEnvironment("my-db", env.Id)
	project := harness.requireProject(env.ProjectId)

	result, err := harness.service.Resolve(context.Background(), postgres.ResolveInput{
		IDOrName: pg.Id,
	})
	require.NoError(t, err)

	require.NotNil(t, result.Postgres)
	assert.Equal(t, pg.Id, result.Postgres.Id)
	require.NotNil(t, result.Environment)
	assert.Equal(t, env.Id, result.Environment.Id)
	require.NotNil(t, result.Project)
	assert.Equal(t, project.Id, result.Project.Id)
}

func TestServiceResolve_PreservesResolvedProjectAndEnvironmentForScopedNameLookup(t *testing.T) {
	harness := newHarness(t)
	env := harness.addProjectAndEnvironment(harness.workspaceID, "Active Project", "production")
	pg := harness.addPostgresInEnvironment("my-db", env.Id)
	project := harness.requireProject(env.ProjectId)

	result, err := harness.service.Resolve(context.Background(), postgres.ResolveInput{
		IDOrName:            "my-db",
		ProjectIDOrName:     pointers.From(project.Name),
		EnvironmentIDOrName: pointers.From(env.Name),
	})
	require.NoError(t, err)

	require.NotNil(t, result.Postgres)
	assert.Equal(t, pg.Id, result.Postgres.Id)
	require.NotNil(t, result.Environment)
	assert.Equal(t, env.Id, result.Environment.Id)
	require.NotNil(t, result.Project)
	assert.Equal(t, project.Id, result.Project.Id)
}

func TestServiceResolve_IDLookup_NonNotFoundError_Surfaces(t *testing.T) {
	harness := newHarness(t)
	pg := harness.addPostgres("my-db")
	harness.server.Postgres.RespondWith(http.StatusInternalServerError)

	_, err := harness.service.Resolve(context.Background(), postgres.ResolveInput{
		IDOrName: pg.Id,
	})
	require.Error(t, err)

	assert.NotContains(t, err.Error(), "No Postgres database named")
	assert.Len(t, harness.server.Postgres.Instances, 1)
	assert.False(t, harness.server.HasRequest("DELETE", "/postgres/"))
}

func TestService_SuspendThenResume(t *testing.T) {
	harness := newHarness(t)
	pg := harness.addPostgres("my-db")
	require.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status)

	err := harness.service.SuspendPostgres(context.Background(), pg.Id)
	require.NoError(t, err)
	assert.Equal(t, client.DatabaseStatusSuspended, harness.server.Postgres.Instances[0].Status)
	assert.True(t, harness.server.HasRequest("POST", "/postgres/"+pg.Id+"/suspend"))

	err = harness.service.ResumePostgres(context.Background(), pg.Id)
	require.NoError(t, err)
	assert.Equal(t, client.DatabaseStatusAvailable, harness.server.Postgres.Instances[0].Status)
	assert.True(t, harness.server.HasRequest("POST", "/postgres/"+pg.Id+"/resume"))
}
