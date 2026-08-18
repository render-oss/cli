package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/cfg"
	"github.com/render-oss/cli/pkg/client"
	telemetryclient "github.com/render-oss/cli/pkg/client/clitelemetry"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/pointers"
)

var testTerminalSignals = command.TerminalSignals{
	StdinTTY:  true,
	StdoutTTY: true,
	StderrTTY: true,
}

var (
	testAgentSignals = []string{"CLAUDECODE", "CODEX_THREAD_ID"}
	testCISignals    = []string{"CI", "GITHUB_ACTIONS"}
	exampleStartedAt = time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)
)

const testInstallationID = "188796f8-6d3f-4c11-b87d-5e64fbcfe741"

func TestSenderSendAndLogGates(t *testing.T) {
	result := command.ExecutionResult{
		CommandPath:    "render services list",
		CompletionKind: command.CompletionKindExplicitExit,
		Duration:       125 * time.Millisecond,
		ExitCode:       7,
		OutputFormat:   pointers.From(command.JSON),
		StartedAt:      exampleStartedAt,
	}
	payloadJSON, err := json.Marshal(newEventPOSTBody(
		result,
		testTerminalSignals,
		testAgentSignals,
		testCISignals,
		testInstallationID,
		"v-test",
		"test-os",
		"test-arch",
	))
	require.NoError(t, err)
	payloadLog := string(payloadJSON) + "\n"

	testCases := []struct {
		name           string
		sendingEnabled bool
		shouldLog      bool
		wantLog        string
	}{
		{name: "disabled"},
		{
			name:      "logging alone",
			shouldLog: true,
			wantLog:   payloadLog,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configDir := t.TempDir()
			t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
			sender := newTestSender(&fakeTelemetryClient{}, tc.sendingEnabled, tc.shouldLog)
			launcherCalls := 0
			sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
				launcherCalls++
				return &fakeEventFileLauncher{}, nil
			}
			var stderr bytes.Buffer

			sender.Send(result, &stderr)

			require.Equal(t, tc.wantLog, stderr.String())
			require.Zero(t, launcherCalls)
			_, err := os.Stat(filepath.Join(configDir, "state"))
			require.ErrorIs(t, err, os.ErrNotExist, "a non-sending path must not create state")
		})
	}
}

func TestSenderHandsEventFileToDetachedSubprocess(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("CI", "")
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "auto")

	var eventPath string
	launcher := &fakeEventFileLauncher{
		startDetachedFunc: func(path string) error {
			eventPath = path
			info, err := os.Stat(path)
			require.NoError(t, err)
			require.True(t, info.Mode().IsRegular())
			return nil
		},
	}
	sender := newTestSender(&fakeTelemetryClient{}, true, false)
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return launcher, nil
	}

	sender.Send(command.ExecutionResult{
		CommandPath:    "render services list",
		CompletionKind: command.CompletionKindSuccess,
		StartedAt:      exampleStartedAt,
	}, io.Discard)

	require.NotEmpty(t, eventPath)
	require.Equal(t, []string{eventPath}, launcher.detachedPaths)
	require.Empty(t, launcher.syncPaths)
	_, err := os.Stat(eventPath)
	require.NoError(t, err, "the detached child owns the event file after launch")

	data, err := os.ReadFile(eventPath)
	require.NoError(t, err)
	var payload client.CreateCliTelemetryEventJSONRequestBody
	require.NoError(t, json.Unmarshal(data, &payload))
	require.Equal(t, "render services list", payload.Command)
	require.Equal(t, telemetryclient.Success, payload.CompletionKind)
}

