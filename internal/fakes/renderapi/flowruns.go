package renderapi

import (
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strconv"
)

// FlowRun is a flow run as the API returns it. Body holds the raw JSON so
// tests describe the wire format instead of the CLI's own types.
type FlowRun struct {
	Id   string
	Body json.RawMessage
}

// Flow is a flow definition as the API returns it. Body holds the raw JSON for
// the same reasons as FlowRun.
type Flow struct {
	Id   string
	Body json.RawMessage
}

// FlowRunStep is a flow run step record. Id combines the parent flow run ID and
// the step's sequence number so handlers can look it up by both.
type FlowRunStep struct {
	FlowRunID string
	Seq       int
	Body      json.RawMessage
}

// FlowResource bundles flow definitions and the list of calls made against the
// list endpoint. Tests inspect ListCalls to assert on filter wiring.
type FlowResource struct {
	Resource[*Flow]
	ListCalls []FlowListCall
}

// FlowListCall captures the query parameters the fake saw for one list call.
type FlowListCall struct {
	OwnerIds          []string
	FlowSlugs         []string
	WorkflowIds       []string
	WorkflowVersionId []string
	Limit             string
	Cursor            string
}

// FlowRunResource bundles flow run records and list-call capture for assertions.
type FlowRunResource struct {
	Resource[*FlowRun]
	Steps         []*FlowRunStep
	ListCalls     []FlowRunListCall
	StepListCalls []FlowRunStepListCall
	StartedRuns   []json.RawMessage
	StartResults  []*FlowRun
	// nextStartRun, when non-nil, is returned verbatim as the 202 response body
	// of the next POST /flow-runs. The default is a stub echoing the request's
	// flow slug.
	nextStartRun json.RawMessage
	// cancelErrors queues HTTP status codes for subsequent cancel calls.
	cancelErrors []int
}

// FlowRunListCall captures the query parameters the fake saw for one list call.
type FlowRunListCall struct {
	OwnerIds          []string
	FlowSlugs         []string
	FlowIds           []string
	Statuses          []string
	WorkflowIds       []string
	WorkflowVersionId []string
	Limit             string
	Cursor            string
}

// FlowRunStepListCall captures the query parameters the fake saw for one steps
// list call.
type FlowRunStepListCall struct {
	FlowRunID string
	Limit     string
	Cursor    string
}

// RespondWithBodyForStart queues the raw JSON body for the next POST
// /flow-runs. The body is returned exactly as given with status 202.
func (fr *FlowRunResource) RespondWithBodyForStart(body json.RawMessage) {
	fr.nextStartRun = body
}

// RespondWithErrorForCancel queues an HTTP status code for the next
// DELETE /flow-runs/{id}.
func (fr *FlowRunResource) RespondWithErrorForCancel(status int) {
	fr.cancelErrors = append(fr.cancelErrors, status)
}

func (fr *FlowRunResource) nextCancelError() (int, bool) {
	if len(fr.cancelErrors) == 0 {
		return 0, false
	}
	status := fr.cancelErrors[0]
	fr.cancelErrors = fr.cancelErrors[1:]
	return status, true
}

// AddStep records a step the fake can return on list/get.
func (fr *FlowRunResource) AddStep(step *FlowRunStep) *FlowRunStep {
	fr.Steps = append(fr.Steps, step)
	return step
}

