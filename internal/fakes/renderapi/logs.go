package renderapi

import (
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	logclient "github.com/render-oss/cli/pkg/client/logs"
)

func deadlineSoon() time.Time { return time.Now().Add(time.Second) }

// LogStreamAttempt scripts one websocket connection. A nonzero CloseCode ends
// the attempt with that websocket close status. HoldOpen keeps the connection
// alive until the client cancels it.
type LogStreamAttempt struct {
	Logs      []logclient.Log
	CloseCode int
	HoldOpen  bool
}

// LogResource holds scripted websocket attempts and observable lifecycle
// events for command-level log-tail tests.
type LogResource struct {
	mu       sync.Mutex
	attempts []LogStreamAttempt
	queries  []url.Values
	opened   chan struct{}
	closed   chan struct{}
}

func newLogResource() *LogResource {
	return &LogResource{opened: make(chan struct{}, 16), closed: make(chan struct{}, 16)}
}

func (l *LogResource) QueueStream(attempts ...LogStreamAttempt) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts = append(l.attempts, attempts...)
}

func (l *LogResource) StreamRequestCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.queries)
}

func (l *LogResource) Query(index int) url.Values {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.queries[index]
}

func (l *LogResource) Opened() <-chan struct{} { return l.opened }
func (l *LogResource) Closed() <-chan struct{} { return l.closed }

func (l *LogResource) nextAttempt(query url.Values) LogStreamAttempt {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.queries = append(l.queries, query)
	if len(l.attempts) == 0 {
		return LogStreamAttempt{HoldOpen: true}
	}
	attempt := l.attempts[0]
	l.attempts = l.attempts[1:]
	return attempt
}

func (s *Server) registerLogRoutes(mux *http.ServeMux, record func(*http.Request)) {
	upgrader := websocket.Upgrader{}
	mux.HandleFunc("GET /logs/subscribe", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		attempt := s.Logs.nextAttempt(r.URL.Query())
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		s.Logs.opened <- struct{}{}
		defer func() {
			_ = conn.Close()
			s.Logs.closed <- struct{}{}
		}()

		for i := range attempt.Logs {
			if err := conn.WriteJSON(&attempt.Logs[i]); err != nil {
				return
			}
		}
		if attempt.CloseCode != 0 {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(attempt.CloseCode, "scripted disconnect"),
				deadlineSoon(),
			)
			return
		}
		if !attempt.HoldOpen {
			_ = conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "complete"),
				deadlineSoon(),
			)
			return
		}

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
}
