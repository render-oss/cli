package cmd

import (
	"bytes"
	"testing"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func executeWorkspacesCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_WORKSPACE", "")

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
	root.AddCommand(workspacesCmd)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

func TestWorkspaces_NonInteractive_ListsWorkspaces(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{Name: "acme-corp", Email: "admin@acme.com"}))
	server.Owners.Add(renderapi.NewOwner(client.Owner{Name: "side-project", Email: "me@example.com"}))

	result, err := executeWorkspacesCommand(t, server, "workspaces", "--output", "text")

	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "acme-corp")
	assert.Contains(t, result.Stdout, "side-project")
	assert.True(t, server.HasRequest("GET", "/owners"), "expected GET /owners to be called")
}

func TestWorkspaces_NonInteractive_EmptyList(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeWorkspacesCommand(t, server, "workspaces", "--output", "text")

	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "NAME", "expected header row to be present")
	assert.NotContains(t, result.Stdout, "@", "expected no workspace rows — all workspace emails contain @")
	assert.True(t, server.HasRequest("GET", "/owners"), "expected GET /owners to be called")
}
