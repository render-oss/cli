package renderapi

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// EnvironmentResource holds environment fixtures and queued errors for the fake API.
type EnvironmentResource struct {
	Resource[*client.Environment]
	getErrors       []int
	listErrors      []int
	resourcesErrors []int
}

// RespondWithErrorForGet queues an HTTP error for the next environment retrieval.
func (r *EnvironmentResource) RespondWithErrorForGet(status int) {
	r.getErrors = append(r.getErrors, status)
}

// RespondWithErrorForList queues an HTTP error for the next environment listing.
func (r *EnvironmentResource) RespondWithErrorForList(status int) {
	r.listErrors = append(r.listErrors, status)
}

// RespondWithErrorForResources queues an HTTP error for the next environment resources change.
func (r *EnvironmentResource) RespondWithErrorForResources(status int) {
	r.resourcesErrors = append(r.resourcesErrors, status)
}

func (r *EnvironmentResource) nextResourcesError() (int, bool) {
	if len(r.resourcesErrors) == 0 {
		return 0, false
	}

	status := r.resourcesErrors[0]
	r.resourcesErrors = r.resourcesErrors[1:]

	return status, true
}

func (s *Server) environmentByID(id string) (*client.Environment, bool) {
	idx := slices.IndexFunc(s.Environments.Instances, func(e *client.Environment) bool {
		return e.Id == id
	})
	if idx == -1 {
		return nil, false
	}

	return s.Environments.Instances[idx], true
}

// Unlike the API, the fake moves only sandbox groups.
func registerEnvironmentResourceRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	mux.HandleFunc("POST /environments/{environmentId}/resources", func(w http.ResponseWriter, r *http.Request) {
		record(r)

		var body client.EnvironmentResourcesPOSTInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}

		groups, ok := s.environmentResourceGroups(w, body.ResourceIds)
		if !ok {
			return
		}

		environment, ok := s.environmentByID(r.PathValue("environmentId"))
		if !ok {
			writeAPIError(w, http.StatusNotFound, "environment not found", "")
			return
		}

		for _, g := range groups {
			g.EnvironmentId = new(environment.Id)
		}

		writeJSON(w, http.StatusCreated, environment)
	})

	mux.HandleFunc("DELETE /environments/{environmentId}/resources", func(w http.ResponseWriter, r *http.Request) {
		record(r)

		groups, ok := s.environmentResourceGroups(w, queryListValues(r, "resourceIds"))
		if !ok {
			return
		}

		for _, g := range groups {
			g.EnvironmentId = nil
		}

		w.WriteHeader(http.StatusNoContent)
	})
}

func (s *Server) environmentResourceGroups(
	w http.ResponseWriter,
	resourceIDs []string,
) ([]*sandboxesclient.SandboxGroup, bool) {
	if status, hasError := s.Environments.nextResourcesError(); hasError {
		w.WriteHeader(status)
		return nil, false
	}

	groups := make([]*sandboxesclient.SandboxGroup, 0, len(resourceIDs))
	for _, id := range resourceIDs {
		group, ok := s.sandboxGroupByID(id)
		if !ok {
			writeAPIError(w, http.StatusNotFound, "sandbox group not found", "")
			return nil, false
		}

		groups = append(groups, group)
	}

	return groups, true
}
