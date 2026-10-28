package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/command"
	"github.com/render-oss/cli/pkg/dependencies"
	"github.com/stretchr/testify/require"
)

func TestLogsTailReconnect(t *testing.T) {
	for _, format := range []string{"text", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			server := renderapi.NewServer(t)
			first := renderapi.NewLog(lclient.Log{Message: "first marker"})
			second := renderapi.NewLog(lclient.Log{Message: "second marker", Timestamp: first.Timestamp})
			server.Logs.QueueStreams(
				renderapi.LogStreamConfig{Logs: []lclient.Log{first}, CloseCode: websocket.CloseNormalClosure},
				renderapi.LogStreamConfig{Logs: []lclient.Log{first, second}, RawMessage: "invalid JSON"},
			)

			result, err := executeLogsCommand(t, server, "--tail", "--output", format)

			require.Len(t, server.Logs.Queries(), 2, "peer closure should reconnect")
			require.Equal(t, 1, strings.Count(result.Stdout, "first marker"), "replayed log should appear once")
			require.Contains(t, result.Stdout, "second marker")
			require.Contains(t, result.Stderr, "Unable to connect to logs. Retrying in 1s.")
			require.Contains(t, result.Stderr, "Log connection restored.")
			require.NotContains(t, result.Stdout, "Unable to connect to logs")
			require.NotContains(t, result.Stdout, "connection restored")
			require.ErrorContains(t, err, "decode tailed log")
			require.NotZero(t, exitCodeFromError(err), "malformed logs should fail the command")
		})
	}
}

func TestLogsTailAndEndAreMutuallyExclusive(t *testing.T) {
	server := renderapi.NewServer(t)

	_, err := executeLogsCommand(t, server, "--tail", "--end", "2026-09-09T12:00:00Z", "--output", "text")

	require.ErrorContains(t, err, "[tail end]")
	require.NotZero(t, exitCodeFromError(err))
	require.Empty(t, server.Requests, "reject conflicting flags before making API requests")
}

func TestNonInteractiveLogsMissingJSONResponse(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Logs.RespondWithRawList("text/plain", []byte("unexpected response"))

	result, err := executeLogsCommand(t, server, "--output", "text")

	require.NoError(t, err)
	require.Empty(t, result.Stdout)
}

func executeLogsCommand(t *testing.T, server *renderapi.Server, args ...string) (CommandResult, error) {
	t.Helper()
	t.Setenv("RENDER_CLI_CONFIG_PATH", newTestConfigPath(t))
	t.Setenv("RENDER_API_KEY", "test-key")
	t.Setenv("RENDER_HOST", server.URL())
	t.Setenv("RENDER_WORKSPACE", testids.WorkspaceID("logs"))
	api, err := client.NewClientWithResponses(server.URL())
	require.NoError(t, err)
	deps := dependencies.New(api)
	root := newRootCmd()
	deps.Logs.LogsCmd = NewLogsCmd(deps)
	root.AddCommand(deps.Logs.LogsCmd)
	setupRootCmdPersistentRun(root, deps)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	root.SetContext(ctx)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"logs", "--resources", testids.ServiceID("logs")}, args...))
	err = root.Execute()
	return CommandResult{Stdout: stdout.String(), Stderr: stderr.String()}, err
}

func TestWriteLogTextSanitizesControlBytes(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	log := &lclient.Log{
		Id:        "log-1",
		Timestamp: ts,
		Message:   "before\x1b[1A\x1b[2K\x1b]8;;https://attacker.example\x1b\\link\x1b]8;;\x1b\\\x1b]52;c;QUJD\x07\rafter",
	}

	var buf bytes.Buffer
	require.NoError(t, writeLog(command.TEXT, &buf, log))
	require.Equal(t, "2026-01-02 03:04:05  beforelinkafter\n", buf.String())

	// Structured formats already escape control bytes and must keep the
	// message intact.
	buf.Reset()
	require.NoError(t, writeLog(command.JSON, &buf, log))
	require.NotContains(t, buf.String(), "\x1b")
	require.Contains(t, buf.String(), `\u001b[1A`)
}

func TestWriteLogTextPreservesMessageInRegularFile(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	message := "before\x1b[1A\x1b[2K\rafter"

	path := filepath.Join(t.TempDir(), "logs.txt")
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, writeLog(command.TEXT, f, &lclient.Log{Timestamp: ts, Message: message}))
	require.NoError(t, f.Close())

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "2026-01-02 03:04:05  "+message+"\n", string(got))
}
