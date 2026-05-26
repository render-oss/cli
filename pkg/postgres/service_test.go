package postgres_test

import (
	"context"
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
		server:      server,
		service:     svc,
		workspaceID: workspaceID,
	}
}

func TestServiceCreate_UsesResolvedWorkspaceAndDefaults(t *testing.T) {
	harness := newHarness(t)

	created, err := harness.service.Create(context.Background(), pgtypes.CreatePostgresInput{})
	require.NoError(t, err)

	assert.NotEmpty(t, created.Id)
	assert.NotEmpty(t, created.Name)
	assert.Equal(t, harness.workspaceID, created.Owner.Id)
	assert.Equal(t, "free", string(created.Plan))
	assert.Equal(t, "18", string(created.Version))
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

	require.NotNil(t, created.EnvironmentId)
	assert.Equal(t, env.Id, *created.EnvironmentId)
}
