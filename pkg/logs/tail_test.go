package logs

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/stretchr/testify/require"
)

func setupTailTest(t *testing.T) (*LogRepo, *renderapi.Server) {
	t.Helper()
	server := renderapi.NewServer(t)
	return NewLogRepo(nil, &config.APIConfig{Host: server.URL(), Key: "test-key"}), server
}

func TestTailLogsResume(t *testing.T) {
	timestamp := time.Date(2026, 9, 9, 12, 0, 0, 123456789, time.UTC)
	params := client.ListLogsParams{
		OwnerId: "owner", Resource: []string{"srv-one", "srv-two"},
		Instance: &[]string{"instance"}, Text: &[]string{"hello"}, Level: &[]string{"info"},
		Type: &[]string{"app"}, Host: &[]string{"example.com"}, StatusCode: &[]string{"200"},
		Method: &[]string{"GET"}, Path: &[]string{"/"}, Task: &[]string{"task"}, TaskRun: &[]string{"run"},
		Direction: pointers.From(lclient.Forward),
	}
	repo, server := setupTailTest(t)
	first := renderapi.NewLog(lclient.Log{Id: "one", Timestamp: timestamp})
	second := renderapi.NewLog(lclient.Log{Id: "two", Timestamp: timestamp})
	older := renderapi.NewLog(lclient.Log{Id: "older", Timestamp: timestamp.Add(-time.Second)})
	third := renderapi.NewLog(lclient.Log{Id: "three", Timestamp: timestamp.Add(time.Second)})
	fourth := renderapi.NewLog(lclient.Log{Id: "four", Timestamp: third.Timestamp})
	server.Logs.QueueStreams(
		renderapi.LogStreamConfig{Logs: []lclient.Log{first, second, older}},
		renderapi.LogStreamConfig{Logs: []lclient.Log{first, third, second, fourth}},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var ids []string
	for event := range repo.TailLogs(ctx, &params) {
		require.NoError(t, event.Err)
		entry := event.Log
		if entry == nil {
			continue
		}
		ids = append(ids, entry.Id)
		if entry.Id == "four" {
			break
		}
	}
	cancel()
	require.Equal(t, []string{"one", "two", "older", "three", "four"}, ids)
	queries := server.Logs.Queries()
	require.Len(t, queries, 2)
	require.NotContains(t, queries[0], "startTime", "let the server supply its historical-log default")
	require.NotContains(t, queries[0], "endTime")
	require.Equal(t, timestamp.Format(time.RFC3339Nano), queries[1].Get("startTime"))
	queries[1].Del("startTime")
	require.Equal(t, queries[0], queries[1], "all filters except the resume timestamp should be preserved")
	require.Nil(t, params.StartTime, "caller filters remain unchanged")
}

// TestTailLogsReconnectBeforeFirstLogArrives checks that disconnects before any
// logs arrive preserve the user's --start, or leave it omitted for server defaults.
func TestTailLogsReconnectBeforeFirstLogArrives(t *testing.T) {
	for _, tc := range []struct {
		name  string
		start *time.Time
	}{
		{name: "default start"},
		{name: "explicit start", start: pointers.From(time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, server := setupTailTest(t)
			log := renderapi.NewLog(lclient.Log{})
			server.Logs.QueueStreams(
				renderapi.LogStreamConfig{},
				renderapi.LogStreamConfig{},
				renderapi.LogStreamConfig{Logs: []lclient.Log{log}, HoldOpen: true},
			)
			params := client.ListLogsParams{StartTime: tc.start}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var received *lclient.Log
			for event := range repo.TailLogs(ctx, &params) {
				require.NoError(t, event.Err)
				if event.Log != nil {
					received = event.Log
					cancel()
				}
			}
			require.Equal(t, &log, received)
			queries := server.Logs.Queries()
			require.Len(t, queries, 3)
			if tc.start != nil {
				start, err := time.Parse(time.RFC3339Nano, queries[0].Get("startTime"))
				require.NoError(t, err)
				require.True(t, start.Equal(*tc.start), "the resume timestamp should preserve the user's explicit --start")
			} else {
				require.NotContains(t, queries[0], "startTime", "let the server supply its historical-log default")
			}
			require.Equal(t, queries[0], queries[1], "first empty connection preserves the resume point")
			require.Equal(t, queries[0], queries[2], "second empty connection preserves the resume point")
			require.Equal(t, tc.start, params.StartTime, "caller filters remain unchanged")
		})
	}
}

func TestTailLogsFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		closeCode int
		payload   string
		wantRetry bool
	}{
		{name: "normal closure", closeCode: websocket.CloseNormalClosure, wantRetry: true},
		{name: "going away", closeCode: websocket.CloseGoingAway, wantRetry: true},
		{name: "server restart", closeCode: websocket.CloseServiceRestart, wantRetry: true},
		{name: "rate limit", status: 429, wantRetry: true},
		{name: "unavailable", status: 503, wantRetry: true},
		{name: "unauthorized", status: 401},
		{name: "forbidden", status: 403},
		{name: "invalid handshake", status: 200},
		{name: "policy violation", closeCode: websocket.ClosePolicyViolation},
		{name: "malformed JSON", payload: "{"},
		{name: "missing log fields are accepted", payload: "{}", wantRetry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, server := setupTailTest(t)
			server.Logs.QueueStreams(renderapi.LogStreamConfig{
				HTTPStatus: tc.status, CloseCode: tc.closeCode, RawMessage: tc.payload,
			})
			if tc.wantRetry {
				server.Logs.QueueStreams(renderapi.LogStreamConfig{HTTPStatus: http.StatusForbidden})
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var terminal error
			for event := range repo.TailLogs(ctx, &client.ListLogsParams{}) {
				terminal = event.Err
			}
			require.Error(t, terminal)
			require.NotErrorIs(t, terminal, context.DeadlineExceeded)
			if tc.wantRetry {
				require.Len(t, server.Logs.Queries(), 2)
			} else {
				require.Len(t, server.Logs.Queries(), 1)
			}
		})
	}
}

