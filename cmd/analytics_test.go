package cmd

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	renderapi "github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/analytics"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
)

func TestAnalyticsCommandIsHiddenFromRootHelp(t *testing.T) {
	harness := newAnalyticsHarness(t, analyticsHarnessInitialState{})

	result := harness.execute("--help")

	require.Equal(t, 0, result.ExitCode)
	require.NotContains(t, result.Stdout, "analytics")
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

func TestAnalyticsNoticeEligibilityIsInheritedByDescendants(t *testing.T) {
	root := &cobra.Command{Use: "render"}
	parent := &cobra.Command{Use: "parent"}
	child := &cobra.Command{Use: "child"}
	distractor := &cobra.Command{Use: "distractor"}
	root.AddCommand(parent, distractor)
	parent.AddCommand(child)

	require.True(t, commandIsAnalyticsNoticeEligible(root))
	require.True(t, commandIsAnalyticsNoticeEligible(child))
	require.True(t, commandIsAnalyticsNoticeEligible(distractor))

	markCommandAnalyticsNoticeIneligible(parent)

	require.False(t, commandIsAnalyticsNoticeEligible(parent))
	require.False(t, commandIsAnalyticsNoticeEligible(child))
	require.True(t, commandIsAnalyticsNoticeEligible(distractor))
}

func TestAnalyticsEligibilityIsFalseForUnresolvedCommand(t *testing.T) {
	require.False(t, commandIsAnalyticsEligible(nil))
}

func TestAnalyticsNoticeEligibilityIsFalseForUnresolvedCommand(t *testing.T) {
	require.False(t, commandIsAnalyticsNoticeEligible(nil))
}

// analyticsHarnessInitialState declares the conditions an [analyticsHarness]
// establishes before its first CLI invocation. Tests using it cannot call
// t.Parallel because [testing.T.Setenv] rejects parallel tests and tests with
// parallel ancestors.
type analyticsHarnessInitialState struct {
	// devGateOpen sets RENDER_TEST_ENABLE_ANALYTICS=1 when true, opening the
	// internal gate required for analytics sends.
	devGateOpen bool

	// userOptedOut controls whether analytics consent is denied by the user.
	userOptedOut bool

	// loggingEnabled controls whether analytics diagnostics are written to stderr.
	loggingEnabled bool

	// noticeMarkerPresent controls whether the disclosure marker exists.
	noticeMarkerPresent bool

	// stderrTTY makes the harness's [command.DetectRuntimeSignals] stub report
	// stderr as a TTY.
	stderrTTY bool

	// ci sets CI=1 and makes the runtime detection stub report CI.
	ci bool

	// runtimeSignalErr is returned by the runtime detection stub.
	runtimeSignalErr error

	// allowSubprocess controls whether "render analytics send" subprocesses may run.
	allowSubprocess bool
}

// analyticsHarness lets tests invoke the CLI with isolated analytics state. Its
// fake API and filesystem state persist across invocations. Each invocation gets
// fresh dependencies and a fresh Cobra app, so command-tree state does not carry
// over.
type analyticsHarness struct {
	t                               *testing.T
	server                          *renderapi.Server
	configDir                       string
	runtimeSignals                  command.RuntimeSignals
	runtimeSignalErr                error
	runtimeSignalDetectionCallCount int
}

// analyticsExecution contains everything one harness invocation produced.
type analyticsExecution struct {
	command.ExecutionResult
	Stdout string
	Stderr string
}

// analyticsInvocation holds the dependencies, Cobra app, and output buffers for
// one CLI invocation. It is an internal harness detail; tests should not
// construct it directly.
type analyticsInvocation struct {
	root   *cobra.Command
	deps   *dependencies.Dependencies
	stdout bytes.Buffer
	stderr bytes.Buffer
}

var analyticsWorkspaceID = testids.WorkspaceID("analytics test workspace")

// newAnalyticsHarness creates a harness for testing the CLI's analytics
// behavior. It seeds an active workspace in the fake API. Each invocation
// registers the real analytics, Postgres, login, and logout commands plus the
// fixture commands used by the notice tests.
func newAnalyticsHarness(t *testing.T, initialState analyticsHarnessInitialState) *analyticsHarness {
	t.Helper()

	server := renderapi.NewServer(t)
	server.Owners.Add(renderapi.NewOwner(client.Owner{
		Id:   analyticsWorkspaceID,
		Name: "Analytics Workspace",
	}))
	harness := &analyticsHarness{
		t:                t,
		server:           server,
		configDir:        t.TempDir(),
		runtimeSignals:   command.RuntimeSignals{StderrTTY: initialState.stderrTTY},
		runtimeSignalErr: initialState.runtimeSignalErr,
	}

	analytics.ClearSignalEnvVars(t)
	t.Setenv("RENDER_CLI_CONFIG_DIR", harness.configDir)
	t.Setenv("RENDER_CLI_CONFIG_PATH", "")
	t.Setenv("RENDER_API_KEY", "test-api-key")
	t.Setenv("RENDER_HOST", server.URL()+"/")
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "sync")
	t.Setenv("RENDER_OUTPUT", "")
	t.Setenv("RENDER_WORKSPACE", analyticsWorkspaceID)
	t.Setenv("RENDER_CLI_DISABLE_ANALYTICS", "")
	harness.setDevelopmentGate(initialState.devGateOpen)
	harness.setUserOptOut(initialState.userOptedOut)
	harness.setAnalyticsLogging(initialState.loggingEnabled)
	harness.setAnalyticsSubprocessPermission(initialState.allowSubprocess)
	harness.setCI(initialState.ci)
	harness.setNoticeMarker(initialState.noticeMarkerPresent)

	return harness
}

