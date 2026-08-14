package renderapi

import (
	"encoding/json"
	"net/http"

	"github.com/render-oss/cli/pkg/client"
)

// CliTelemetryResource collects posted CLI telemetry events and lets tests
// program the responses the endpoint returns.
type CliTelemetryResource struct {
	Resource[client.CreateCliTelemetryEventJSONRequestBody]
	responseQueue []cliTelemetryResponse
}

type cliTelemetryResponse struct {
	status     int
	retryAfter string
}

// RespondWith queues an HTTP status code to return on the next telemetry post.
// Use this to simulate API errors; the queue is drained in FIFO order.
func (c *CliTelemetryResource) RespondWith(status int) {
	c.responseQueue = append(c.responseQueue, cliTelemetryResponse{status: status})
}

// RespondWithRetryAfter queues an HTTP status code carrying a Retry-After
// header. The value is sent verbatim so tests can exercise delta-seconds,
// HTTP-dates, and values the client should refuse.
func (c *CliTelemetryResource) RespondWithRetryAfter(status int, retryAfter string) {
	c.responseQueue = append(c.responseQueue, cliTelemetryResponse{status: status, retryAfter: retryAfter})
}

// nextResponse removes and returns the response at the front of the queue. The
// second value reports whether one was queued at all; when it is false the
// handler answers normally instead.
func (c *CliTelemetryResource) nextResponse() (cliTelemetryResponse, bool) {
	if len(c.responseQueue) == 0 {
		return cliTelemetryResponse{}, false
	}
	response := c.responseQueue[0]
	c.responseQueue = c.responseQueue[1:]
	return response, true
}

// registerCliTelemetryRoutes wires POST /cli-telemetry-events on the shared mux.
// It collects each event body on s.CliTelemetry for later assertion and mirrors
// the real API by returning 202 Accepted.
func registerCliTelemetryRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /cli-telemetry-events", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if response, queued := s.CliTelemetry.nextResponse(); queued {
			if response.retryAfter != "" {
				w.Header().Set("Retry-After", response.retryAfter)
			}
			// Do not add an event resource, because the server did not accept it.
			w.WriteHeader(response.status)
			return
		}
		var event client.CreateCliTelemetryEventJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.CliTelemetry.Add(event)
		w.WriteHeader(http.StatusAccepted)
	})
}