func TestSenderRemovesEventFileWhenDetachedLaunchFails(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("CI", "")
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "auto")

	launchErr := errors.New("launch failed")
	launcher := &fakeEventFileLauncher{
		startDetachedFunc: func(string) error { return launchErr },
	}
	sender := newTestSender(&fakeTelemetryClient{}, true, false)
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return launcher, nil
	}

	sender.Send(command.ExecutionResult{}, io.Discard)

	require.Len(t, launcher.detachedPaths, 1)
	_, err := os.Stat(launcher.detachedPaths[0])
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestSenderReportsEventFileAndLauncherFailuresWhenLogging(t *testing.T) {
	t.Run("event file", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
		// Make the expected state directory a regular file so creating
		// state/analytics/events fails.
		require.NoError(t, os.WriteFile(filepath.Join(configDir, "state"), nil, 0o600))

		sender := newTestSender(&fakeTelemetryClient{}, true, true)
		launcher := &fakeEventFileLauncher{}
		sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
			return launcher, nil
		}
		var stderr bytes.Buffer

		sender.Send(command.ExecutionResult{}, &stderr)

		require.Contains(t, stderr.String(), "analytics error: write analytics event file:")
		require.Empty(t, launcher.detachedPaths, "should not start a subprocess if no event file was written")
		require.Empty(t, launcher.syncPaths, "should not start a subprocess if no event file was written")
	})

	t.Run("launcher", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("RENDER_CLI_CONFIG_DIR", configDir)
		t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "banana")
		launcherErr := errors.New("executable unavailable")
		sender := newTestSender(&fakeTelemetryClient{}, true, true)
		sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
			return nil, launcherErr
		}
		installationIDCalls := 0
		sender.getInstallationID = func() (string, error) {
			installationIDCalls++
			return testInstallationID, nil
		}
		var stderr bytes.Buffer

		sender.Send(command.ExecutionResult{}, &stderr)

		require.Contains(t, stderr.String(), "analytics error: "+launcherErr.Error())
		require.Contains(t, stderr.String(), `unknown RENDER_CLI_ANALYTICS_STRATEGY value "banana"; ignoring it`)
		require.Equal(t, 1, installationIDCalls, "logging still builds the analytics payload before reporting the launcher error")
		// Send resolves the launcher before writing, so a refused launch leaves no
		// event file behind — rather than writing one and deleting it again.
		_, err := os.Stat(filepath.Join(configDir, "state", "analytics", "events"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestSilentLauncherFailureDoesNotResolveInstallationID(t *testing.T) {
	launcherErr := errors.New("executable unavailable")
	sender := newTestSender(&fakeTelemetryClient{}, true, false)
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return nil, launcherErr
	}
	installationIDCalls := 0
	sender.getInstallationID = func() (string, error) {
		installationIDCalls++
		return testInstallationID, nil
	}

	sender.Send(command.ExecutionResult{}, io.Discard)

	require.Zero(t, installationIDCalls)
}

func TestSenderPassesDiscardedDiagnosticsToSilentSynchronousSubprocess(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "sync")
	syncErr := errors.New("sync failed after consuming event")
	launcher := &fakeEventFileLauncher{
		runSyncFunc: func(_ context.Context, path string, diagnostics io.Writer) error {
			require.Equal(t, io.Discard, diagnostics, "silent callers must pass io.Discard explicitly")
			require.NoError(t, os.Remove(path))
			return syncErr
		},
	}
	sender := newTestSender(&fakeTelemetryClient{}, true, false)
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return launcher, nil
	}
	var stderr bytes.Buffer

	sender.Send(command.ExecutionResult{}, &stderr)

	require.Len(t, launcher.syncPaths, 1)
	require.Empty(t, stderr.String())
	_, err := os.Stat(launcher.syncPaths[0])
	require.ErrorIs(t, err, os.ErrNotExist, "missing files after child cleanup are expected")
}

func TestSenderWarnsAboutUnknownStrategyWhenLogging(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "banana")
	launcher := &fakeEventFileLauncher{
		runSyncFunc: func(_ context.Context, path string, _ io.Writer) error {
			return os.Remove(path)
		},
	}
	sender := newTestSender(&fakeTelemetryClient{}, true, true)
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return launcher, nil
	}
	var stderr bytes.Buffer

	sender.Send(command.ExecutionResult{}, &stderr)

	require.Contains(t, stderr.String(), `unknown RENDER_CLI_ANALYTICS_STRATEGY value "banana"; ignoring it`)
}

