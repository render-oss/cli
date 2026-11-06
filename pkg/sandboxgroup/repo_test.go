package sandboxgroup_test

import (
	"context"
	"net/http"
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

func seedDefaultAndOtherGroup(server *renderapi.Server) (*sandboxesclient.SandboxGroup, *sandboxesclient.SandboxGroup) {
	ownerID := testids.WorkspaceID("active")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: ownerID}))
	defaultGroup := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		OwnerId: ownerID,
		Name:    "Default",
	}))
	otherGroup := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		OwnerId: ownerID,
		Name:    "agents",
	}))
	otherGroup.IsDefault = false
	return defaultGroup, otherGroup
}

func TestRepoGet_ReturnsGroup(t *testing.T) {
	repo, server := newRepoWithServer(t)
	_, otherGroup := seedDefaultAndOtherGroup(server)

	got, err := repo.Get(context.Background(), otherGroup.Id)
	require.NoError(t, err)

	assert.Equal(t, otherGroup.Id, got.Id)
	assert.Equal(t, "agents", got.Name)
	assert.False(t, got.IsDefault)
}

func TestRepoGet_UnknownGroupReturnsNotFound(t *testing.T) {
	repo, server := newRepoWithServer(t)
	seedDefaultAndOtherGroup(server)

	_, err := repo.Get(context.Background(), "sbg-doesnotexist000000")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "404")
}

func TestRepoDelete_RemovesGroup(t *testing.T) {
	repo, server := newRepoWithServer(t)
	defaultGroup, otherGroup := seedDefaultAndOtherGroup(server)

	err := repo.Delete(context.Background(), otherGroup.Id)
	require.NoError(t, err)

	require.Len(t, server.SandboxGroups.Instances, 1)
	assert.Equal(t, defaultGroup.Id, server.SandboxGroups.Instances[0].Id)
}

func TestRepoDelete_ConflictCodesMapToSentinelErrors(t *testing.T) {
	tests := []struct {
		name          string
		deleteDefault bool
		suspend       bool
		wantErr       error
	}{
		{name: "default group", deleteDefault: true, wantErr: sandboxgroup.ErrDefaultGroupNotDeletable},
		{name: "suspended group", suspend: true, wantErr: sandboxgroup.ErrSuspendedGroupNotDeletable},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, server := newRepoWithServer(t)
			defaultGroup, otherGroup := seedDefaultAndOtherGroup(server)
			target := otherGroup
			if tc.deleteDefault {
				target = defaultGroup
			}
			if tc.suspend {
				server.SandboxGroups.Suspend(target.Id)
			}

			err := repo.Delete(context.Background(), target.Id)

			require.ErrorIs(t, err, tc.wantErr)
			assert.Len(t, server.SandboxGroups.Instances, 2)
		})
	}
}

func TestRepoDelete_OtherConflictKeepsAPIError(t *testing.T) {
	repo, server := newRepoWithServer(t)
	_, otherGroup := seedDefaultAndOtherGroup(server)
	server.SandboxGroups.RespondWith(http.StatusConflict)

	err := repo.Delete(context.Background(), otherGroup.Id)

	require.Error(t, err)
	assert.NotErrorIs(t, err, sandboxgroup.ErrDefaultGroupNotDeletable)
	assert.NotErrorIs(t, err, sandboxgroup.ErrSuspendedGroupNotDeletable)
	assert.Contains(t, err.Error(), "409")
}

func TestRepoSetDefault_MovesDefaultWithinWorkspace(t *testing.T) {
	repo, server := newRepoWithServer(t)
	defaultGroup, otherGroup := seedDefaultAndOtherGroup(server)
	foreignGroup := server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		OwnerId: testids.WorkspaceID("other"),
	}))

	got, err := repo.SetDefault(context.Background(), otherGroup.Id)
	require.NoError(t, err)

	assert.Equal(t, otherGroup.Id, got.Id)
	assert.True(t, got.IsDefault)
	assert.True(t, otherGroup.IsDefault)
	assert.False(t, defaultGroup.IsDefault)
	assert.True(t, foreignGroup.IsDefault, "another workspace's default is untouched")
}

func TestRepoSetDefault_SuspendedGroupMapsToSentinelError(t *testing.T) {
	repo, server := newRepoWithServer(t)
	defaultGroup, otherGroup := seedDefaultAndOtherGroup(server)
	server.SandboxGroups.Suspend(otherGroup.Id)

	_, err := repo.SetDefault(context.Background(), otherGroup.Id)

	require.ErrorIs(t, err, sandboxgroup.ErrSuspendedGroupCannotBeDefault)
	assert.True(t, defaultGroup.IsDefault)
	assert.False(t, otherGroup.IsDefault)
}

func TestRepoSetDefault_UnknownGroupReturnsNotFound(t *testing.T) {
	repo, server := newRepoWithServer(t)
	defaultGroup, _ := seedDefaultAndOtherGroup(server)

	_, err := repo.SetDefault(context.Background(), "sbg-doesnotexist000000")
	require.Error(t, err)

	assert.Contains(t, err.Error(), "404")
	assert.True(t, defaultGroup.IsDefault)
}
