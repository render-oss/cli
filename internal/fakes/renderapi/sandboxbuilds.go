package renderapi

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/render-oss/cli/internal/testids"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type SandboxBuildResource struct {
	Resource[*sandboxesclient.SandboxBuild]
	errorQueue []int
}

func (r *SandboxBuildResource) RespondWith(status int) {
	r.errorQueue = append(r.errorQueue, status)
}

func (r *SandboxBuildResource) nextError() (int, bool) {
	if len(r.errorQueue) == 0 {
		return 0, false
	}
	status := r.errorQueue[0]
	r.errorQueue = r.errorQueue[1:]
	return status, true
}

func NewSandboxBuild(b sandboxesclient.SandboxBuild) *sandboxesclient.SandboxBuild {
	if b.Id == "" {
		b.Id = testids.RandomSandboxBuildID()
	}
	if b.SandboxGroupId == "" {
		b.SandboxGroupId = testids.RandomSandboxGroupID()
	}
	if b.Name == "" {
		b.Name = "python"
	}
	if b.Source.Repository == "" {
		b.Source.Repository = "docker.io/library"
	}
	if b.Source.Image == "" {
		b.Source.Image = "python"
	}
	if b.Source.Reference == "" {
		b.Source.Reference = "3.13"
	}
	if b.SourceDigest == "" {
		b.SourceDigest = "sha256:c19d4a1e5b7f0a2d9c3e8b6f4a1d7c0e2b9f5a3d8c6e1b4f7a0d3c9e2b5f8a1d"
	}
	if b.Status == "" {
		b.Status = sandboxesclient.SandboxBuildStatusSucceeded
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	finished := b.Status == sandboxesclient.SandboxBuildStatusSucceeded || b.Status == sandboxesclient.SandboxBuildStatusFailed
	if finished && b.FinishedAt == nil {
		finishedAt := b.CreatedAt.Add(10 * time.Second)
		b.FinishedAt = &finishedAt
	}
	return &b
}

func (s *Server) buildByID(id string) (*sandboxesclient.SandboxBuild, bool) {
	for _, b := range s.SandboxBuilds.Instances {
		if b.Id == id {
			return b, true
		}
	}
	return nil, false
}

func (s *Server) buildIndex(groupID, buildID string) int {
	return slices.IndexFunc(s.SandboxBuilds.Instances, func(b *sandboxesclient.SandboxBuild) bool {
		return b.Id == buildID && b.SandboxGroupId == groupID
	})
}

// latestBuildByName returns the group's newest succeeded build with the name,
// or its newest build of any status when none has succeeded.
func (s *Server) latestBuildByName(groupID, name string) (*sandboxesclient.SandboxBuild, bool) {
	var latest, latestSucceeded *sandboxesclient.SandboxBuild
	for _, b := range s.SandboxBuilds.Instances {
		if b.SandboxGroupId != groupID || b.Name != name {
			continue
		}
		if latest == nil || b.CreatedAt.After(latest.CreatedAt) {
			latest = b
		}
		if b.Status == sandboxesclient.SandboxBuildStatusSucceeded && (latestSucceeded == nil || b.CreatedAt.After(latestSucceeded.CreatedAt)) {
			latestSucceeded = b
		}
	}
	if latestSucceeded != nil {
		return latestSucceeded, true
	}
	return latest, latest != nil
}

func registerSandboxBuildRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /sandbox-groups/{groupId}/builds", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.SandboxBuilds.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		groupID := r.PathValue("groupId")
		if s.sandboxGroupOwner(groupID) == "" {
			writeAPIError(w, http.StatusNotFound, "not found: sandbox group", "")
			return
		}
		var body sandboxesclient.SandboxBuildPOST
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}
		// Repository paths are lowercase; tags and digests need not be.
		repository := body.SourceImage
		if i := strings.LastIndexAny(repository, ":@"); i != -1 && !strings.Contains(repository[i:], "/") {
			repository = repository[:i]
		}
		if repository == "" || repository != strings.ToLower(repository) {
			writeAPIError(w, http.StatusBadRequest, "invalid sourceImage: could not parse reference: "+body.SourceImage, "")
			return
		}
		// The API returns the group's build of the same name and digest that
		// hasn't failed instead of starting another; the fake keys on the name.
		for _, existing := range s.SandboxBuilds.Instances {
			if existing.SandboxGroupId == groupID && existing.Name == body.Name && existing.Status != sandboxesclient.SandboxBuildStatusFailed {
				writeJSON(w, http.StatusCreated, existing)
				return
			}
		}
		build := s.SandboxBuilds.Add(NewSandboxBuild(sandboxesclient.SandboxBuild{
			SandboxGroupId: groupID, Name: body.Name, Status: sandboxesclient.SandboxBuildStatusRunning,
		}))
		writeJSON(w, http.StatusCreated, build)
	})

	mux.HandleFunc("GET /sandbox-groups/{groupId}/builds/{buildId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.SandboxBuilds.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		idx := s.buildIndex(r.PathValue("groupId"), r.PathValue("buildId"))
		if idx == -1 {
			writeAPIError(w, http.StatusNotFound, "not found: build", "")
			return
		}
		writeJSON(w, http.StatusOK, s.SandboxBuilds.Instances[idx])
	})
}
