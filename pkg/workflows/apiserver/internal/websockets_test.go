package internal_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/pkg/workflows/apiserver/internal"
	"github.com/stretchr/testify/require"
)

func TestWebsocketChannelWrapper(t *testing.T) {
	t.Run("read and write", func(t *testing.T) {
		ws := setupTestWSServer()(t, func(conn *websocket.Conn) {
			read, write := internal.WebsocketChannelWrapper(conn)

			data := <-read
			require.Equal(t, websocket.TextMessage, data.MessageType)
			require.Equal(t, []byte("hello"), data.Data)

			write <- internal.WebSocketData{MessageType: websocket.TextMessage, Data: []byte("world")}
		})

		err := ws.WriteMessage(websocket.TextMessage, []byte("hello"))
		require.NoError(t, err)

		messageType, data, err := ws.ReadMessage()
		require.NoError(t, err)
		require.Equal(t, websocket.TextMessage, messageType)
		require.Equal(t, []byte("world"), data)

		err = ws.Close()
	})

	t.Run("closing write socket gracefully closes", func(t *testing.T) {
		ws := setupTestWSServer()(t, func(conn *websocket.Conn) {
			_, write := internal.WebsocketChannelWrapper(conn)

			close(write)
		})

		_, _, err := ws.ReadMessage()
		require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))

		err = ws.Close()
	})

	t.Run("writing close message closes channels", func(t *testing.T) {
		ws := setupTestWSServer()(t, func(conn *websocket.Conn) {
			read, write := internal.WebsocketChannelWrapper(conn)

			write <- internal.WebSocketData{
				MessageType: websocket.CloseMessage,
				Data:        websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing"),
			}

			_, ok := <-read
			require.False(t, ok)
		})

		_, _, err := ws.ReadMessage()
		require.True(t, websocket.IsCloseError(err, websocket.CloseNormalClosure))

		err = ws.Close()
	})

	t.Run("read error closes channels", func(t *testing.T) {
		ws := setupTestWSServer()(t, func(conn *websocket.Conn) {
			read, _ := internal.WebsocketChannelWrapper(conn)

			_, ok := <-read
			require.False(t, ok)
		})

		require.NoError(t, ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "closing")))
		require.NoError(t, ws.Close())
	})
}

func setupTestWSServer() func(t *testing.T, fn func(conn *websocket.Conn)) *websocket.Conn {
	wsDial := func(t *testing.T, url string) *websocket.Conn {
		t.Helper()

		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })

		return conn
	}

	return func(t *testing.T, fn func(conn *websocket.Conn)) *websocket.Conn {
		t.Helper()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })

			fn(conn)
		}))
		t.Cleanup(func() { srv.Close() })

		url, err := url.Parse(srv.URL)
		require.NoError(t, err)

		url.Scheme = "ws"

		return wsDial(t, url.String())
	}
}
