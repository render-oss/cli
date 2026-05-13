package keyvalue

import (
	"context"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateKeyValue_DoesNotOverrideOwnerIDWithActiveWorkspace(t *testing.T) {
	t.Setenv("RENDER_WORKSPACE", "tea-active-workspace")

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{
		Id:   "tea-target-workspace",
		Name: "Target Workspace",
	}))

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	repo := NewRepo(c)

	kv, err := repo.CreateKeyValue(context.Background(), client.CreateKeyValueJSONRequestBody{
		Name:    "my-kv",
		OwnerId: "tea-target-workspace",
		Plan:    client.KeyValuePlanFree,
	})

	require.NoError(t, err)
	assert.Equal(t, "tea-target-workspace", kv.Owner.Id)
}
