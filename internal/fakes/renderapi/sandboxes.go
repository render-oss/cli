package renderapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

const (
	ErrorCodeSnapshotNotFound     = "snapshot_not_found"
	ErrorCodeSnapshotNotAvailable = "snapshot_not_available"
	ErrorCodeSnapshotPlanMismatch = "snapshot_plan_mismatch"
)

type SandboxResource struct {
	Resource[*sandboxesclient.Sandbox]
	getErrorQueue []int
}

// RespondToGetWith queues an HTTP error status for the next
// GET /sandboxes/{sandboxId}.
func (r *SandboxResource) RespondToGetWith(status int) {
	r.getErrorQueue = append(r.getErrorQueue, status)
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
	if sb.NetworkPolicy.Type == "" {
		if sb.NetworkPolicy.Default != nil && *sb.NetworkPolicy.Default != "" {
			sb.NetworkPolicy.Type = *sb.NetworkPolicy.Default
		} else {
			sb.NetworkPolicy.Type = sandboxesclient.AllowAll
		}
	}
	defaultPolicyType := sb.NetworkPolicy.Type
	sb.NetworkPolicy.Default = &defaultPolicyType
	if sb.TimeoutSeconds == 0 {
		sb.TimeoutSeconds = 3600
	}
	if sb.IdleTimeoutSeconds == 0 {
		sb.IdleTimeoutSeconds = 900
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

func (s *Server) snapshotByName(groupID, name string) (*sandboxesclient.SandboxSnapshot, bool) {
	var latest *sandboxesclient.SandboxSnapshot
	for _, snap := range s.SandboxSnapshots.Instances {
		if snap.SandboxGroupId != groupID || snap.Status != sandboxesclient.SandboxSnapshotStatusAvailable ||
			snap.Name == nil || *snap.Name != name {
			continue
		}
		if latest == nil || snap.RequestedAt.After(latest.RequestedAt) {
			latest = snap
		}
	}
	return latest, latest != nil
}

func registerSandboxRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("GET /sandboxes", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		q := r.URL.Query()
		limit, _ := strconv.Atoi(q.Get("limit"))
		if limit <= 0 {
			limit = 20
		}
		statuses := strings.Split(q.Get("status"), ",")
		result := make([]client.SandboxWithCursor, 0)
		pastCursor := q.Get("cursor") == ""
		for _, sb := range s.Sandboxes.Instances {
			if sb.SandboxGroupId == nil || s.sandboxGroupOwner(*sb.SandboxGroupId) != q.Get("ownerId") {
				continue
			}
			if groupID := q.Get("sandboxGroupId"); groupID != "" && *sb.SandboxGroupId != groupID {
				continue
			}
			if !pastCursor {
				pastCursor = sb.Id == q.Get("cursor")
				continue
			}
			if q.Get("status") == "" && sb.Status == sandboxesclient.SandboxStatusTerminated {
				continue
			}
			if q.Get("status") != "" && !slices.Contains(statuses, string(sb.Status)) {
				continue
			}
			result = append(result, client.SandboxWithCursor{Sandbox: *sb, Cursor: sb.Id})
			if len(result) == limit {
				break
			}
		}
		writeJSON(w, http.StatusOK, result)
	})

	mux.HandleFunc("GET /sandboxes/{sandboxId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if q := s.Sandboxes.getErrorQueue; len(q) > 0 {
			s.Sandboxes.getErrorQueue = q[1:]
			writeAPIError(w, q[0], http.StatusText(q[0]), "")
			return
		}
		sb, found := s.sandboxByID(r.PathValue("sandboxId"))
		if !found {
			writeAPIError(w, http.StatusNotFound, "sandbox not found", "")
			return
		}
		writeJSON(w, http.StatusOK, sb)
	})

	mux.HandleFunc("POST /sandboxes", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		var body sandboxesclient.SandboxPOST
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}
		groupID := s.defaultSandboxGroupID(body.OwnerId)
		if body.SandboxGroupId != nil {
			groupID = *body.SandboxGroupId
			if s.sandboxGroupOwner(groupID) != body.OwnerId {
				writeAPIError(w, http.StatusNotFound, "sandbox group not found", "")
				return
			}
		}
		if body.SnapshotId != nil && body.SnapshotName != nil {
			writeAPIError(w, http.StatusBadRequest, "snapshotId and snapshotName are mutually exclusive", "")
			return
		}
		if body.SnapshotName != nil && invalidSnapshotName(*body.SnapshotName) {
			writeAPIError(w, http.StatusBadRequest, "invalid snapshot name", ErrorCodeInvalidSnapshotName)
			return
		}
		if body.BuildName != nil && *body.BuildName == "" {
			writeAPIError(w, http.StatusBadRequest, "invalid buildName: name must be 1 to 255 characters", "")
			return
		}
		if body.BuildId != nil {
			build, found := s.buildByID(*body.BuildId)
			if !found || build.SandboxGroupId != groupID {
				writeAPIError(w, http.StatusNotFound, "not found: build", "")
				return
			}
			if build.Status != sandboxesclient.SandboxBuildStatusSucceeded {
				writeAPIError(w, http.StatusConflict, "build has not succeeded", "")
				return
			}
		}
		if body.BuildName != nil {
			build, found := s.latestBuildByName(groupID, *body.BuildName)
			if !found {
				writeAPIError(w, http.StatusNotFound, "not found: build", "")
				return
			}
			if build.Status != sandboxesclient.SandboxBuildStatusSucceeded {
				writeAPIError(w, http.StatusConflict, "build has not succeeded", "")
				return
			}
		}
		if body.SnapshotId != nil {
			snap, found := s.snapshotByID(*body.SnapshotId)
			if !found || s.sandboxGroupOwner(snap.SandboxGroupId) != body.OwnerId {
				writeAPIError(w, http.StatusNotFound, "snapshot not found", ErrorCodeSnapshotNotFound)
				return
			}
			if snap.SandboxGroupId != groupID {
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
		if body.SnapshotName != nil {
			snap, found := s.snapshotByName(groupID, *body.SnapshotName)
			if !found {
				writeAPIError(w, http.StatusNotFound, "snapshot not found", ErrorCodeSnapshotNotFound)
				return
			}
			if snap.Kind == sandboxesclient.Runtime && body.Plan != nil && *body.Plan != snap.Plan {
				writeAPIError(w, http.StatusConflict, "plan does not match the runtime snapshot", ErrorCodeSnapshotPlanMismatch)
				return
			}
		}
		sb := sandboxesclient.Sandbox{SandboxGroupId: &groupID}
		if body.Plan != nil {
			sb.Plan = *body.Plan
		}
		if body.TimeoutSeconds != nil {
			sb.TimeoutSeconds = *body.TimeoutSeconds
		}
		if body.NetworkPolicy != nil {
			policyType := body.NetworkPolicy.Type
			if policyType == nil {
				policyType = body.NetworkPolicy.Default
			}
			if policyType != nil {
				typeValue := *policyType
				sb.NetworkPolicy.Type = typeValue
				sb.NetworkPolicy.Default = &typeValue
			}
			sb.NetworkPolicy.Rules = body.NetworkPolicy.Rules
		}
		created := NewSandbox(sb)
		if body.IdleTimeoutSeconds != nil {
			created.IdleTimeoutSeconds = *body.IdleTimeoutSeconds
		}
		writeJSON(w, http.StatusCreated, s.Sandboxes.Add(created))
	})
}
