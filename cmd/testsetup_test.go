package cmd

import (
	"bytes"
	"os"
	"sync"
	"testing"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/stretchr/testify/require"
)

// CommandResult holds the captured output from a CLI command execution.
type CommandResult struct {
	Stdout string
	Stderr string
}

var (
	cliSetupOnce sync.Once
	// rootCmdMu serializes access to the package-level rootCmd. Tests that call t.Parallel() will
	// not race, but they will queue at this lock — parallelism won't improve throughput until
	// rootCmd is refactored to be constructed per-invocation rather than shared globally.
	rootCmdMu sync.Mutex
)

// ensureCLISetup bootstraps the cobra command tree for testing.
// Safe to call multiple times — setup runs exactly once per test binary.
func ensureCLISetup() {
	cliSetupOnce.Do(func() {
		if err := os.Setenv("RENDER_API_KEY", "test-setup-key"); err != nil {
			panic("test setup: failed to set RENDER_API_KEY: " + err.Error())
		}
		// SetupCommands must see an API key so NewDefaultClient can build the
		// command dependencies without returning ErrLogin. Client construction
		// does not issue HTTP requests; commands that create clients during RunE
		// will use the RENDER_HOST set by executeCommand below.
		// TODO GROW-2433: Refactor tests to construct a fresh root command and
		// dependency graph per execution. Then setup can use each fake server
		// host instead of sharing a setup-time client across all tests.
		_ = SetupCommands()
		os.Unsetenv("RENDER_API_KEY") //nolint:errcheck
	})
}

// executeCommand runs an arbitrary CLI command against the fake server with no workspace configured.
func executeCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	ensureCLISetup()

	tmpCfg, err := os.CreateTemp("", "render-test-config-*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tmpCfg.Name()) })
	_ = tmpCfg.Close()

	t.Setenv("RENDER_CLI_CONFIG_PATH", tmpCfg.Name())
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_WORKSPACE", "")

	var stdout, stderr bytes.Buffer
	rootCmdMu.Lock()
	defer rootCmdMu.Unlock()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(args)

	execErr := rootCmd.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, execErr
}
