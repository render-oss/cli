package renderapi

import (
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/render-oss/cli/pkg/client"
	lclient "github.com/render-oss/cli/pkg/client/logs"
)

// NewLog creates a Log with a populated message, timestamp, and unique ID.
// Non-zero values from the supplied entry are used; defaults are supplied otherwise.
// The default timestamp is a random time within the last 24 hours. Tests that
// depend on ordering or replay boundaries should supply explicit timestamps.
func NewLog(entry lclient.Log) lclient.Log {
	if entry.Id == "" {
		entry.Id = uuid.NewString()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC().Add(-time.Duration(rand.Int64N(int64(24 * time.Hour))))
	}
	if entry.Message == "" {
		entry.Message = "test log " + entry.Id
	}
	return entry
}

// LogStreamConfig configures the fake response to one log subscription.
// The zero value hangs up immediately after sending the scripted data, without
// a close frame. Use HoldOpen to model a healthy production subscription that
// stays connected while waiting for more logs.
type LogStreamConfig struct {
	// Logs are sent in slice order after the WebSocket handshake succeeds.
	Logs []lclient.Log
	// HTTPStatus, when nonzero, responds with this HTTP status instead of
	// upgrading. All other fields are ignored; use 401/403 for auth failures.
	HTTPStatus int
	// CloseCode sends a WebSocket close frame after the messages. Zero closes
	// the transport without a frame, simulating an abrupt disconnect.
	CloseCode int
	// RawMessage sends one unencoded text frame after Logs. Use it to test
	// malformed JSON. An empty string sends no additional frame.
	RawMessage string
	// HoldOpen waits for the client to disconnect after sending the messages.
	// It takes precedence over CloseCode; use it to test idle cancellation.
	HoldOpen bool
}

// LogStream tracks the lifecycle of one queued subscription.
// Failed handshakes leave both lifecycle channels open because no WebSocket
// connection was established.
type LogStream struct {
	config LogStreamConfig
	opened chan struct{}
	closed chan struct{}
}

// Opened is closed after the WebSocket handshake succeeds.
func (s *LogStream) Opened() <-chan struct{} { return s.opened }

// Closed is closed after the handler closes the established WebSocket.
func (s *LogStream) Closed() <-chan struct{} { return s.closed }

// LogResource stores logs for GET /logs. Seed them with Add before making requests.
// Listing returns all stored logs; query filtering is not implemented.
// RespondWithRawList overrides a list response for exceptional-response tests.
//
// GET /logs/subscribe uses QueueStreams independently of the stored logs because
// streaming tests need explicit control over message order, timing, and disconnects.
type LogResource struct {
	Resource[lclient.Log]
	t testing.TB
	// mu protects scripted responses, queries, and connection cleanup state.
	mu sync.Mutex
	// rawListResponses contains queued overrides for GET /logs.
	rawListResponses []rawLogListResponse
	// streams contains queued responses; each subscription consumes the first.
	streams []*LogStream
	// queries records filters for every attempt, including failed handshakes.
	queries []url.Values
	// connections tracks upgraded sockets, which httptest.Server does not close.
	connections map[*websocket.Conn]struct{}
	closing     bool
}

func (l *LogResource) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closing = true
	for conn := range l.connections {
		_ = conn.Close()
	}
}

type rawLogListResponse struct {
	contentType string
	body        []byte
}

// RespondWithRawList queues an unparsed 200 response from GET /logs.
// Once queued overrides are consumed, listing returns the stored logs again.
func (l *LogResource) RespondWithRawList(contentType string, body []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.rawListResponses = append(l.rawListResponses, rawLogListResponse{
		contentType: contentType,
		body:        slices.Clone(body),
	})
}

// nextRawListResponse returns the next override, or nil to use the stored logs.
func (l *LogResource) nextRawListResponse() *rawLogListResponse {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.rawListResponses) == 0 {
		return nil
	}
	response := l.rawListResponses[0]
	l.rawListResponses = l.rawListResponses[1:]
	return &response
}

