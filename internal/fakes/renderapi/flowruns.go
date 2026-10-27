package renderapi

import (
	"encoding/json"
	"net/http"
)

// FlowRun is a flow run as the API returns it. Body holds the raw JSON so
// tests describe the wire format instead of the CLI's own types.
type FlowRun struct {
	Id   string
	Body json.RawMessage
}

// registerFlowRunRoutes wires GET /flow-runs/{flowRunId} on the shared mux.
func registerFlowRunRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("GET /flow-runs/{flowRunId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("flowRunId")
		for _, run := range s.FlowRuns.Instances {
			if run.Id == id {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(run.Body)
				return
			}
		}
		message := "flow run not found"
		writeJSON(w, http.StatusNotFound, map[string]string{"message": message})
	})
}
