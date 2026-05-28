package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
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
	ea := newEarlyAccessCmd()
	root.AddCommand(ea)
	setupPGCommands(ea, deps)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}
