package renderapi

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type SandboxCredentialState struct {
	GroupID    string
	Credential sandboxesclient.SandboxCredential
	Value      string
	History    []SandboxCredentialRevision
}

type SandboxCredentialRevision struct {
	Credential sandboxesclient.SandboxCredential
	Value      string
}

type SandboxCredentialResource struct {
	Resource[*SandboxCredentialState]
	errorQueues map[string][]int
}

func (r *SandboxCredentialResource) respondWithError(action string, status int) {
	if r.errorQueues == nil {
		r.errorQueues = make(map[string][]int)
	}
	r.errorQueues[action] = append(r.errorQueues[action], status)
}

func (r *SandboxCredentialResource) RespondWithErrorForList(status int) {
	r.respondWithError("list", status)
}

func (r *SandboxCredentialResource) RespondWithErrorForCreate(status int) {
	r.respondWithError("create", status)
}

func (r *SandboxCredentialResource) RespondWithErrorForGet(status int) {
	r.respondWithError("get", status)
}

func (r *SandboxCredentialResource) RespondWithErrorForUpdate(status int) {
	r.respondWithError("update", status)
}

func (r *SandboxCredentialResource) RespondWithErrorForDelete(status int) {
	r.respondWithError("delete", status)
}

func (r *SandboxCredentialResource) RespondWithErrorForVersions(status int) {
	r.respondWithError("versions", status)
}

func (r *SandboxCredentialResource) RespondWithErrorForRestore(status int) {
	r.respondWithError("restore", status)
}

func (r *SandboxCredentialResource) find(group, id string) (*SandboxCredentialState, bool) {
	for _, state := range r.Instances {
		if state.GroupID == group && state.Credential.Id == id {
			return state, true
		}
	}
	return nil, false
}

func (state *SandboxCredentialState) revise(restoredFrom *int64) {
	id := int64(len(state.History) + 1)
	config := state.Credential
	config.Version = nil
	data, _ := json.Marshal(struct {
		Group  string
		Config sandboxesclient.SandboxCredential
		Value  string
	}{state.GroupID, config, state.Value})
	state.Credential.Version = &sandboxesclient.SandboxCredentialVersion{
		Id: id, UpdatedAt: time.Now().UTC(), UpdatedBy: "usr-fake",
		Fingerprint: fmt.Sprintf("%x", sha256.Sum256(data)), RestoredFromVersionId: restoredFrom,
	}
	state.History = append(state.History, SandboxCredentialRevision{state.Credential, state.Value})
}

func registerSandboxCredentialRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	base := "/sandbox-groups/{sandboxGroupId}/credentials"
	wrap := func(action string, handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			record(r)
			resource := s.SandboxCredentials
			if queue := resource.errorQueues[action]; len(queue) > 0 {
				status := queue[0]
				resource.errorQueues[action] = queue[1:]
				writeAPIError(w, status, "credential operation failed", "credential_error")
				return
			}
			if s.sandboxGroupOwner(r.PathValue("sandboxGroupId")) == "" {
				writeAPIError(w, http.StatusNotFound, "sandbox group not found", "")
				return
			}
			handler(w, r)
		}
	}
	find := func(w http.ResponseWriter, r *http.Request) (*SandboxCredentialState, bool) {
		state, ok := s.SandboxCredentials.find(r.PathValue("sandboxGroupId"), r.PathValue("credentialId"))
		if !ok {
			writeAPIError(w, http.StatusNotFound, "credential not found", "")
		}
		return state, ok
	}
	mux.HandleFunc("GET "+base, wrap("list", func(w http.ResponseWriter, r *http.Request) {
		out := []sandboxesclient.SandboxCredential{}
		for _, state := range s.SandboxCredentials.Instances {
			if state.GroupID == r.PathValue("sandboxGroupId") {
				out = append(out, state.Credential)
			}
		}
		slices.SortFunc(out, func(a, b sandboxesclient.SandboxCredential) int { return strings.Compare(a.Id, b.Id) })
		writeJSON(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST "+base, wrap("create", func(w http.ResponseWriter, r *http.Request) {
		var body sandboxesclient.SandboxCredentialCreate
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}
		group := r.PathValue("sandboxGroupId")
		if _, ok := s.SandboxCredentials.find(group, body.Id); ok {
			writeAPIError(w, http.StatusConflict, "credential already exists", "")
			return
		}
		state := &SandboxCredentialState{GroupID: group, Value: body.Value, Credential: sandboxesclient.SandboxCredential{
			Id: body.Id, Domain: strings.ToLower(body.Domain), Enabled: true, Injection: body.Injection,
			Methods: []string{}, Paths: []string{},
		}}
		if body.Enabled != nil {
			state.Credential.Enabled = *body.Enabled
		}
		if body.Methods != nil {
			state.Credential.Methods = *body.Methods
		}
		if body.Paths != nil {
			state.Credential.Paths = *body.Paths
		}
		state.revise(nil)
		s.SandboxCredentials.Add(state)
		writeJSON(w, http.StatusCreated, state.Credential)
	}))
	mux.HandleFunc("GET "+base+"/{credentialId}", wrap("get", func(w http.ResponseWriter, r *http.Request) {
		if state, ok := find(w, r); ok {
			writeJSON(w, http.StatusOK, state.Credential)
		}
	}))
	mux.HandleFunc("PATCH "+base+"/{credentialId}", wrap("update", func(w http.ResponseWriter, r *http.Request) {
		state, ok := find(w, r)
		if !ok {
			return
		}
		var body sandboxesclient.SandboxCredentialUpdate
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}
		if body.Domain != nil {
			state.Credential.Domain = strings.ToLower(*body.Domain)
		}
		if body.Value != nil {
			state.Value = *body.Value
		}
		if body.Enabled != nil {
			state.Credential.Enabled = *body.Enabled
		}
		if body.Methods != nil {
			state.Credential.Methods = *body.Methods
		}
		if body.Paths != nil {
			state.Credential.Paths = *body.Paths
		}
		if body.Injection != nil {
			state.Credential.Injection = *body.Injection
		}
		state.revise(nil)
		writeJSON(w, http.StatusOK, state.Credential)
	}))
	mux.HandleFunc("DELETE "+base+"/{credentialId}", wrap("delete", func(w http.ResponseWriter, r *http.Request) {
		group, id := r.PathValue("sandboxGroupId"), r.PathValue("credentialId")
		s.SandboxCredentials.Instances = slices.DeleteFunc(s.SandboxCredentials.Instances, func(state *SandboxCredentialState) bool {
			return state.GroupID == group && state.Credential.Id == id
		})
		w.WriteHeader(http.StatusAccepted)
	}))
	mux.HandleFunc("GET "+base+"/{credentialId}/versions", wrap("versions", func(w http.ResponseWriter, r *http.Request) {
		state, ok := find(w, r)
		if !ok {
			return
		}
		out := make([]sandboxesclient.SandboxCredentialVersion, 0, len(state.History))
		for i := len(state.History) - 1; i >= 0; i-- {
			out = append(out, *state.History[i].Credential.Version)
		}
		writeJSON(w, http.StatusOK, out)
	}))
	mux.HandleFunc("POST "+base+"/{credentialId}/restore", wrap("restore", func(w http.ResponseWriter, r *http.Request) {
		state, ok := find(w, r)
		if !ok {
			return
		}
		var body sandboxesclient.SandboxCredentialRestore
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeAPIError(w, http.StatusBadRequest, "invalid body", "")
			return
		}
		for _, revision := range state.History {
			if revision.Credential.Version.Id == body.VersionId {
				state.Credential, state.Value = revision.Credential, revision.Value
				state.revise(&body.VersionId)
				writeJSON(w, http.StatusOK, state.Credential)
				return
			}
		}
		writeAPIError(w, http.StatusNotFound, "credential version not found", "")
	}))
}
