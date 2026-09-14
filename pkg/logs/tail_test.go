package logs

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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

// TestReadLogsHeartbeat checks that pongs keep a quiet connection alive, missing
// pongs cause a retryable timeout, and cancellation stops the connection.
func TestReadLogsHeartbeat(t *testing.T) {
	type heartbeatTest struct {
		session        *tailSession
		cancel         context.CancelFunc
		readResult     chan error
		serverResult   chan error
		pingCount      atomic.Int32
		respondToPings atomic.Bool
	}

	// Start a quiet connection that answers pings until the test disables replies.
	setup := func(t *testing.T) *heartbeatTest {
		t.Helper()
		client, server := pipeWebSocketPair(t)
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		h := &heartbeatTest{
			session:      &tailSession{delay: 8 * time.Second},
			cancel:       cancel,
			readResult:   make(chan error, 1),
			serverResult: make(chan error, 1),
		}
		h.respondToPings.Store(true)

		defaultPingHandler := server.PingHandler()
		server.SetPingHandler(func(data string) error {
			h.pingCount.Add(1)
			if h.respondToPings.Load() {
				return defaultPingHandler(data)
			}
			return nil
		})

		// Read both sides so WebSocket control frames are processed.
		go func() {
			_, _, err := server.ReadMessage()
			h.serverResult <- err
		}()
		go func() { h.readResult <- h.session.readLogs(ctx, client) }()
		synctest.Wait()
		return h
	}

	// Require both readers to finish and return the client's terminal error.
	requireStopped := func(t *testing.T, h *heartbeatTest) error {
		t.Helper()
		synctest.Wait()
		require.Len(t, h.readResult, 1, "the log reader should have stopped")
		err := <-h.readResult
		require.Error(t, err)
		require.Len(t, h.serverResult, 1, "ending the reader should close the socket")
		require.Error(t, <-h.serverResult)
		return err
	}

	t.Run("no pongs", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			h := setup(t)
			h.respondToPings.Store(false)

			// Sending pings without receiving replies must not extend the deadline.
			time.Sleep(59 * time.Second)
			synctest.Wait()
			require.EqualValues(t, 2, h.pingCount.Load(), "send a ping every twenty seconds")
			require.Empty(t, h.readResult, "allow sixty seconds for the first pong")

			time.Sleep(time.Second)
			err := requireStopped(t, h)
			var networkErr net.Error
			require.ErrorAs(t, err, &networkErr)
			require.True(t, networkErr.Timeout())
			require.True(t, isRetryable(err), "a missing pong should trigger reconnection")
			require.Equal(t, 8*time.Second, h.session.delay, "time connected without a pong or log should preserve backoff")
		})
	})

	t.Run("pongs stop", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			h := setup(t)

			// Three pongs keep the quiet connection alive past its original deadline.
			time.Sleep(60 * time.Second)
			synctest.Wait()
			require.EqualValues(t, 3, h.pingCount.Load())
			require.Empty(t, h.readResult, "pongs should extend the original deadline")

			// Stop replying. The final pong gives the connection sixty more seconds.
			h.respondToPings.Store(false)
			time.Sleep(59 * time.Second)
			synctest.Wait()
			require.Empty(t, h.readResult, "allow sixty seconds from the last pong")

			time.Sleep(time.Second)
			err := requireStopped(t, h)
			var networkErr net.Error
			require.ErrorAs(t, err, &networkErr)
			require.True(t, networkErr.Timeout())
			require.True(t, isRetryable(err), "losing pong replies should trigger reconnection")
			require.Equal(t, time.Second, h.session.delay, "received pongs should reset accumulated backoff")
		})
	})

	t.Run("cancellation", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			h := setup(t)
			time.Sleep(40 * time.Second)
			synctest.Wait()
			require.EqualValues(t, 2, h.pingCount.Load())
			require.Empty(t, h.readResult)

			// Cancellation should stop a responsive connection without advancing time.
			canceledAt := time.Now()
			h.cancel()
			_ = requireStopped(t, h)
			require.Equal(t, canceledAt, time.Now(), "cancellation should not wait for the heartbeat deadline")
		})
	})
}

func TestReadLogsHeartbeatWithBlockedConsumer(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		client, server := pipeWebSocketPair(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		session := &tailSession{
			events:      make(chan Event),
			boundaryIDs: make(map[string]struct{}),
		}
		// Keep processing pings without replying, so the read deadline is the
		// only timeout and we can check when it resumes after event delivery.
		server.SetPingHandler(func(string) error { return nil })
		go func() { _, _, _ = server.ReadMessage() }()
		result := make(chan error, 1)
		go func() { result <- session.readLogs(ctx, client) }()
		time.Sleep(10 * time.Second)
		require.NoError(t, server.WriteJSON(lclient.Log{Id: "blocked", Timestamp: time.Now()}))
		synctest.Wait()

		time.Sleep(70 * time.Second)
		require.Equal(t, "blocked", (<-session.events).Log.Id)
		synctest.Wait()
		require.Empty(t, result, "consumer delay should not exhaust the heartbeat deadline")

		time.Sleep(49 * time.Second)
		synctest.Wait()
		require.Empty(t, result)
		time.Sleep(time.Second)
		synctest.Wait()
		require.Len(t, result, 1, "missing pongs should still time out once reading resumes")
		var networkErr net.Error
		require.ErrorAs(t, <-result, &networkErr)
		require.True(t, networkErr.Timeout())
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
