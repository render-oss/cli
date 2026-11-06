package renderapi

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
)

// NewWorkflow returns a Workflow with sensible defaults for any zero-value
// fields: a generated ID, a name, a region, and non-zero timestamps.
func NewWorkflow(w wfclient.Workflow) *wfclient.Workflow {
	if w.Id == "" {
		w.Id = testids.RandomWorkflowID()
	}
	if w.Name == "" {
		w.Name = "my-workflow"
	}
	if w.Region == "" {
		w.Region = "oregon"
	}
	now := time.Now()
	if w.CreatedAt.IsZero() {
		w.CreatedAt = now
	}
	if w.UpdatedAt.IsZero() {
		w.UpdatedAt = now
	}
	return &w
}

// registerWorkflowRoutes wires GET /workflows and GET /workflows/{workflowId}
// on the shared mux. The list supports the ownerId and workflowID filters and
// returns every match on a single page.
func registerWorkflowRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("GET /workflows", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		ownerIDs := queryListValues(r, "ownerId")
		workflowIDs := queryListValues(r, "workflowID")
		result := make([]client.WorkflowWithCursor, 0, len(s.Workflows.Instances))
		for i, wf := range s.Workflows.Instances {
			if len(ownerIDs) > 0 && !slices.Contains(ownerIDs, wf.OwnerId) {
				continue
			}
			if len(workflowIDs) > 0 && !slices.Contains(workflowIDs, wf.Id) {
				continue
			}
			result = append(result, client.WorkflowWithCursor{
				Cursor:   client.Cursor(fmt.Sprintf("c%d", i)),
				Workflow: *wf,
			})
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /workflows/{workflowId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("workflowId")
		for _, wf := range s.Workflows.Instances {
			if wf.Id == id {
				writeJSON(w, http.StatusOK, wf)
				return
			}
		}
		http.NotFound(w, r)
	})

}
