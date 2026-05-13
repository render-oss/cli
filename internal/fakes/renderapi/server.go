package renderapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/render-oss/cli/pkg/client"
	"github.com/rs/xid"
)

// writeJSON encodes v as JSON and writes it with the given status code.
// Returns HTTP 500 if encoding fails (shouldn't happen with these types).
func writeJSON(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "fake server: encode error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

// RecordedRequest captures a single HTTP request made to the fake server.
type RecordedRequest struct {
	Method string
	URI    string
}

// Resource is a generic in-memory store for any fake API resource type.
type Resource[T any] struct {
	Instances []T
}

// Add appends item to the store and returns it.
func (r *Resource[T]) Add(item T) T {
	r.Instances = append(r.Instances, item)
	return item
}

// KVResource holds key-value store state and error injection for the fake server.
// Tests can assert against Instances.
type KVResource struct {
	Resource[*client.KeyValueDetail]
	// errorQueue holds HTTP status codes to return on successive create operations, drained in order.
	errorQueue []int
}

// RespondWith queues an HTTP status code to return on the next create operation.
// Use this to simulate API errors; the queue is drained in FIFO order.
func (kv *KVResource) RespondWith(status int) {
	kv.errorQueue = append(kv.errorQueue, status)
}

func (kv *KVResource) nextError() (int, bool) {
	if len(kv.errorQueue) == 0 {
		return 0, false
	}
	status := kv.errorQueue[0]
	kv.errorQueue = kv.errorQueue[1:]
	return status, true
}

// NewOwner returns an Owner with sensible defaults for any zero-value fields.
func NewOwner(o client.Owner) client.Owner {
	if o.Id == "" {
		o.Id = "tea-" + xid.New().String()
	}
	if o.Name == "" {
		o.Name = "My Team"
	}
	if o.Email == "" {
		o.Email = "team@example.com"
	}
	return o
}

// NewProject returns a Project with sensible defaults for any zero-value fields.
func NewProject(p client.Project) client.Project {
	if p.Id == "" {
		p.Id = "prj-" + xid.New().String()
	}
	if p.Name == "" {
		p.Name = "My Project"
	}
	return p
}

// NewEnvironment returns an Environment with sensible defaults for any zero-value fields.
func NewEnvironment(e client.Environment) client.Environment {
	if e.Id == "" {
		e.Id = "env-" + xid.New().String()
	}
	if e.Name == "" {
		e.Name = "My Environment"
	}
	return e
}

// NewKV returns a KeyValueDetail with sensible defaults for any zero-value fields.
func NewKV(kv *client.KeyValueDetail) *client.KeyValueDetail {
	if kv == nil {
		kv = &client.KeyValueDetail{}
	}
	if kv.Id == "" {
		kv.Id = fmt.Sprintf("kv-%s", xid.New().String())
	}
	if kv.Name == "" {
		kv.Name = "my-kv"
	}
	if kv.Region == "" {
		kv.Region = client.Oregon
	}
	if kv.Status == "" {
		kv.Status = client.DatabaseStatusAvailable
	}
	if kv.IpAllowList == nil {
		kv.IpAllowList = []client.CidrBlockAndDescription{}
	}
	if kv.CreatedAt.IsZero() || kv.UpdatedAt.IsZero() {
		now := time.Now()
		if kv.CreatedAt.IsZero() {
			kv.CreatedAt = now
		}
		if kv.UpdatedAt.IsZero() {
			kv.UpdatedAt = now
		}
	}
	return kv
}

// Server is a fake Render API HTTP server for command-level tests.
// All HTTP plumbing is internal — tests seed state via Add() methods and assert against resource Instances.
type Server struct {
	server       *httptest.Server
	Requests     []RecordedRequest
	Owners       *Resource[client.Owner]
	Projects     *Resource[client.Project]
	Environments *Resource[client.Environment]
	KV           *KVResource
}

// URL returns the base URL of the fake server.
func (s *Server) URL() string {
	return s.server.URL
}

// HasRequest returns true if any recorded request matches the given method and URI substring.
func (s *Server) HasRequest(method, uriSubstring string) bool {
	for _, r := range s.Requests {
		if r.Method == method && strings.Contains(r.URI, uriSubstring) {
			return true
		}
	}
	return false
}

// NewServer starts a fake Render API server covering all routes used by cmd-level tests.
// The server is closed automatically when t completes. Seed state via server.Owners.Add(), etc.
func NewServer(t *testing.T) *Server {
	t.Helper()

	s := &Server{
		Owners:       &Resource[client.Owner]{},
		Projects:     &Resource[client.Project]{},
		Environments: &Resource[client.Environment]{},
		KV:           &KVResource{},
	}

	mux := http.NewServeMux()

	record := func(r *http.Request) {
		s.Requests = append(s.Requests, RecordedRequest{Method: r.Method, URI: r.URL.RequestURI()})
	}

	// GET /owners — list workspaces (supports ?name= filter)
	mux.HandleFunc("/owners", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		result := s.Owners.Instances
		if name := r.URL.Query().Get("name"); name != "" {
			var filtered []client.Owner
			for _, o := range s.Owners.Instances {
				if o.Name == name {
					filtered = append(filtered, o)
				}
			}
			result = filtered
		}
		wrapped := make([]client.OwnerWithCursor, len(result))
		for i := range result {
			o := result[i]
			wrapped[i] = client.OwnerWithCursor{Owner: &o}
		}
		writeJSON(w, http.StatusOK, wrapped)
	})

	// GET /owners/{id} — retrieve workspace by ID
	mux.HandleFunc("/owners/", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/owners/")
		for _, o := range s.Owners.Instances {
			if o.Id == id {
				writeJSON(w, http.StatusOK, o)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /projects — list projects (supports ?name= filter)
	mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		result := s.Projects.Instances
		if name := r.URL.Query().Get("name"); name != "" {
			var filtered []client.Project
			for _, p := range s.Projects.Instances {
				if p.Name == name {
					filtered = append(filtered, p)
				}
			}
			result = filtered
		}
		wrapped := make([]client.ProjectWithCursor, len(result))
		for i := range result {
			wrapped[i] = client.ProjectWithCursor{Project: result[i]}
		}
		writeJSON(w, http.StatusOK, wrapped)
	})

	// GET /projects/{id} — retrieve project by ID
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/projects/")
		for _, p := range s.Projects.Instances {
			if p.Id == id {
				writeJSON(w, http.StatusOK, p)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /environments — list environments (supports ?projectId= and ?name= filters)
	mux.HandleFunc("/environments", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		result := s.Environments.Instances
		if projectIDs := r.URL.Query()["projectId"]; len(projectIDs) > 0 {
			var filtered []client.Environment
			for _, e := range result {
				for _, pid := range projectIDs {
					if e.ProjectId == pid {
						filtered = append(filtered, e)
					}
				}
			}
			result = filtered
		}
		if name := r.URL.Query().Get("name"); name != "" {
			var filtered []client.Environment
			for _, e := range result {
				if e.Name == name {
					filtered = append(filtered, e)
				}
			}
			result = filtered
		}
		wrapped := make([]client.EnvironmentWithCursor, len(result))
		for i := range result {
			wrapped[i] = client.EnvironmentWithCursor{Environment: result[i]}
		}
		writeJSON(w, http.StatusOK, wrapped)
	})

	// GET /environments/{id} — retrieve environment by ID
	mux.HandleFunc("/environments/", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/environments/")
		for _, e := range s.Environments.Instances {
			if e.Id == id {
				writeJSON(w, http.StatusOK, e)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// POST /key-value — create KV store
	mux.HandleFunc("/key-value", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body client.CreateKeyValueJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if status, hasError := s.KV.nextError(); hasError {
			w.WriteHeader(status)
			return
		}

		var owner client.Owner
		for _, o := range s.Owners.Instances {
			if o.Id == body.OwnerId {
				owner = o
				break
			}
		}

		region := client.Oregon
		if body.Region != nil {
			region = client.Region(*body.Region)
		}

		var maxmemoryPolicy *string
		if body.MaxmemoryPolicy != nil {
			mp := string(*body.MaxmemoryPolicy)
			maxmemoryPolicy = &mp
		}

		kv := &client.KeyValueDetail{
			Id:            fmt.Sprintf("kv-%s", xid.New().String()),
			Name:          body.Name,
			Plan:          body.Plan,
			Region:        region,
			Owner:         owner,
			Status:        client.DatabaseStatusAvailable,
			EnvironmentId: body.EnvironmentId,
			IpAllowList:   []client.CidrBlockAndDescription{},
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if maxmemoryPolicy != nil {
			kv.Options = client.KeyValueOptions{MaxmemoryPolicy: maxmemoryPolicy}
		}
		s.KV.Instances = append(s.KV.Instances, kv)

		writeJSON(w, http.StatusCreated, kv)
	})

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}
