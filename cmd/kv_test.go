package cmd

import (
	"bytes"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/stretchr/testify/require"
)

// kvTestWorkspaceID is the workspace ID that kv command tests select as
// active before exercising a subcommand. Used by kv create / delete / update
// tests via executeKV* helpers.
var kvTestWorkspaceID = testids.WorkspaceID("active")

const kvTestWorkspaceName = "Test Workspace"

// seedKV adds a KV instance owned by the active workspace, with a random
// valid ID and the given name. Returns the seeded detail for assertions.
func seedKV(server *renderapi.Server, name string) *client.KeyValueDetail {
	kv := renderapi.NewKV(client.KeyValueDetail{
		Name:  name,
		Owner: client.Owner{Id: kvTestWorkspaceID},
	})
	server.KV.Instances = append(server.KV.Instances, kv)
	return kv
}

// seedKVInEnv adds a KV instance scoped to a specific environment.
func seedKVInEnv(server *renderapi.Server, name, envID string) *client.KeyValueDetail {
	kv := renderapi.NewKV(client.KeyValueDetail{
		Name:          name,
		Owner:         client.Owner{Id: kvTestWorkspaceID},
		EnvironmentId: &envID,
	})
	server.KV.Instances = append(server.KV.Instances, kv)
	return kv
}

// executeKVCommand runs a `kv` command with the default active workspace
// seeded in the test config. Use this for ordinary KV command tests.
func executeKVCommand(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	return executeKVCommandWithConfig(t, server, &config.Config{
		Workspace:     kvTestWorkspaceID,
		WorkspaceName: kvTestWorkspaceName,
	}, extraArgs...)
}

// executeKVCommandWithoutActiveWorkspace runs a `kv` command without
// seeding the active workspace config. Prefer executeKVCommand unless the test
// specifically covers missing-workspace behavior or workspace derivation.
func executeKVCommandWithoutActiveWorkspace(t *testing.T, server *renderapi.Server, extraArgs ...string) (CommandResult, error) {
	t.Helper()
	return executeKVCommandWithConfig(t, server, &config.Config{}, extraArgs...)
}

// executeKVCommandWithConfig implements the workspace setup variants.
// Tests should call executeKVCommand or executeKVCommandWithoutActiveWorkspace
// instead of calling this helper directly.
func executeKVCommandWithConfig(t *testing.T, server *renderapi.Server, cfg *config.Config, extraArgs ...string) (CommandResult, error) {
	t.Helper()

	server.Owners.Add(renderapi.NewOwner(client.Owner{Id: kvTestWorkspaceID, Name: kvTestWorkspaceName}))
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_WORKSPACE", "")
	if cfg != nil {
		require.NoError(t, cfg.Persist())
	}

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{
			StdinTTY:  false,
			StdoutTTY: false,
			StderrTTY: false,
		}, nil
	}

	root := newRootCmd()
	setupKVCommands(root, deps)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"kv"}, extraArgs...))

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}
