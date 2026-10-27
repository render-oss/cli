package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

// executeSandboxCommand runs the CLI with the sandbox tree registered at the
// root, mirroring executeSandboxGroupsCommand in sandboxgroups_test.go.
func executeSandboxCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
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
	setupSandboxCommands(root, deps)
	setupRootCmdPersistentRun(root, deps)

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)

	execErr := root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}

// The parent command is plural per the PRD CLI spec and STYLE.md's
// plural-resource rule.
func TestSandboxes_PluralCommandResolves(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeSandboxCommand(t, server, "sandboxes", "--help")
	require.NoError(t, err)
	assert.Contains(t, result.Stdout, "Manage sandboxes")
	assert.Contains(t, result.Stdout, "render sandboxes")
}

// The singular form is removed outright (no alias), per the #sandboxes-core
// decision that pre-alpha backwards compatibility is not needed.
func TestSandboxes_SingularFormRemoved(t *testing.T) {
	server := renderapi.NewServer(t)

	_, err := executeSandboxCommand(t, server, "sandbox")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestSandboxes_HelpListsSubcommands(t *testing.T) {
	server := renderapi.NewServer(t)

	result, err := executeSandboxCommand(t, server, "sandboxes", "--help")
	require.NoError(t, err)
	for _, sub := range []string{"create", "exec", "list", "snapshots", "stop"} {
		assert.Contains(t, result.Stdout, sub, "expected subcommand %q in help output", sub)
	}
}