// QueueStreams appends responses for successive subscriptions.
// Subscriptions use these scripts rather than the logs stored with Add.
// It returns lifecycle handles in the same order as the supplied configurations.
// Test authors must script all log streams via this method. An unscripted
// request fails the test and receives HTTP 403 to stop accidental reconnects.
func (l *LogResource) QueueStreams(configs ...LogStreamConfig) []*LogStream {
	l.mu.Lock()
	defer l.mu.Unlock()
	streams := make([]*LogStream, len(configs))
	for i, config := range configs {
		streams[i] = &LogStream{config: config, opened: make(chan struct{}), closed: make(chan struct{})}
	}
	l.streams = append(l.streams, streams...)
	return streams
}

// Queries returns the filters received on each subscription attempt for
// assertions about filter preservation, resume timestamps, and attempt counts.
// Copies allow tests to compare or modify the result without racing handlers.
func (l *LogResource) Queries() []url.Values {
	l.mu.Lock()
	defer l.mu.Unlock()
	queries := make([]url.Values, len(l.queries))
	for i, query := range l.queries {
		queries[i] = make(url.Values)
		for key, values := range query {
			queries[i][key] = slices.Clone(values)
		}
	}
	return queries
}

// next records the request filters and consumes the next queued response.
// With no queued response, it reports a test failure and returns an HTTP 403
// configuration to stop accidental reconnects promptly.
func (l *LogResource) next(query url.Values) *LogStream {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, query)
	if len(l.streams) == 0 {
		l.t.Errorf("unscripted log subscription: %s", query.Encode())
		return &LogStream{config: LogStreamConfig{HTTPStatus: http.StatusForbidden}}
	}
	stream := l.streams[0]
	l.streams = l.streams[1:]
	return stream
}

func registerLogRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("GET /logs", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		s.handleListLogs(w, r)
	})
	mux.HandleFunc("GET /logs/subscribe", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		s.handleSubscribeLogs(w, r)
	})
}

func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	if response := s.Logs.nextRawListResponse(); response != nil {
		w.Header().Set("Content-Type", response.contentType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(response.body)
		return
	}
	writeJSON(w, http.StatusOK, client.Logs200Response{Logs: append([]lclient.Log{}, s.Logs.Instances...)})
}

func (s *Server) handleSubscribeLogs(w http.ResponseWriter, r *http.Request) {
	stream := s.Logs.next(r.URL.Query())
	if stream.config.HTTPStatus != 0 {
		http.Error(w, "scripted handshake failure", stream.config.HTTPStatus)
		return
	}
	conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
		s.Logs.mu.Lock()
		delete(s.Logs.connections, conn)
		s.Logs.mu.Unlock()
		close(stream.closed)
	}()
	close(stream.opened)
	s.Logs.mu.Lock()
	if s.Logs.closing {
		s.Logs.mu.Unlock()
		return
	}
	if s.Logs.connections == nil {
		s.Logs.connections = make(map[*websocket.Conn]struct{})
	}
	s.Logs.connections[conn] = struct{}{}
	s.Logs.mu.Unlock()
	stream.run(conn)
}

// run sends the scripted messages and applies the configured disconnect behavior.
// The caller owns the connection and closes it when run returns.
func (s *LogStream) run(conn *websocket.Conn) {
	for _, entry := range s.config.Logs {
		if err := conn.WriteJSON(entry); err != nil {
			return
		}
	}
	if s.config.RawMessage != "" {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(s.config.RawMessage)); err != nil {
			return
		}
	}
	if s.config.HoldOpen {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}
	if s.config.CloseCode != 0 {
		if err := conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(s.config.CloseCode, "scripted close"), time.Now().Add(time.Second)); err != nil {
			return
		}
		// Give the client a bounded opportunity to acknowledge the close
		// before closing the transport, consuming any pending control frames.
		if err := conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return
		}
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}
}