func TestCommandInvokedEventCarriesRuntimeFields(t *testing.T) {
	terminalSignals := command.TerminalSignals{
		StdinTTY:     true,
		StdoutTTY:    false,
		StderrTTY:    true,
		DumbTerminal: true,
	}
	event := newEventPOSTBody(command.ExecutionResult{
		CommandPath:           "render services list",
		CompletionKind:        command.CompletionKindSuccess,
		LaunchedFullScreenTUI: true,
		OutputFormat:          pointers.From(command.YAML),
		StartedAt:             exampleStartedAt,
	}, terminalSignals, testAgentSignals, []string{}, testInstallationID, "v-test", "test-os", "test-arch")

	require.Equal(t, testAgentSignals, event.AgentSignals)
	require.Equal(t, "yaml", event.OutputFormat)
	require.True(t, event.IsStdinTty)
	require.False(t, event.IsStdoutTty)
	require.True(t, event.IsStderrTty)
	require.True(t, event.IsTermDumb)
	require.Equal(t, "render services list", event.Command)
	require.Equal(t, "2026-07-23T12:00:00Z", event.StartedAt)
	require.NotNil(t, event.LaunchedFullScreenTui)
	require.True(t, *event.LaunchedFullScreenTui)

	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"agent_signals": ["CLAUDECODE", "CODEX_THREAD_ID"],
		"arch": "test-arch",
		"ci_signals": [],
		"cli_version": "v-test",
		"command": "render services list",
		"completion_kind": "success",
		"duration_ms": 0,
		"exit_code": 0,
		"installation_id": "`+testInstallationID+`",
		"os": "test-os",
		"output_format": "yaml",
		"started_at": "2026-07-23T12:00:00Z",
		"is_stdin_tty": true,
		"is_stdout_tty": false,
		"is_stderr_tty": true,
		"is_term_dumb": true,
		"launched_full_screen_tui": true
	}`, string(encoded))
}

func TestAgentSignalValuesNeverAppearInSerializedAnalyticsEvent(t *testing.T) {
	const canary = "value-must-not-appear"
	ClearSignalEnvVars(t)
	t.Setenv("CODEX_THREAD_ID", canary)
	t.Setenv("CURSOR_AGENT", canary)

	event := newEventPOSTBody(
		command.ExecutionResult{},
		command.TerminalSignals{},
		DetectAgentSignals(),
		[]string{},
		testInstallationID,
		"v-test",
		"test-os",
		"test-arch",
	)

	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), canary)
	require.Equal(t, []string{"CODEX_THREAD_ID", "CURSOR_AGENT"}, event.AgentSignals)
}

func TestCommandInvokedEventSerializesStartedAt(t *testing.T) {
	event := newEventPOSTBody(
		command.ExecutionResult{StartedAt: exampleStartedAt},
		command.TerminalSignals{},
		[]string{},
		[]string{},
		testInstallationID,
		"v-test",
		"test-os",
		"test-arch",
	)

	encoded, err := json.Marshal(event)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(encoded, &payload))
	require.Equal(t, "2026-07-23T12:00:00Z", payload["started_at"],
		"started_at must be serialized as RFC3339 in the UTC timezone")
}

func TestCommandInvokedEventDefaultsUnresolvedOutputToUnknown(t *testing.T) {
	event := newEventPOSTBody(
		command.ExecutionResult{},
		command.TerminalSignals{},
		[]string{},
		[]string{},
		testInstallationID,
		"v-test",
		"test-os",
		"test-arch",
	)
	require.Equal(t, unknownOutputFormat, event.OutputFormat)
}

func TestDisabledSenderDoesNotGetInstallationID(t *testing.T) {
	sender := newTestSender(&fakeTelemetryClient{}, false, false)
	called := false
	sender.getInstallationID = func() (string, error) {
		called = true
		return testInstallationID, nil
	}

	sender.Send(command.ExecutionResult{}, io.Discard)

	require.False(t, called)
}

func TestInstallationIDResolutionFailureIsBestEffort(t *testing.T) {
	resolutionErr := errors.New("installation ID unavailable")
	runFailedResolution := func(t *testing.T, shouldLog bool) string {
		t.Helper()

		t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
		t.Setenv("CI", "")
		sender := newTestSender(&fakeTelemetryClient{}, true, shouldLog)
		sender.getInstallationID = func() (string, error) {
			return "", resolutionErr
		}
		var payload client.CreateCliTelemetryEventJSONRequestBody
		consumeEvent := func(path string) error {
			payload = readEventPayload(t, path)
			return os.Remove(path)
		}
		sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
			return &fakeEventFileLauncher{
				startDetachedFunc: consumeEvent,
				runSyncFunc: func(_ context.Context, path string, _ io.Writer) error {
					return consumeEvent(path)
				},
			}, nil
		}
		var stderr bytes.Buffer

		sender.Send(command.ExecutionResult{}, &stderr)

		require.Empty(t, payload.InstallationId)
		return stderr.String()
	}

	t.Run("logging disabled", func(t *testing.T) {
		logOutput := runFailedResolution(t, false)
		require.Empty(t, logOutput)
	})

	t.Run("logging enabled", func(t *testing.T) {
		logOutput := runFailedResolution(t, true)
		require.Contains(t, logOutput, "analytics error: getting installation ID: "+resolutionErr.Error())
	})
}

func TestNewUsesExactEnvironmentGates(t *testing.T) {
	testCases := []struct {
		name               string
		sendValue          string
		logValue           string
		doNotTrack         string
		disableEnv         string
		configDisabled     bool
		wantSendingEnabled bool
		wantShouldLog      bool
	}{
		{name: "unset"},
		{name: "non-one values", sendValue: "true", logValue: "true"},
		{name: "logging only", logValue: "1", wantShouldLog: true},
		{name: "sending only", sendValue: "1", wantSendingEnabled: true},
		{name: "both", sendValue: "1", logValue: "1", wantSendingEnabled: true, wantShouldLog: true},
		{name: "DO_NOT_TRACK vetoes an enabled dev gate", sendValue: "1", doNotTrack: "1"},
		{name: "RENDER_CLI_DISABLE_ANALYTICS vetoes an enabled dev gate", sendValue: "1", disableEnv: "true"},
		{name: "config opt-out vetoes an enabled dev gate", sendValue: "1", configDisabled: true},
		{name: "opt-out leaves logging alone", sendValue: "1", logValue: "1", doNotTrack: "1", wantShouldLog: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", tc.sendValue)
			t.Setenv("RENDER_LOG_ANALYTICS", tc.logValue)
			t.Setenv("DO_NOT_TRACK", tc.doNotTrack)
			t.Setenv("RENDER_CLI_DISABLE_ANALYTICS", tc.disableEnv)
			// Consent resolution reads the config file, so isolate it from the
			// machine's real ~/.render/cli.yaml.
			t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
			t.Setenv("RENDER_CLI_CONFIG_PATH", "")
			if tc.configDisabled {
				cfgFile := &config.Config{Analytics: config.AnalyticsConfig{Disabled: true}}
				require.NoError(t, cfgFile.Persist())
			}

			sender := New(&client.ClientWithResponses{})

			require.Equal(t, tc.wantSendingEnabled, sender.sendingEnabled)
			require.Equal(t, tc.wantShouldLog, sender.shouldLog)
		})
	}
}

func TestSenderUsesConfiguredAPIClient(t *testing.T) {
	server := renderapi.NewServer(t)
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("RENDER_CLI_CONFIG_PATH", "")
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "sync")
	t.Setenv("RENDER_TEST_ENABLE_ANALYTICS", "1")
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("RENDER_CLI_DISABLE_ANALYTICS", "")
	t.Setenv("RENDER_HOST", server.URL()+"/")
	t.Setenv("RENDER_API_KEY", "secret-token")
	apiClient, err := client.NewDefaultClient()
	require.NoError(t, err)
	sender := New(apiClient)
	sender.getInstallationID = func() (string, error) {
		return testInstallationID, nil
	}
	launcher := &fakeEventFileLauncher{
		runSyncFunc: func(ctx context.Context, path string, diagnostics io.Writer) error {
			return sender.SendFile(ctx, path, diagnostics)
		},
	}
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return launcher, nil
	}
	terminalSignals := command.DetectTerminalSignals()
	agentSignals := DetectAgentSignals()
	ciSignals := DetectCISignals()

	sender.Send(command.ExecutionResult{
		CommandPath:    "render services list",
		CompletionKind: command.CompletionKindSuccess,
		ExitCode:       0,
		StartedAt:      exampleStartedAt,
	}, io.Discard)

	require.Equal(t, []client.CreateCliTelemetryEventJSONRequestBody{{
		AgentSignals:          agentSignals,
		Arch:                  runtime.GOARCH,
		CiSignals:             ciSignals,
		CliVersion:            cfg.Version,
		Command:               "render services list",
		CompletionKind:        telemetryclient.Success,
		ExitCode:              0,
		IsStdinTty:            terminalSignals.StdinTTY,
		IsStdoutTty:           terminalSignals.StdoutTTY,
		IsStderrTty:           terminalSignals.StderrTTY,
		IsTermDumb:            terminalSignals.DumbTerminal,
		InstallationId:        testInstallationID,
		LaunchedFullScreenTui: pointers.From(false),
		Os:                    runtime.GOOS,
		OutputFormat:          unknownOutputFormat,
		StartedAt:             "2026-07-23T12:00:00Z",
	}}, server.CliTelemetry.Instances)
	require.Len(t, launcher.syncPaths, 1)
	_, err = os.Stat(launcher.syncPaths[0])
	require.ErrorIs(t, err, os.ErrNotExist)
}

// TestCompletionKindsAreKnownToTelemetryAPI guards against local/remote enum
// drift: Send casts a command.CompletionKind straight to the generated
// telemetry enum without validating it. If a new local completion kind is added
// before the API schema knows about it, this test fails until the client is
// regenerated, rather than the CLI silently sending an unknown value.
//
// I am considering getting rid of the server-owned enum, at which point we could delete this test
// The server could simply expect a string here instead, allowing client to own the completion kinds fully
func TestCompletionKindsAreKnownToTelemetryAPI(t *testing.T) {
	for _, kind := range command.CompletionKindValues() {
		t.Run(string(kind), func(t *testing.T) {
			apiKind := telemetryclient.CliTelemetryEventPOSTInputCompletionKind(kind)
			require.Truef(t, apiKind.Valid(),
				"completion kind %q is not a known telemetry enum value; add it to the API schema and regenerate the client", kind)
		})
	}
}

func TestSenderCleansUpAfterSynchronousSubprocessTimeout(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "sync")
	timeoutErr := errors.New("analytics subprocess exceeded 3s deadline")
	launcher := &fakeEventFileLauncher{
		runSyncFunc: func(_ context.Context, _ string, _ io.Writer) error {
			return timeoutErr
		},
	}
	sender := newTestSender(&fakeTelemetryClient{}, true, true)
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return launcher, nil
	}
	var stderr bytes.Buffer

	sender.Send(command.ExecutionResult{CommandPath: "render services list"}, &stderr)

	require.Len(t, launcher.syncPaths, 1)
	require.Same(t, &stderr, launcher.syncDiagnostics[0])
	_, err := os.Stat(launcher.syncPaths[0])
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Contains(t, stderr.String(), "analytics error: "+timeoutErr.Error())
}

func TestAnalyticsLogWriteFailureDoesNotPreventSubprocessOrCleanup(t *testing.T) {
	t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
	launcher := &fakeEventFileLauncher{
		runSyncFunc: func(_ context.Context, path string, _ io.Writer) error {
			return os.Remove(path)
		},
	}
	sender := newTestSender(&fakeTelemetryClient{}, true, true)
	sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
		return launcher, nil
	}

	sender.Send(command.ExecutionResult{CommandPath: "render services list"}, failingWriter{})

	require.Len(t, launcher.syncPaths, 1)
	_, err := os.Stat(launcher.syncPaths[0])
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestHTTPFailureIsSwallowedWithoutRetry(t *testing.T) {
	runFailedSend := func(t *testing.T, shouldLog bool) string {
		t.Helper()
		t.Setenv("RENDER_CLI_CONFIG_DIR", t.TempDir())
		t.Setenv("RENDER_CLI_ANALYTICS_STRATEGY", "sync")

		body := &eventPOSTResponseBody{}
		apiClient := &fakeTelemetryClient{
			response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Body:       body,
			},
		}
		sender := newTestSender(apiClient, true, shouldLog)
		sender.newLauncher = func() (analyticsSubprocessLauncher, error) {
			return &fakeEventFileLauncher{
				runSyncFunc: func(ctx context.Context, path string, diagnostics io.Writer) error {
					return sender.SendFile(ctx, path, diagnostics)
				},
			}, nil
		}
		var stderr bytes.Buffer

		sender.Send(command.ExecutionResult{CommandPath: "render services list"}, &stderr)

		require.Equal(t, 1, apiClient.calls)
		require.True(t, body.closed)
		return stderr.String()
	}

	t.Run("logging enabled", func(t *testing.T) {
		logOutput := runFailedSend(t, true)
		require.Contains(t, logOutput, "analytics error: send analytics event: unexpected response 500 Internal Server Error")
	})

	t.Run("logging disabled", func(t *testing.T) {
		logOutput := runFailedSend(t, false)
		require.Empty(t, logOutput)
	})
}

// newTestSender builds a Sender with fixed environment fields so tests exercise
// the gates and transport without depending on the host's cfg/runtime values.
func newTestSender(apiClient cliTelemetryClient, sendingEnabled, shouldLog bool) *Sender {
	sender := newSender(
		apiClient,
		sendingEnabled,
		shouldLog,
		func() command.TerminalSignals {
			return testTerminalSignals
		},
		func() []string {
			return testAgentSignals
		},
		func() []string {
			return testCISignals
		},
	)
	sender.cliVersion = "v-test"
	sender.goos = "test-os"
	sender.goarch = "test-arch"
	sender.getInstallationID = func() (string, error) {
		return testInstallationID, nil
	}
	return sender
}

type fakeTelemetryClient struct {
	calls    int
	payload  client.CreateCliTelemetryEventJSONRequestBody
	response *http.Response
	err      error
	handler  func(context.Context, client.CreateCliTelemetryEventJSONRequestBody) (*http.Response, error)
}

func (f *fakeTelemetryClient) CreateCliTelemetryEvent(ctx context.Context, payload client.CreateCliTelemetryEventJSONRequestBody, _ ...client.RequestEditorFn) (*http.Response, error) {
	f.calls++
	f.payload = payload
	if f.handler != nil {
		return f.handler(ctx, payload)
	}
	return f.response, f.err
}

type eventPOSTResponseBody struct {
	closed bool
}

func (b *eventPOSTResponseBody) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (b *eventPOSTResponseBody) Close() error {
	b.closed = true
	return nil
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type fakeEventFileLauncher struct {
	detachedPaths     []string
	syncPaths         []string
	syncDiagnostics   []io.Writer
	startDetachedFunc func(string) error
	runSyncFunc       func(context.Context, string, io.Writer) error
}

func (f *fakeEventFileLauncher) startDetached(path string) error {
	f.detachedPaths = append(f.detachedPaths, path)
	if f.startDetachedFunc != nil {
		return f.startDetachedFunc(path)
	}
	return nil
}

func (f *fakeEventFileLauncher) runSync(ctx context.Context, path string, diagnostics io.Writer) error {
	f.syncPaths = append(f.syncPaths, path)
	f.syncDiagnostics = append(f.syncDiagnostics, diagnostics)
	if f.runSyncFunc != nil {
		return f.runSyncFunc(ctx, path, diagnostics)
	}
	return nil
}

func readEventPayload(t *testing.T, path string) client.CreateCliTelemetryEventJSONRequestBody {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var payload client.CreateCliTelemetryEventJSONRequestBody
	require.NoError(t, json.Unmarshal(data, &payload))
	return payload
}