// boolToEnvValue returns "1" for true and an empty string for false.
func boolToEnvValue(enabled bool) string {
	if enabled {
		return "1"
	}
	return ""
}

func (h *analyticsHarness) setDevelopmentGate(open bool) {
	h.t.Helper()
	h.t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", boolToEnvValue(open))
}

func (h *analyticsHarness) setUserOptOut(optedOut bool) {
	h.t.Helper()
	h.t.Setenv("DO_NOT_TRACK", boolToEnvValue(optedOut))
}

func (h *analyticsHarness) setAnalyticsLogging(enabled bool) {
	h.t.Helper()
	h.t.Setenv("RENDER_LOG_ANALYTICS", boolToEnvValue(enabled))
}

func (h *analyticsHarness) setAnalyticsSubprocessPermission(allowed bool) {
	h.t.Helper()
	h.t.Setenv(analytics.AllowSubprocessInTestsEnv, boolToEnvValue(allowed))
}

func (h *analyticsHarness) setCI(active bool) {
	h.t.Helper()
	h.runtimeSignals.CI = active
	h.t.Setenv("CI", boolToEnvValue(active))
}

func (h *analyticsHarness) setNoticeMarker(present bool) {
	h.t.Helper()

	markerPath := h.noticeMarkerPath()
	if !present {
		if err := os.Remove(markerPath); err != nil {
			require.ErrorIs(h.t, err, os.ErrNotExist)
		}
		return
	}

	require.NoError(h.t, os.MkdirAll(filepath.Dir(markerPath), 0o700))
	require.NoError(h.t, os.WriteFile(markerPath, nil, 0o600))
}

// newInvocation builds fresh dependencies, a fresh Cobra app, and fresh output
// buffers for one invocation. Tests should call [analyticsHarness.execute] or
// [analyticsHarness.complete].
func (h *analyticsHarness) newInvocation() *analyticsInvocation {
	h.t.Helper()

	c, err := client.NewClientWithResponses(h.server.URL())
	require.NoError(h.t, err)
	deps := dependencies.New(c)
	deps.DetectRuntimeSignals = func() (command.RuntimeSignals, error) {
		h.runtimeSignalDetectionCallCount++
		if h.runtimeSignalErr != nil {
			return command.RuntimeSignals{}, h.runtimeSignalErr
		}
		return h.runtimeSignals, nil
	}

	invocation := &analyticsInvocation{
		root: newRootCmd(),
		deps: deps,
	}
	invocation.root.SetOut(&invocation.stdout)
	invocation.root.SetErr(&invocation.stderr)
	setupAnalyticsCommands(invocation.root, deps)
	setupPGCommands(invocation.root, deps)
	setupRootCmdPersistentRun(invocation.root, deps)
	invocation.root.AddCommand(newLoginCmd(), newLogoutCmd())
	invocation.root.AddCommand(
		&cobra.Command{
			Use:  "ping",
			RunE: func(*cobra.Command, []string) error { return nil },
		},
		&cobra.Command{
			Use:    "hidden-user-command",
			Hidden: true,
			RunE:   func(*cobra.Command, []string) error { return nil },
		},
	)
	return invocation
}

// noticeMarkerPath is where the harness's isolated config directory keeps the
// one-time notice marker.
func (h *analyticsHarness) noticeMarkerPath() string {
	return filepath.Join(h.configDir, "state", "analytics", "notice-shown")
}

// requireNoAnalyticsSendState verifies that no installation ID or other
// analytics send state was persisted.
func (h *analyticsHarness) requireNoAnalyticsSendState() {
	h.t.Helper()

	require.NoFileExists(h.t, filepath.Join(h.configDir, "state", "installation-id.txt"),
		"analytics send state must not include an installation ID")

	entries, err := os.ReadDir(filepath.Join(h.configDir, "state", "analytics"))
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	require.NoError(h.t, err)
	entryNames := make([]string, len(entries))
	for i, entry := range entries {
		entryNames[i] = entry.Name()
	}
	require.LessOrEqual(h.t, len(entries), 1,
		"analytics state directory may contain only the disclosure marker; found %v", entryNames)
	if len(entries) == 1 {
		require.Equal(h.t, filepath.Base(h.noticeMarkerPath()), entries[0].Name(),
			"analytics state directory may contain only the disclosure marker; found %v", entryNames)
	}
}

// execute runs the provided args on a fresh instance of the Cobra app.
func (h *analyticsHarness) execute(args ...string) analyticsExecution {
	h.t.Helper()

	invocation := h.newInvocation()
	invocation.root.SetArgs(args)
	result := runExecution(invocation.root, time.Now())
	onExecutionComplete(result, invocation.deps, invocation.root)
	return invocation.result(result)
}

// complete calls onExecutionComplete directly so tests can isolate completion
// handling from command execution and classification.
func (h *analyticsHarness) complete(result command.ExecutionResult) analyticsExecution {
	h.t.Helper()

	invocation := h.newInvocation()
	onExecutionComplete(result, invocation.deps, invocation.root)
	return invocation.result(result)
}

// result snapshots the observable result and output of this invocation. It is
// an internal harness helper; tests receive its value from execute or complete.
func (i *analyticsInvocation) result(result command.ExecutionResult) analyticsExecution {
	return analyticsExecution{
		ExecutionResult: result,
		Stdout:          i.stdout.String(),
		Stderr:          i.stderr.String(),
	}
}

func (e analyticsExecution) countLoggedAnalyticsEvents() int {
	return strings.Count(e.Stderr, `"command":"`+e.CommandPath+`"`)
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
