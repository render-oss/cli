package sandboxgroup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandboxgroup"
)

func newRepoWithServer(t *testing.T) (*sandboxgroup.Repo, *renderapi.Server) {
	t.Helper()
	server := renderapi.NewServer(t)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	return sandboxgroup.NewRepo(c), server
}

func TestRepoList_ReturnsGroups(t *testing.T) {
	repo, server := newRepoWithServer(t)
	ownerID := testids.WorkspaceID("active")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ownerID}))
	seeded := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		OwnerId: ownerID,
		Name:    "Default",
	}))

	got, err := repo.List(context.Background(), ownerID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, seeded.Id, got[0].Id)
	assert.Equal(t, "Default", got[0].Name)
}

func TestRepoList_ReturnsEmptySliceWhenNoGroups(t *testing.T) {
	repo, server := newRepoWithServer(t)
	ownerID := testids.WorkspaceID("active")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ownerID}))

	got, err := repo.List(context.Background(), ownerID)
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.NotNil(t, got, "callers rely on a non-nil empty slice")
}

func TestRepoList_PropagatesAPIError(t *testing.T) {
	repo, server := newRepoWithServer(t)
	_ = server // referenced for symmetry with the other tests
	ownerID := testids.WorkspaceID("unknown")
	// No owner seeded; fake responds 404.

	_, err := repo.List(context.Background(), ownerID)
	require.Error(t, err)
}

func TestRepoCreate_ReturnsCreatedGroup(t *testing.T) {
	repo, server := newRepoWithServer(t)
	ownerID := testids.WorkspaceID("active")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ownerID}))

	got, err := repo.Create(context.Background(), sandboxesclient.SandboxGroupPOST{
		OwnerId: ownerID,
		Name:    "agents",
	})
	require.NoError(t, err)
	assert.Equal(t, "agents", got.Name)
	assert.Equal(t, ownerID, got.OwnerId)
	assert.True(t, got.IsDefault, "first group in a workspace is the default")
	require.Len(t, server.SandboxGroups.Instances, 1)
	assert.Equal(t, server.SandboxGroups.Instances[0].Id, got.Id)
}

func TestRepoCreate_PropagatesAPIError(t *testing.T) {
	repo, server := newRepoWithServer(t)
	ownerID := testids.WorkspaceID("active")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ownerID}))
	server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		OwnerId: ownerID,
		Name:    "agents",
	}))

	_, err := repo.Create(context.Background(), sandboxesclient.SandboxGroupPOST{
		OwnerId: ownerID,
		Name:    "agents",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sandbox group name is taken")
}
