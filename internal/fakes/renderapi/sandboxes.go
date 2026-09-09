package renderapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/render-oss/cli/internal/testids"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

const (
	ErrorCodeSnapshotNotFound     = "snapshot_not_found"
	ErrorCodeSnapshotNotAvailable = "snapshot_not_available"
	ErrorCodeSnapshotPlanMismatch = "snapshot_plan_mismatch"
)

type SandboxResource struct {
	Resource[*sandboxesclient.Sandbox]
}

func NewSandbox(sb sandboxesclient.Sandbox) *sandboxesclient.Sandbox {
	if sb.Id == "" {
		sb.Id = testids.RandomSandboxID()
	}
	if sb.Plan == "" {
		sb.Plan = sandboxesclient.Starter
	}
	if sb.Region == "" {
		sb.Region = "oregon"
	}
	if sb.Status == "" {
		sb.Status = sandboxesclient.SandboxStatusRunning
	}
	if sb.NetworkPolicy.Default == "" {
		sb.NetworkPolicy.Default = sandboxesclient.AllowAll
	}
	if sb.TimeoutSeconds == 0 {
		sb.TimeoutSeconds = 3600
	}
	if sb.CreatedAt.IsZero() {
		sb.CreatedAt = time.Now()
	}
	return &sb
}

func (s *Server) sandboxByID(id string) (*sandboxesclient.Sandbox, bool) {
	for _, sb := range s.Sandboxes.Instances {
		if sb.Id == id {
			return sb, true
		}
	}
	return nil, false
}

func (s *Server) sandboxGroupOwner(groupID string) string {
	for _, g := range s.SandboxGroups.Instances {
		if g.Id == groupID {
			return g.OwnerId
		}
	}
	return ""
}

func (s *Server) snapshotByID(id string) (*sandboxesclient.SandboxSnapshot, bool) {
	for _, snap := range s.SandboxSnapshots.Instances {
		if snap.Id == id {
			return snap, true
		}
	}
	return nil, false
}

func registerSandboxRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /sandboxes", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		var body sandboxesclient.SandboxPOST
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}
		if body.SnapshotId != nil {
			snap, found := s.snapshotByID(*body.SnapshotId)
			if !found || s.sandboxGroupOwner(snap.SandboxGroupId) != body.OwnerId {
				writeAPIError(w, http.StatusNotFound, "snapshot not found", ErrorCodeSnapshotNotFound)
				return
			}
			if snap.SandboxGroupId != s.defaultSandboxGroupID(body.OwnerId) {
				writeAPIError(w, http.StatusConflict, "snapshot belongs to a different sandbox group", ErrorCodeSnapshotNotAvailable)
				return
			}
			if snap.Status != sandboxesclient.SandboxSnapshotStatusAvailable {
				writeAPIError(w, http.StatusConflict, "snapshot is not available", ErrorCodeSnapshotNotAvailable)
				return
			}
			if snap.Kind == sandboxesclient.Runtime && body.Plan != nil && *body.Plan != snap.Plan {
				writeAPIError(w, http.StatusConflict, "plan does not match the runtime snapshot", ErrorCodeSnapshotPlanMismatch)
				return
			}
		}
		var sb sandboxesclient.Sandbox
		if body.Plan != nil {
			sb.Plan = *body.Plan
		}
		if body.Region != nil {
			sb.Region = *body.Region
		}
		if body.TimeoutSeconds != nil {
			sb.TimeoutSeconds = *body.TimeoutSeconds
		}
		if body.NetworkPolicy != nil {
			sb.NetworkPolicy = *body.NetworkPolicy
		}
		writeJSON(w, http.StatusCreated, s.Sandboxes.Add(NewSandbox(sb)))
	})
}
