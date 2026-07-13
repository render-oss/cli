package renderapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// SandboxGroupResource holds sandbox-group state and error injection for the
// fake server. Tests can assert against Instances.
type SandboxGroupResource struct {
	Resource[*sandboxesclient.SandboxGroup]
	errorQueue []int
}

// RespondWith queues an HTTP status code to return on the next sandbox-group
// operation handled by the fake server. The queue is drained in FIFO order.
func (r *SandboxGroupResource) RespondWith(status int) {
	r.errorQueue = append(r.errorQueue, status)
}

func (r *SandboxGroupResource) nextError() (int, bool) {
	if len(r.errorQueue) == 0 {
		return 0, false
	}
	status := r.errorQueue[0]
	r.errorQueue = r.errorQueue[1:]
	return status, true
}

// NewSandboxGroup returns a SandboxGroup with sensible defaults for any
// zero-value fields, mimicking the API's Alpha guarantees (one default group
// per workspace, populated region, non-nil timestamps). IsDefault is always
// set to true; for a non-default group (Beta), set the field after Add.
func NewSandboxGroup(g sandboxesclient.SandboxGroup) *sandboxesclient.SandboxGroup {
	if g.Id == "" {
		g.Id = testids.RandomSandboxGroupID()
	}
	if g.Name == "" {
		g.Name = "Default"
	}
	if g.Region == "" {
		g.Region = "oregon"
	}
	// A bool field can't distinguish "unset" from "explicitly false", so the
	// factory always produces the workspace's default group (the only kind
	// that exists in Alpha).
	g.IsDefault = true
	now := time.Now()
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	if g.UpdatedAt.IsZero() {
		g.UpdatedAt = now
	}
	return &g
}

// registerSandboxGroupRoutes wires GET /sandbox-groups on the shared mux. GET
// returns groups filtered by ownerId (matches the API's server-side contract:
// ownerId is required and accepts at most one value).
func registerSandboxGroupRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("GET /sandbox-groups", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.SandboxGroups.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		ownerIDs := queryListValues(r, "ownerId")
		if len(ownerIDs) == 0 {
			message := "ownerId is required"
			writeJSON(w, http.StatusBadRequest, client.Error{Message: &message})
			return
		}
		if len(ownerIDs) > 1 {
			message := "ownerId accepts at most one value"
			writeJSON(w, http.StatusBadRequest, client.Error{Message: &message})
			return
		}
		ownerID := ownerIDs[0]
		if _, ok := s.ownerByID(ownerID); !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		result := make([]client.SandboxGroupWithCursor, 0, len(s.SandboxGroups.Instances))
		for i, g := range s.SandboxGroups.Instances {
			if g.OwnerId != ownerID {
				continue
			}
			// Copy so the response doesn't mutate seeded state.
			group := *g
			result = append(result, client.SandboxGroupWithCursor{
				Cursor:       client.Cursor(fmt.Sprintf("c%d", i)),
				SandboxGroup: group,
			})
		}
		writeJSON(w, http.StatusOK, result)
	})
}
