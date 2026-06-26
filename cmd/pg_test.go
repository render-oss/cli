package cmd

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/render-oss/cli/pkg/pointers"
)

var pgActiveWorkspaceID = testids.WorkspaceID("active")

func executePGCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-api-key")

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
	setupPGCommands(root, deps)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

// seedPG adds a new Postgres database to the seeded active workspace.
func seedPG(server *renderapi.Server, name string) *client.PostgresDetail {
	return server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:  name,
		Owner: client.Owner{Id: pgActiveWorkspaceID},
	}))
}

// seedPGInEnv adds a new Postgres database to the specified environment inside the active workspace.
func seedPGInEnv(server *renderapi.Server, name string, envID string) *client.PostgresDetail {
	return server.Postgres.Add(renderapi.NewPostgres(client.PostgresDetail{
		Name:          name,
		Owner:         client.Owner{Id: pgActiveWorkspaceID},
		EnvironmentId: pointers.From(envID),
	}))
}

// unmarshalPGJSONOutput decodes command stdout for tests that assert Postgres JSON output.
func unmarshalPGJSONOutput(t *testing.T, stdout string) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &body), "expected valid JSON, got: %s", stdout)
	return body
}
