package cmd

import (
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/stretchr/testify/require"
)

// ACTIVE_WORKSPACE_ID is the workspace ID that kv command tests select as
// active before exercising a subcommand. Used by kv create / delete / update
// tests via executeKV* helpers.
var ACTIVE_WORKSPACE_ID = testids.WorkspaceID("active")

// seedKV adds a KV instance owned by the active workspace, with a random
// valid ID and the given name. Returns the seeded detail for assertions.
func seedKV(server *renderapi.Server, name string) *client.KeyValueDetail {
	kv := renderapi.NewKV(&client.KeyValueDetail{
		Name:  name,
		Owner: client.Owner{Id: ACTIVE_WORKSPACE_ID},
	})
	server.KV.Instances = append(server.KV.Instances, kv)
	return kv
}

// seedKVInEnv adds a KV instance scoped to a specific environment.
func seedKVInEnv(server *renderapi.Server, name, envID string) *client.KeyValueDetail {
	kv := renderapi.NewKV(&client.KeyValueDetail{
		Name:          name,
		Owner:         client.Owner{Id: ACTIVE_WORKSPACE_ID},
		EnvironmentId: &envID,
	})
	server.KV.Instances = append(server.KV.Instances, kv)
	return kv
}

func TestKeyValueAliasResolvesToKVCommand(t *testing.T) {
	short, _, err := rootCmd.Find([]string{"ea", "kv"})
	require.NoError(t, err)
	require.Same(t, kvCmd, short)

	alias, _, err := rootCmd.Find([]string{"ea", "keyvalue"})
	require.NoError(t, err)
	require.Same(t, kvCmd, alias)
}
