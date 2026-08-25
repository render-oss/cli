package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func TestAnalyticsCommandIsHiddenFromRootHelp(t *testing.T) {
	harness := newAnalyticsHarness(t, false)

	result := harness.execute("--help")

	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, harness.stdout.String(), "analytics")
}

func TestAnalyticsEligibilityIsInheritedByDescendants(t *testing.T) {
	root := &cobra.Command{Use: "render"}
	parent := &cobra.Command{Use: "parent"}
	child := &cobra.Command{Use: "child"}
	distractor := &cobra.Command{Use: "distractor"}
	root.AddCommand(parent, distractor)
	parent.AddCommand(child)

	require.True(t, commandIsAnalyticsEligible(root))
	require.True(t, commandIsAnalyticsEligible(child))
	require.True(t, commandIsAnalyticsEligible(distractor))

	markCommandAnalyticsIneligible(parent)

	require.False(t, commandIsAnalyticsEligible(parent))
	require.False(t, commandIsAnalyticsEligible(child))
	require.True(t, commandIsAnalyticsEligible(distractor))
}

func TestAnalyticsEligibilityIsFalseForUnresolvedCommand(t *testing.T) {
	require.False(t, commandIsAnalyticsEligible(nil))
}

type analyticsHarness struct {
	t         *testing.T
	server    *renderapi.Server
	configDir string
	root      *cobra.Command
	deps      *dependencies.Dependencies
	stdout    bytes.Buffer
	stderr    bytes.Buffer
}

// newAnalyticsHarness sets up a test harness to make writing tests for analytics commands easier
func newAnalyticsHarness(t *testing.T, shouldSend bool) *analyticsHarness {
	t.Helper()

	configDir := t.TempDir()
	t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
	t.Setenv("RENDER_CLI_CONFIG_PATH", "")
	t.Setenv("RENDER_API_KEY", "test-api-key")
	// The dev gate is held open and sending toggles through a user opt-out
	// instead, exercising the consent path the released CLI will use.
	t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", "1")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("RENDER_CLI_DISABLE_ANALYTICS", "")
	if !shouldSend {
		t.Setenv("DO_NOT_TRACK", "1")
	}

	server := renderapi.NewServer(t)
	c, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		return command.RuntimeSignals{}, nil
	}

	harness := &analyticsHarness{
		t:         t,
		server:    server,
		configDir: configDir,
		root:      newRootCmd(),
		deps:      deps,
	}
	harness.root.SetOut(&harness.stdout)
	harness.root.SetErr(&harness.stderr)
	setupAnalyticsCommands(harness.root, deps)
	setupRootCmdPersistentRun(harness.root, deps)
	return harness
}

// analyticsNoticeMarkerPath is where the harness's isolated config directory
// keeps the one-time notice marker.
func (h *analyticsHarness) analyticsNoticeMarkerPath() string {
	return filepath.Join(h.configDir, "state", "analytics-notice-shown")
}

// execute runs the provided args as a command
func (h *analyticsHarness) execute(args ...string) command.ExecutionResult {
	h.t.Helper()

	h.root.SetArgs(args)
	result := runExecution(h.root, time.Now())
	onExecutionComplete(result, h.deps, h.root)
	return result
}

func writeAnalyticsEventFile(t *testing.T, configDir string, contents []byte) string {
	t.Helper()

	path := analyticsEventPath(configDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, contents, 0o600))
	return path
}

// analyticsEventPath creates a new unique path for an event json file
func analyticsEventPath(configDir string) string {
	return filepath.Join(configDir, "state", "analytics", "events", "event-"+uuid.NewString()+".json")
}