// registerFlowRunRoutes wires the flow-related endpoints on the shared mux.
func registerFlowRunRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	registerFlowRoutes(mux, s, record)

	mux.HandleFunc("GET /flow-runs", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		call := FlowRunListCall{
			OwnerIds:          queryListValues(r, "ownerId"),
			FlowSlugs:         queryListValues(r, "flowSlug"),
			FlowIds:           queryListValues(r, "flowId"),
			Statuses:          queryListValues(r, "status"),
			WorkflowIds:       queryListValues(r, "workflowId"),
			WorkflowVersionId: queryListValues(r, "workflowVersionId"),
			Limit:             r.URL.Query().Get("limit"),
			Cursor:            r.URL.Query().Get("cursor"),
		}
		s.FlowRuns.ListCalls = append(s.FlowRuns.ListCalls, call)

		result := make([]map[string]json.RawMessage, 0, len(s.FlowRuns.Instances))
		for i, run := range s.FlowRuns.Instances {
			result = append(result, map[string]json.RawMessage{
				"flowRun": run.Body,
				"cursor":  json.RawMessage(strconv.Quote("c" + strconv.Itoa(i))),
			})
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("POST /flow-runs", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		body, _ := requestBodyBytes(r)
		s.FlowRuns.StartedRuns = append(s.FlowRuns.StartedRuns, body)

		response := s.FlowRuns.nextStartRun
		if response == nil {
			var parsed struct {
				Flow string `json:"flow"`
			}
			_ = json.Unmarshal(body, &parsed)
			stub := map[string]any{
				"id":     "frn-stub",
				"flowId": "flw-" + parsed.Flow,
				"status": "pending",
			}
			encoded, _ := json.Marshal(stub)
			response = encoded
		} else {
			s.FlowRuns.nextStartRun = nil
		}
		s.FlowRuns.StartResults = append(s.FlowRuns.StartResults, &FlowRun{Body: response})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write(response)
	})

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
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "flow run not found"})
	})

	mux.HandleFunc("DELETE /flow-runs/{flowRunId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, ok := s.FlowRuns.nextCancelError(); ok {
			message := http.StatusText(status)
			writeJSON(w, status, map[string]string{"message": message})
			return
		}
		id := r.PathValue("flowRunId")
		idx := slices.IndexFunc(s.FlowRuns.Instances, func(fr *FlowRun) bool { return fr.Id == id })
		if idx == -1 {
			writeJSON(w, http.StatusNotFound, map[string]string{"message": "flow run not found"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /flow-runs/{flowRunId}/steps", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("flowRunId")
		s.FlowRuns.StepListCalls = append(s.FlowRuns.StepListCalls, FlowRunStepListCall{
			FlowRunID: id,
			Limit:     r.URL.Query().Get("limit"),
			Cursor:    r.URL.Query().Get("cursor"),
		})
		matched := make([]*FlowRunStep, 0)
		for _, step := range s.FlowRuns.Steps {
			if step.FlowRunID == id {
				matched = append(matched, step)
			}
		}
		result := make([]map[string]json.RawMessage, 0, len(matched))
		for i, step := range matched {
			result = append(result, map[string]json.RawMessage{
				"step":   step.Body,
				"cursor": json.RawMessage(strconv.Quote("c" + strconv.Itoa(i))),
			})
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /flow-runs/{flowRunId}/steps/{stepSeq}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("flowRunId")
		seq, err := strconv.Atoi(r.PathValue("stepSeq"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "invalid step sequence"})
			return
		}
		for _, step := range s.FlowRuns.Steps {
			if step.FlowRunID == id && step.Seq == seq {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(step.Body)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "step not found"})
	})
}

func registerFlowRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("GET /flows", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		call := FlowListCall{
			OwnerIds:          queryListValues(r, "ownerId"),
			FlowSlugs:         queryListValues(r, "flowSlug"),
			WorkflowIds:       queryListValues(r, "workflowId"),
			WorkflowVersionId: queryListValues(r, "workflowVersionId"),
			Limit:             r.URL.Query().Get("limit"),
			Cursor:            r.URL.Query().Get("cursor"),
		}
		s.Flows.ListCalls = append(s.Flows.ListCalls, call)

		result := make([]map[string]json.RawMessage, 0, len(s.Flows.Instances))
		for i, flow := range s.Flows.Instances {
			result = append(result, map[string]json.RawMessage{
				"flow":   flow.Body,
				"cursor": json.RawMessage(strconv.Quote("c" + strconv.Itoa(i))),
			})
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /flows/{flowId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("flowId")
		for _, flow := range s.Flows.Instances {
			if flow.Id == id {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(flow.Body)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"message": "flow not found"})
	})
}

// requestBodyBytes returns the request body as a byte slice, draining the
// reader. Returns an empty slice when the body is empty. The caller doesn't
// need to restore the body because the record wrapper already does so.
func requestBodyBytes(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return []byte{}, nil
	}
	return io.ReadAll(r.Body)
}
