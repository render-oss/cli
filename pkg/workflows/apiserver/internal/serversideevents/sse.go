package serversideevents

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	RetryInterval     = 2000
	HeartbeatInterval = 25 * time.Second
	LastEventIDHeader = "Last-Event-ID"
)

type Input struct {
	LastEventID string
	Request     *http.Request
}

type Message[T any] struct {
	ID    uint64
	Event *string
	Data  T
}

func ServerSideEvents[T any](ch <-chan Message[T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Required SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Ensure we can flush
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// If client provided Last-Event-ID, you can resume from there
		lastID := r.Header.Get(LastEventIDHeader)
		if lastID != "" {
			log.Printf("Client resumed from id=%s", lastID)
		}

		// Send a recommended retry value (ms) so EventSource backs off sanely
		fmt.Fprintf(w, "retry: %d\n\n", RetryInterval)
		flusher.Flush()

		ctx := r.Context()

		// Heartbeat interval (send comment lines so proxies keep the connection)
		heartbeat := time.NewTicker(HeartbeatInterval)
		defer heartbeat.Stop()

		for {
			select {
			case <-ctx.Done():
				// Client disconnected or server shutting down
				return

			case <-heartbeat.C:
				// Comment lines start with ':' and are ignored by the browser
				fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()

			case t := <-ch:
				b, _ := json.Marshal(t.Data)

				fmt.Fprintf(w, "id: %d\n", t.ID)
				if t.Event != nil {
					fmt.Fprintf(w, "event: %s\n", *t.Event)
				}
				fmt.Fprintf(w, "data: %s\n\n", b)
				flusher.Flush()
			}
		}
	}
}