func TestTailLogsCancellation(t *testing.T) {
	repo, server := setupTailTest(t)
	stream := server.Logs.QueueStreams(renderapi.LogStreamConfig{HoldOpen: true})[0]
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		var terminal error
		for event := range repo.TailLogs(ctx, &client.ListLogsParams{}) {
			terminal = event.Err
		}
		if terminal == nil {
			// Expected on cancellation: TailLogs closes the channel without an
			// error event, so the context supplies the reason the tail ended.
			terminal = ctx.Err()
		}
		done <- terminal
	}()
	select {
	case <-stream.Opened():
	case <-time.After(3 * time.Second):
		t.Fatal("connection did not open")
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("cancellation did not stop idle read")
	}
	select {
	case <-stream.Closed():
	case <-time.After(time.Second):
		t.Fatal("socket remained open")
	}
	require.Len(t, server.Logs.Queries(), 1)
}

// TestReadLogsRetryDelay checks that delivering a log resets accumulated backoff
// without waiting for a heartbeat or a minimum connection duration.
func TestReadLogsRetryDelay(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		client, server := pipeWebSocketPair(t)
		session := &tailSession{
			delay:       8 * time.Second,
			events:      make(chan Event),
			boundaryIDs: make(map[string]struct{}),
		}
		readResult := make(chan error, 1)
		go func() { readResult <- session.readLogs(t.Context(), client) }()
		synctest.Wait()

		log := renderapi.NewLog(lclient.Log{Timestamp: time.Now().UTC()})
		require.NoError(t, server.WriteJSON(log))
		require.Equal(t, &log, (<-session.events).Log)

		require.NoError(t, server.Close())
		synctest.Wait()
		require.Len(t, readResult, 1)
		require.Error(t, <-readResult)
		require.Equal(t, time.Second, session.delay, "a delivered log should reset accumulated backoff")
	})
}

// pipeWebSocketPair performs a WebSocket handshake over an in-memory connection.
// It returns (client, server).
// Unlike TCP sockets, net.Pipe lets synctest advance time while reads are blocked.
func pipeWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})
	serverChan := make(chan *websocket.Conn, 1)
	go func() {
		reader := bufio.NewReader(serverConn)
		req, err := http.ReadRequest(reader)
		if err != nil {
			t.Errorf("read WebSocket handshake: %v", err)
			_ = serverConn.Close()
			return
		}
		writer := &pipeResponseWriter{ResponseRecorder: httptest.NewRecorder(), conn: serverConn, reader: reader}
		peer, err := (&websocket.Upgrader{}).Upgrade(writer, req, nil)
		if err != nil {
			t.Errorf("upgrade WebSocket connection: %v", err)
			_ = serverConn.Close()
			return
		}
		serverChan <- peer
	}()
	dialer := websocket.Dialer{NetDialContext: func(context.Context, string, string) (net.Conn, error) {
		return clientConn, nil
	}}
	client, _, err := dialer.DialContext(t.Context(), "ws://logs.test/", nil)
	require.NoError(t, err)
	return client, <-serverChan
}

// pipeResponseWriter lets the WebSocket upgrader take ownership of the pipe.
type pipeResponseWriter struct {
	*httptest.ResponseRecorder
	conn   net.Conn
	reader *bufio.Reader
}

// Hijack implements http.Hijacker so the upgrader can switch from HTTP to
// WebSocket on the pipe. It returns the connection, buffered reader/writer,
// and no error.
func (w *pipeResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, bufio.NewReadWriter(w.reader, bufio.NewWriter(w.conn)), nil
}

func TestTailLogsRetriesUntilCancellation(t *testing.T) {
	repo, server := setupTailTest(t)
	server.Logs.QueueStreams(
		renderapi.LogStreamConfig{HTTPStatus: 503},
		renderapi.LogStreamConfig{HTTPStatus: 503},
		renderapi.LogStreamConfig{HTTPStatus: 503},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		var terminal error
		for event := range repo.TailLogs(ctx, &client.ListLogsParams{}) {
			terminal = event.Err
		}
		if terminal == nil {
			terminal = ctx.Err()
		}
		done <- terminal
	}()
	require.Eventually(t, func() bool { return len(server.Logs.Queries()) == 3 }, 10*time.Second, 10*time.Millisecond,
		"exhausting several failed handshakes must leave the tail retrying")
	select {
	case err := <-done:
		t.Fatalf("tail stopped retrying: %v", err)
	default:
	}
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt retry wait")
	}
	require.Len(t, server.Logs.Queries(), 3)
}
