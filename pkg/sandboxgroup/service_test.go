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

func TestServiceList_ResolvesActiveWorkspace(t *testing.T) {
	server := renderapi.NewServer(t)
	activeWorkspace := testids.WorkspaceID("active")
	otherWorkspace := testids.WorkspaceID("other")
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: activeWorkspace}))
	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: otherWorkspace}))

	server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		OwnerId: activeWorkspace, Name: "Mine",
	}))
	server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		OwnerId: otherWorkspace, Name: "Not mine",
	}))

	t.Setenv("RENDER_WORKSPACE", activeWorkspace)

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	svc := sandboxgroup.NewService(sandboxgroup.NewRepo(c))

	got, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Mine", got[0].Name)
}

func TestServiceList_MissingWorkspaceReturnsError(t *testing.T) {
	server := renderapi.NewServer(t)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	svc := sandboxgroup.NewService(sandboxgroup.NewRepo(c))

	// No RENDER_WORKSPACE set and no config file, so config.WorkspaceID errors.
	t.Setenv("RENDER_WORKSPACE", "")
	t.Setenv("RENDER_CLI_CONFIG_PATH", t.TempDir()+"/nonexistent.yaml")

	_, err = svc.List(context.Background())
	require.Error(t, err)
}
