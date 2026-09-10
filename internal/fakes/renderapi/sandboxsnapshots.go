package renderapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

const (
	ErrorCodeSandboxNotRunning = "sandbox_not_running"
	ErrorCodeSnapshotCreating  = "snapshot_creating"
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
	if s.ExpiresAt.IsZero() {
		s.ExpiresAt = s.RequestedAt.Add(7 * 24 * time.Hour)
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

func (s *Server) snapshotIndex(groupID, snapshotID string) int {
	return slices.IndexFunc(s.SandboxSnapshots.Instances, func(snap *sandboxesclient.SandboxSnapshot) bool {
		return snap.Id == snapshotID && snap.SandboxGroupId == groupID
	})
}

func (s *Server) snapshotList(r *http.Request, keep func(*sandboxesclient.SandboxSnapshot) bool) []client.SandboxSnapshotWithCursor {
	statuses := queryListValues(r, "status")
	var matched []*sandboxesclient.SandboxSnapshot
	for _, snap := range s.SandboxSnapshots.Instances {
		if !keep(snap) {
			continue
		}
		if len(statuses) > 0 && !slices.Contains(statuses, string(snap.Status)) {
			continue
		}
		matched = append(matched, snap)
	}
	slices.SortStableFunc(matched, func(a, b *sandboxesclient.SandboxSnapshot) int {
		return b.RequestedAt.Compare(a.RequestedAt)
	})
	result := make([]client.SandboxSnapshotWithCursor, 0, len(matched))
	for i, snap := range matched {
		result = append(result, client.SandboxSnapshotWithCursor{
			Cursor:   client.Cursor(fmt.Sprintf("c%d", i)),
			Snapshot: *snap,
		})
	}
	return pageOf(result, r)
}

func pageOf(all []client.SandboxSnapshotWithCursor, r *http.Request) []client.SandboxSnapshotWithCursor {
	start := 0
	if cursor := r.URL.Query().Get("cursor"); cursor != "" {
		start = slices.IndexFunc(all, func(item client.SandboxSnapshotWithCursor) bool { return string(item.Cursor) == cursor }) + 1
	}
	end := len(all)
	if limit, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && limit > 0 && start+limit < end {
		end = start + limit
	}
	return all[start:end]
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

	mux.HandleFunc("GET /sandbox-groups/{groupId}/snapshots", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.SandboxSnapshots.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		if _, ok := s.ownerFromQuery(w, r); !ok {
			return
		}
		groupID := r.PathValue("groupId")
		writeJSON(w, http.StatusOK, s.snapshotList(r, func(snap *sandboxesclient.SandboxSnapshot) bool {
			return snap.SandboxGroupId == groupID
		}))
	})

	mux.HandleFunc("GET /sandbox-groups/{groupId}/snapshots/{snapshotId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.SandboxSnapshots.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		if _, ok := s.ownerFromQuery(w, r); !ok {
			return
		}
		idx := s.snapshotIndex(r.PathValue("groupId"), r.PathValue("snapshotId"))
		if idx == -1 {
			writeAPIError(w, http.StatusNotFound, "snapshot not found", "")
			return
		}
		writeJSON(w, http.StatusOK, s.SandboxSnapshots.Instances[idx])
	})

	mux.HandleFunc("DELETE /sandbox-groups/{groupId}/snapshots/{snapshotId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.SandboxSnapshots.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		if _, ok := s.ownerFromQuery(w, r); !ok {
			return
		}
		idx := s.snapshotIndex(r.PathValue("groupId"), r.PathValue("snapshotId"))
		if idx == -1 {
			writeAPIError(w, http.StatusNotFound, "snapshot not found", "")
			return
		}
		if s.SandboxSnapshots.Instances[idx].Status == sandboxesclient.SandboxSnapshotStatusCreating {
			writeAPIError(w, http.StatusConflict, "snapshot is still being created", ErrorCodeSnapshotCreating)
			return
		}
		s.SandboxSnapshots.Instances = slices.Delete(s.SandboxSnapshots.Instances, idx, idx+1)
		w.WriteHeader(http.StatusNoContent)
	})
}
