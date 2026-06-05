package keyvalue

import (
	"context"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/owner"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceResolveEnrichesRelatedResources(t *testing.T) {
	ctx := context.Background()

	t.Run("ID only lookup fetches environment and project", func(t *testing.T) {
		server := renderapi.NewServer(t)
		server.Owners.Add(renderapi.NewOwner(client.Owner{Id: "tea-test-workspace", Name: "Test Workspace"}))
		seeded := server.CreateProject(
			renderapi.ProjectAttrs{Name: "My Project", OwnerId: "tea-test-workspace"},
			renderapi.EnvAttrs{Name: "production"},
		)
		env := seeded.Env("production")
		kv := server.KV.Add(renderapi.NewKV(client.KeyValueDetail{
			Name:          "cache",
			EnvironmentId: &env.Id,
		}))

		resolved, err := newTestKeyValueService(t, server).Resolve(ctx, ResolveInput{IDOrName: kv.Id})

		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, kv.Id, resolved.KeyValue.Id)
		require.NotNil(t, resolved.Environment)
		assert.Equal(t, env.Id, resolved.Environment.Id)
		require.NotNil(t, resolved.Project)
		assert.Equal(t, seeded.Project.Id, resolved.Project.Id)
		assert.True(t, server.HasRequest("GET", "/key-value/"+kv.Id))
		assert.True(t, server.HasRequest("GET", "/environments/"+env.Id))
		assert.True(t, server.HasRequest("GET", "/projects/"+seeded.Project.Id))
	})

	t.Run("ungrouped KV short-circuits enrichment", func(t *testing.T) {
		server := renderapi.NewServer(t)
		kv := server.KV.Add(renderapi.NewKV(client.KeyValueDetail{Name: "cache"}))

		resolved, err := newTestKeyValueService(t, server).Resolve(ctx, ResolveInput{IDOrName: kv.Id})

		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.Equal(t, kv.Id, resolved.KeyValue.Id)
		assert.Nil(t, resolved.Environment)
		assert.Nil(t, resolved.Project)
		assert.True(t, server.HasRequest("GET", "/key-value/"+kv.Id))
		assert.False(t, server.HasRequest("GET", "/environments/"))
		assert.False(t, server.HasRequest("GET", "/projects/"))
	})
}

func newTestKeyValueService(t *testing.T, server *renderapi.Server) *Service {
	t.Helper()

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)

	ownerRepo := owner.NewRepo(c)
	projectRepo := project.NewRepo(c)
	environmentRepo := environment.NewRepo(c)
	return NewService(
		NewRepo(c),
		environmentRepo,
		projectRepo,
		resolve.New(ownerRepo, projectRepo, environmentRepo),
	)
}
