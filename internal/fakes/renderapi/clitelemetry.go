package renderapi

import (
	"encoding/json"
	"net/http"

	"github.com/render-oss/cli/pkg/client"
)

// registerCliTelemetryRoutes wires POST /cli-telemetry-events on the shared mux.
// It collects each event body on s.CliTelemetry for later assertion and mirrors
// the real API by returning 202 Accepted.
func registerCliTelemetryRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /cli-telemetry-events", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		var event client.CreateCliTelemetryEventJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.CliTelemetry.Add(event)
		w.WriteHeader(http.StatusAccepted)
	})
}
