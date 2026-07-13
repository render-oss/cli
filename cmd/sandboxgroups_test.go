package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

var sandboxGroupsActiveWorkspaceID = testids.WorkspaceID("active")

func executeSandboxGroupsCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")

	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{StdinTTY: false, StdoutTTY: false, StderrTTY: false}, nil
	}

	root := newRootCmd()
	ea := newEarlyAccessCmd()
	root.AddCommand(ea)
	setupSandboxGroupsCommands(ea, deps)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

func seedSandboxGroup(server *renderapi.Server, name string) *sandboxesclient.SandboxGroup {
	return seedSandboxGroupForOwner(server, name, sandboxGroupsActiveWorkspaceID)
}

func seedSandboxGroupForOwner(server *renderapi.Server, name, ownerID string) *sandboxesclient.SandboxGroup {
	return server.SandboxGroups.Add(renderapi.NewSandboxGroup(sandboxesclient.SandboxGroup{
		Name:    name,
		OwnerId: ownerID,
	}))
}

// unmarshalSandboxGroupsJSONOutput decodes the list command's JSON output. The
// command passes a bare []*SandboxGroup to command.NonInteractive (mirroring
// sandboxlist.go), so the output is a top-level JSON array, not a {"data": ...}
// envelope.
func unmarshalSandboxGroupsJSONOutput(t *testing.T, stdout string) []any {
	t.Helper()
	var data []any
	require.NoError(t, json.Unmarshal([]byte(stdout), &data), "expected a JSON array, got: %s", stdout)
	return data
}
