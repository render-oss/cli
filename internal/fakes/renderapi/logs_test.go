package renderapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/internal/fakes/renderapi"
	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
	"github.com/stretchr/testify/require"
)

// TestScriptedLogHandshakeFailure checks that a scripted HTTP failure rejects the
// handshake without signaling a WebSocket lifecycle event.
func TestScriptedLogHandshakeFailure(t *testing.T) {
	server := renderapi.NewServer(t)
	stream := server.Logs.QueueStreams(renderapi.LogStreamConfig{HTTPStatus: http.StatusServiceUnavailable})[0]

	_, response, err := dialLogSubscription(t, server)
	require.ErrorIs(t, err, websocket.ErrBadHandshake)
	require.Equal(t, http.StatusServiceUnavailable, response.StatusCode)
	require.NoError(t, response.Body.Close())
	select {
	case <-stream.Opened():
		t.Fatal("failed handshake signaled an open WebSocket")
	case <-stream.Closed():
		t.Fatal("failed handshake signaled a closed WebSocket")
	default:
	}
}

// TestScriptedLogDelivery checks that a scripted log reaches the client unchanged.
func TestScriptedLogDelivery(t *testing.T) {
	server := renderapi.NewServer(t)
	entry := renderapi.NewLog(lclient.Log{Message: "hello"})
	server.Logs.QueueStreams(renderapi.LogStreamConfig{Logs: []lclient.Log{entry}, HoldOpen: true})

	conn, _, err := dialLogSubscription(t, server)
	require.NoError(t, err)
	var got lclient.Log
	require.NoError(t, conn.ReadJSON(&got))
	require.Equal(t, entry, got)
}

// TestScriptedLogRawMessage checks that malformed JSON is delivered verbatim
// as a WebSocket text frame.
func TestScriptedLogRawMessage(t *testing.T) {
	server := renderapi.NewServer(t)
	const rawMessage = "{invalid JSON"
	server.Logs.QueueStreams(renderapi.LogStreamConfig{RawMessage: rawMessage, HoldOpen: true})

	conn, _, err := dialLogSubscription(t, server)
	require.NoError(t, err)
	messageType, message, err := conn.ReadMessage()
	require.NoError(t, err)
	require.Equal(t, websocket.TextMessage, messageType)
	require.Equal(t, rawMessage, string(message))
}

// TestScriptedLogLifecycle checks that a held-open subscription signals opening
// and signals closure after the client disconnects.
func TestScriptedLogLifecycle(t *testing.T) {
	server := renderapi.NewServer(t)
	stream := server.Logs.QueueStreams(renderapi.LogStreamConfig{HoldOpen: true})[0]

	conn, _, err := dialLogSubscription(t, server)
	require.NoError(t, err)
	select {
	case _, ok := <-stream.Opened():
		require.False(t, ok, "opened channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("subscription did not open")
	}
	require.NoError(t, conn.Close())
	select {
	case _, ok := <-stream.Closed():
		require.False(t, ok, "closed channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("subscription did not close")
	}
}

// TestScriptedLogSubscriptionOrderAndQueries checks that attempts consume scripts
// in order and record resource filters for successful and failed handshakes.
func TestScriptedLogSubscriptionOrderAndQueries(t *testing.T) {
	server := renderapi.NewServer(t)
	server.Logs.QueueStreams(
		renderapi.LogStreamConfig{HTTPStatus: http.StatusServiceUnavailable},
		renderapi.LogStreamConfig{HoldOpen: true},
		renderapi.LogStreamConfig{HTTPStatus: http.StatusForbidden},
	)

	for _, status := range []int{http.StatusServiceUnavailable, http.StatusSwitchingProtocols, http.StatusForbidden} {
		conn, response, err := dialLogSubscription(t, server)
		if status == http.StatusSwitchingProtocols {
			require.NoError(t, err)
			require.NoError(t, conn.Close())
		} else {
			require.ErrorIs(t, err, websocket.ErrBadHandshake)
			require.NoError(t, response.Body.Close())
		}
		require.Equal(t, status, response.StatusCode)
	}
	queries := server.Logs.Queries()
	require.Len(t, queries, 3)
	for _, query := range queries {
		require.Equal(t, []string{"srv-one", "srv-two"}, query["resource"])
	}
}

func dialLogSubscription(t *testing.T, server *renderapi.Server) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	req, err := client.NewSubscribeLogsRequest(server.URL(), &client.SubscribeLogsParams{
		OwnerId: "owner", Resource: []string{"srv-one", "srv-two"},
	})
	require.NoError(t, err)
	req.URL.Scheme = "ws"
	dialer := websocket.Dialer{HandshakeTimeout: time.Second}
	conn, response, err := dialer.Dial(req.URL.String(), nil)
	if err == nil {
		t.Cleanup(func() { _ = conn.Close() })
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
	}
	return conn, response, err
}

// TestScriptedLogCloseCode checks that closing the transport after a scripted
// close frame preserves the code observed by the client.
func TestScriptedLogCloseCode(t *testing.T) {
	for _, sendPong := range []bool{false, true} {
		name := "quiet client"
		if sendPong {
			name = "client sends pong before reading"
		}
		t.Run(name, func(t *testing.T) {
			server := renderapi.NewServer(t)
			server.Logs.QueueStreams(renderapi.LogStreamConfig{CloseCode: websocket.CloseGoingAway})
			req, err := client.NewSubscribeLogsRequest(server.URL(), &client.SubscribeLogsParams{
				OwnerId: "owner", Resource: []string{"srv-one"},
			})
			require.NoError(t, err)
			req.URL.Scheme = "ws"
			dialer := websocket.Dialer{HandshakeTimeout: time.Second}
			conn, _, err := dialer.Dial(req.URL.String(), nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })
			if sendPong {
				// The server may already have closed. A failed write doesn't
				// change the close code we expect to receive below.
				if err := conn.WriteControl(websocket.PongMessage, []byte("pong"), time.Now().Add(time.Second)); err != nil {
					t.Logf("pong write after server shutdown: %v", err)
				}
			}
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(time.Second)))
			_, _, err = conn.ReadMessage()
			require.True(t, websocket.IsCloseError(err, websocket.CloseGoingAway), "expected scripted close code, got %v", err)
		})
	}
}

// TestLogStreamCleanup checks that server cleanup closes a subscription
// even when the client leaves its connection open.
func TestLogStreamCleanup(t *testing.T) {
	var stream *renderapi.LogStream
	parent := t
	require.True(t, t.Run("leave client connected", func(t *testing.T) {
		server := renderapi.NewServer(t)
		stream = server.Logs.QueueStreams(renderapi.LogStreamConfig{HoldOpen: true})[0]
		req, err := client.NewSubscribeLogsRequest(server.URL(), &client.SubscribeLogsParams{
			OwnerId: "owner", Resource: []string{"srv-one"},
		})
		require.NoError(t, err)
		req.URL.Scheme = "ws"
		dialer := websocket.Dialer{HandshakeTimeout: time.Second}
		conn, _, err := dialer.Dial(req.URL.String(), nil)
		require.NoError(t, err)
		// Keep the client connected through subtest cleanup; close it even
		// if the server cleanup assertion below fails.
		parent.Cleanup(func() { _ = conn.Close() })
		select {
		case <-stream.Opened():
		case <-time.After(time.Second):
			t.Fatal("subscription did not open")
		}
	}))
	select {
	case <-stream.Closed():
	case <-time.After(time.Second):
		t.Fatal("server cleanup did not close the subscription")
	}
}
