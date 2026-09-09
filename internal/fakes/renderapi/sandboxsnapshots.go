package renderapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

const (
	ErrorCodeSandboxNotRunning = "sandbox_not_running"
)

type SandboxSnapshotResource struct {
	Resource[*sandboxesclient.SandboxSnapshot]
	errorQueue []int
}

func (r *SandboxSnapshotResource) RespondWith(status int) {
	r.errorQueue = append(r.errorQueue, status)
}

func (r *SandboxSnapshotResource) nextError() (int, bool) {
	if len(r.errorQueue) == 0 {
		return 0, false
	}
	status := r.errorQueue[0]
	r.errorQueue = r.errorQueue[1:]
	return status, true
}

func NewSandboxSnapshot(s sandboxesclient.SandboxSnapshot) *sandboxesclient.SandboxSnapshot {
	if s.Id == "" {
		s.Id = testids.RandomSandboxSnapshotID()
	}
	if s.SandboxGroupId == "" {
		s.SandboxGroupId = testids.RandomSandboxGroupID()
	}
	if s.SourceSandboxId == "" {
		s.SourceSandboxId = testids.RandomSandboxID()
	}
	if s.Kind == "" {
		s.Kind = sandboxesclient.Filesystem
	}
	if s.Status == "" {
		s.Status = sandboxesclient.SandboxSnapshotStatusAvailable
	}
	if s.Plan == "" {
		s.Plan = sandboxesclient.Starter
	}
	if s.RequestedAt.IsZero() {
		s.RequestedAt = time.Now()
	}
	return &s
}

func writeAPIError(w http.ResponseWriter, status int, message, code string) {
	apiErr := client.Error{Message: &message}
	if code != "" {
		apiErr.Code = &code
	}
	writeJSON(w, status, apiErr)
}

func (s *Server) ownerFromQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	ownerIDs := queryListValues(r, "ownerId")
	if len(ownerIDs) == 0 {
		writeAPIError(w, http.StatusBadRequest, "ownerId is required", "")
		return "", false
	}
	if len(ownerIDs) > 1 {
		writeAPIError(w, http.StatusBadRequest, "ownerId accepts at most one value", "")
		return "", false
	}
	if _, ok := s.ownerByID(ownerIDs[0]); !ok {
		writeAPIError(w, http.StatusNotFound, "owner not found", "")
		return "", false
	}
	return ownerIDs[0], true
}

func (s *Server) defaultSandboxGroupID(ownerID string) string {
	for _, g := range s.SandboxGroups.Instances {
		if g.OwnerId == ownerID {
			return g.Id
		}
	}
	return ""
}

func registerSandboxSnapshotRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /sandboxes/{sandboxId}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.SandboxSnapshots.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		ownerID, ok := s.ownerFromQuery(w, r)
		if !ok {
			return
		}
		sandboxID := r.PathValue("sandboxId")
		sb, found := s.sandboxByID(sandboxID)
		if !found {
			writeAPIError(w, http.StatusNotFound, "sandbox not found", "")
			return
		}
		if sb.Status != sandboxesclient.SandboxStatusRunning {
			writeAPIError(w, http.StatusConflict, "sandbox is not running", ErrorCodeSandboxNotRunning)
			return
		}
		var body sandboxesclient.SandboxSnapshotPOST
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}
		kind := sandboxesclient.Filesystem
		if body.Kind != nil {
			kind = *body.Kind
		}
		snapshot := s.SandboxSnapshots.Add(NewSandboxSnapshot(sandboxesclient.SandboxSnapshot{
			SandboxGroupId:  s.defaultSandboxGroupID(ownerID),
			SourceSandboxId: sandboxID,
			Kind:            kind,
			Status:          sandboxesclient.SandboxSnapshotStatusCreating,
			Plan:            sb.Plan,
		}))
		writeJSON(w, http.StatusAccepted, snapshot)
	})
}
