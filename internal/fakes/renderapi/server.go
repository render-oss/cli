package renderapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	pgclient "github.com/render-oss/cli/pkg/client/postgres"
	"github.com/render-oss/cli/pkg/pointers"
)

// queryListValues returns all values for key, splitting each occurrence on
// commas. URL.Query already preserves repeated params such as ?k=a&k=b as
// multiple values, so this handles both ?k=a&k=b and ?k=a,b.
func queryListValues(r *http.Request, key string) []string {
	raw := r.URL.Query()[key]
	if len(raw) == 0 {
		return nil
	}
	var out []string
	for _, v := range raw {
		for part := range strings.SplitSeq(v, ",") {
			if part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

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

// PostgresResource holds Postgres state and error injection for the fake server.
// Tests can assert against Instances.
type PostgresResource struct {
	Resource[*client.PostgresDetail]
	errorQueue []int
}

// RespondWith queues an HTTP status code to return on the next Postgres
// operation handled by the fake server. The queue is drained in FIFO order.
func (pg *PostgresResource) RespondWith(status int) {
	pg.errorQueue = append(pg.errorQueue, status)
}

func (pg *PostgresResource) nextError() (int, bool) {
	if len(pg.errorQueue) == 0 {
		return 0, false
	}
	status := pg.errorQueue[0]
	pg.errorQueue = pg.errorQueue[1:]
	return status, true
}

// NewOwner returns an Owner with sensible defaults for any zero-value fields.
func NewOwner(o client.Owner) *client.Owner {
	if o.Id == "" {
		o.Id = testids.RandomWorkspaceID()
	}
	if o.Name == "" {
		o.Name = "My Team"
	}
	if o.Email == "" {
		o.Email = "team@example.com"
	}
	return &o
}

// ProjectAttrs defines the fields callers can specify when creating a fake project.
type ProjectAttrs struct {
	Id      string
	Name    string
	OwnerId string
}

// NewProject returns a Project with sensible defaults for any zero-value fields.
func NewProject(attrs ProjectAttrs) *client.Project {
	if attrs.Id == "" {
		attrs.Id = testids.RandomProjectID()
	}
	if attrs.Name == "" {
		attrs.Name = "My Project"
	}
	return &client.Project{
		Id:    attrs.Id,
		Name:  attrs.Name,
		Owner: client.Owner{Id: attrs.OwnerId},
	}
}

// NewEnvironment returns an Environment with sensible defaults for any zero-value fields.
func NewEnvironment(e client.Environment) *client.Environment {
	if e.Id == "" {
		e.Id = testids.RandomEnvironmentID()
	}
	if e.Name == "" {
		e.Name = "My Environment"
	}
	return &e
}

// NewKV returns a KeyValueDetail with sensible defaults for any zero-value fields.
func NewKV(kv client.KeyValueDetail) *client.KeyValueDetail {
	if kv.Id == "" {
		kv.Id = testids.RandomKeyValueID()
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
	return &kv
}

// NewPostgres returns a PostgresDetail with sensible defaults for any
// zero-value fields.
func NewPostgres(pg client.PostgresDetail) *client.PostgresDetail {
	if pg.Id == "" {
		pg.Id = testids.RandomPostgresID()
	}
	if pg.Name == "" {
		pg.Name = "my-postgres"
	}
	if pg.Plan == "" {
		pg.Plan = pgclient.Free
	}
	if pg.Version == "" {
		pg.Version = client.PostgresVersion("18")
	}
	if pg.Region == "" {
		pg.Region = client.Oregon
	}
	if pg.Status == "" {
		pg.Status = client.DatabaseStatusAvailable
	}
	if pg.DatabaseName == "" {
		pg.DatabaseName = pg.Name + "_db"
	}
	if pg.DatabaseUser == "" {
		pg.DatabaseUser = "appuser"
	}
	if pg.IpAllowList == nil {
		pg.IpAllowList = []client.CidrBlockAndDescription{}
	}
	if pg.ReadReplicas == nil {
		pg.ReadReplicas = client.ReadReplicas{}
	}
	if pg.DashboardUrl == "" {
		pg.DashboardUrl = "https://dashboard.render.com/d/" + pg.Id
	}
	now := time.Now()
	if pg.CreatedAt.IsZero() {
		pg.CreatedAt = now
	}
	if pg.UpdatedAt.IsZero() {
		pg.UpdatedAt = now
	}
	return &pg
}

func postgresListItem(pg *client.PostgresDetail) client.Postgres {
	return client.Postgres{
		CreatedAt:               pg.CreatedAt,
		DashboardUrl:            pg.DashboardUrl,
		DatabaseName:            pg.DatabaseName,
		DatabaseUser:            pg.DatabaseUser,
		DiskAutoscalingEnabled:  pg.DiskAutoscalingEnabled,
		DiskSizeGB:              pg.DiskSizeGB,
		EnvironmentId:           pg.EnvironmentId,
		ExpiresAt:               pg.ExpiresAt,
		HighAvailabilityEnabled: pg.HighAvailabilityEnabled,
		Id:                      pg.Id,
		IpAllowList:             pg.IpAllowList,
		Name:                    pg.Name,
		Owner:                   pg.Owner,
		Plan:                    pg.Plan,
		PrimaryPostgresID:       pg.PrimaryPostgresID,
		ReadReplicas:            pg.ReadReplicas,
		Region:                  pg.Region,
		Role:                    pg.Role,
		Status:                  pg.Status,
		Suspenders:              pg.Suspenders,
		UpdatedAt:               pg.UpdatedAt,
		Version:                 pg.Version,
	}
}

// NewUser returns a User with sensible defaults for any zero-value fields.
func NewUser(u client.User) client.User {
	if u.Email == "" {
		u.Email = "user@example.com"
	}
	if u.Name == "" {
		u.Name = "Test User"
	}
	return u
}

// Server is a fake Render API HTTP server for command-level tests.
// All HTTP plumbing is internal — tests seed state via Add() methods and assert against resource Instances.
type Server struct {
	server       *httptest.Server
	Requests     []RecordedRequest
	CurrentUser  *client.User
	Owners       *Resource[*client.Owner]
	Projects     *Resource[*client.Project]
	Environments *Resource[*client.Environment]
	KV           *KVResource
	Postgres     *PostgresResource
	Services *ServiceResource
}

// ownerByID returns the Owner with the given ID from the seeded owners. The
// boolean reports whether a match was found; the Owner is only meaningful when
// the boolean is true.
func (s *Server) ownerByID(id string) (client.Owner, bool) {
	for _, o := range s.Owners.Instances {
		if o.Id == id {
			return *o, true
		}
	}
	return client.Owner{}, false
}

// URL returns the base URL of the fake server.
func (s *Server) URL() string {
	return s.server.URL
}

// SetCurrentUser seeds the user returned by GET /users and returns it.
func (s *Server) SetCurrentUser(u client.User) client.User {
	s.CurrentUser = &u
	return u
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
		Owners:       &Resource[*client.Owner]{},
		Projects:     &Resource[*client.Project]{},
		Environments: &Resource[*client.Environment]{},
		KV:           &KVResource{},
		Postgres:     &PostgresResource{},
		Services:     &ServiceResource{},
	}

	mux := http.NewServeMux()

	record := func(r *http.Request) {
		s.Requests = append(s.Requests, RecordedRequest{Method: r.Method, URI: r.URL.RequestURI()})
	}

	// GET /users - retrieve the authenticated user.
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if s.CurrentUser == nil {
			message := "unauthorized"
			writeJSON(w, http.StatusUnauthorized, client.Error{Message: &message})
			return
		}
		writeJSON(w, http.StatusOK, s.CurrentUser)
	})

	// GET /owners — list workspaces (supports ?name= filter)
	mux.HandleFunc("GET /owners", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		result := s.Owners.Instances
		if name := r.URL.Query().Get("name"); name != "" {
			var filtered []*client.Owner
			for _, o := range s.Owners.Instances {
				if o.Name == name {
					filtered = append(filtered, o)
				}
			}
			result = filtered
		}
		wrapped := make([]client.OwnerWithCursor, len(result))
		for i := range result {
			wrapped[i] = client.OwnerWithCursor{Owner: result[i]}
		}
		writeJSON(w, http.StatusOK, wrapped)
	})

	// GET /owners/{id} — retrieve workspace by ID
	mux.HandleFunc("GET /owners/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		for _, o := range s.Owners.Instances {
			if o.Id == id {
				writeJSON(w, http.StatusOK, o)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /projects — list projects (supports ?ownerId= and ?name= filters)
	mux.HandleFunc("GET /projects", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		result := s.Projects.Instances
		if ownerIDs := queryListValues(r, "ownerId"); len(ownerIDs) > 0 {
			var filtered []*client.Project
			for _, p := range result {
				if slices.Contains(ownerIDs, p.Owner.Id) {
					filtered = append(filtered, p)
				}
			}
			result = filtered
		}
		if name := r.URL.Query().Get("name"); name != "" {
			var filtered []*client.Project
			for _, p := range result {
				if p.Name == name {
					filtered = append(filtered, p)
				}
			}
			result = filtered
		}
		wrapped := make([]client.ProjectWithCursor, len(result))
		for i := range result {
			// Copy before setting Owner so the response doesn't mutate stored state.
			p := *result[i]
			owner, ok := s.ownerByID(p.Owner.Id)
			if !ok {
				http.Error(w, "fake server: project owner not seeded: "+p.Owner.Id, http.StatusInternalServerError)
				return
			}
			p.Owner = owner
			wrapped[i] = client.ProjectWithCursor{Project: p}
		}
		writeJSON(w, http.StatusOK, wrapped)
	})

	// GET /projects/{id} — retrieve project by ID
	mux.HandleFunc("GET /projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		for _, stored := range s.Projects.Instances {
			if stored.Id == id {
				// Copy before setting Owner so the response doesn't mutate stored state.
				p := *stored
				owner, ok := s.ownerByID(p.Owner.Id)
				if !ok {
					http.Error(w, "fake server: project owner not seeded: "+p.Owner.Id, http.StatusInternalServerError)
					return
				}
				p.Owner = owner
				writeJSON(w, http.StatusOK, p)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /environments — list environments (supports ?projectId= and ?name= filters)
	mux.HandleFunc("GET /environments", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		result := s.Environments.Instances
		if projectIDs := queryListValues(r, "projectId"); len(projectIDs) > 0 {
			var filtered []*client.Environment
			for _, e := range result {
				if slices.Contains(projectIDs, e.ProjectId) {
					filtered = append(filtered, e)
				}
			}
			result = filtered
		}
		if name := r.URL.Query().Get("name"); name != "" {
			var filtered []*client.Environment
			for _, e := range result {
				if e.Name == name {
					filtered = append(filtered, e)
				}
			}
			result = filtered
		}
		wrapped := make([]client.EnvironmentWithCursor, len(result))
		for i := range result {
			wrapped[i] = client.EnvironmentWithCursor{Environment: *result[i]}
		}
		writeJSON(w, http.StatusOK, wrapped)
	})

	// GET /environments/{id} — retrieve environment by ID
	mux.HandleFunc("GET /environments/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		for _, e := range s.Environments.Instances {
			if e.Id == id {
				writeJSON(w, http.StatusOK, e)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /key-value — list KV instances (supports ?name= and ?environmentId= filters)
	mux.HandleFunc("GET /key-value", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		name := r.URL.Query().Get("name")
		envIDs := queryListValues(r, "environmentId")
		result := make([]client.KeyValueWithCursor, 0, len(s.KV.Instances))
		// Preserve insertion order so list tests can make deterministic
		// assertions about multiple resources.
		for i, kv := range s.KV.Instances {
			if name != "" && kv.Name != name {
				continue
			}
			if len(envIDs) > 0 {
				if kv.EnvironmentId == nil || !slices.Contains(envIDs, *kv.EnvironmentId) {
					continue
				}
			}
			result = append(result, client.KeyValueWithCursor{
				Cursor: client.Cursor(fmt.Sprintf("c%d", i)),
				KeyValue: client.KeyValue{
					CreatedAt:     kv.CreatedAt,
					EnvironmentId: kv.EnvironmentId,
					Id:            kv.Id,
					IpAllowList:   kv.IpAllowList,
					Name:          kv.Name,
					Options:       kv.Options,
					Owner:         kv.Owner,
					Plan:          kv.Plan,
					Region:        kv.Region,
					Status:        kv.Status,
					UpdatedAt:     kv.UpdatedAt,
					Version:       kv.Version,
				},
			})
		}
		writeJSON(w, http.StatusOK, result)
	})

	// POST /key-value — create a KV instance
	mux.HandleFunc("POST /key-value", func(w http.ResponseWriter, r *http.Request) {
		record(r)
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
				owner = *o
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

		ipAllowList := []client.CidrBlockAndDescription{}
		if body.IpAllowList != nil {
			ipAllowList = *body.IpAllowList
		}

		kv := &client.KeyValueDetail{
			Id:            testids.RandomKeyValueID(),
			Name:          body.Name,
			Plan:          body.Plan,
			Region:        region,
			Owner:         owner,
			Status:        client.DatabaseStatusAvailable,
			EnvironmentId: body.EnvironmentId,
			IpAllowList:   ipAllowList,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if maxmemoryPolicy != nil {
			kv.Options = client.KeyValueOptions{MaxmemoryPolicy: maxmemoryPolicy}
		}
		s.KV.Instances = append(s.KV.Instances, kv)

		writeJSON(w, http.StatusCreated, kv)
	})

	// GET /key-value/{id} — retrieve a KV instance
	mux.HandleFunc("GET /key-value/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		if status, hasError := s.KV.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		for _, kv := range s.KV.Instances {
			if kv.Id == id {
				writeJSON(w, http.StatusOK, kv)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// DELETE /key-value/{id} — delete a KV instance
	mux.HandleFunc("DELETE /key-value/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		for i, kv := range s.KV.Instances {
			if kv.Id == id {
				s.KV.Instances = slices.Delete(s.KV.Instances, i, i+1)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// PATCH /key-value/{id} — update a KV instance
	mux.HandleFunc("PATCH /key-value/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.KV.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		var body client.KeyValuePATCHInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		idx := slices.IndexFunc(s.KV.Instances, func(kv *client.KeyValueDetail) bool {
			return kv.Id == id
		})
		if idx == -1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		kv := s.KV.Instances[idx]
		if body.Name != nil {
			kv.Name = *body.Name
		}
		if body.Plan != nil {
			kv.Plan = *body.Plan
		}
		if body.MaxmemoryPolicy != nil {
			mp := string(*body.MaxmemoryPolicy)
			kv.Options = client.KeyValueOptions{MaxmemoryPolicy: &mp}
		}
		if body.IpAllowList != nil {
			kv.IpAllowList = *body.IpAllowList
		}
		kv.UpdatedAt = time.Now()
		writeJSON(w, http.StatusOK, kv)
	})

	// POST /key-value/{id}/suspend — suspend a KV instance
	mux.HandleFunc("POST /key-value/{id}/suspend", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.KV.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		for _, kv := range s.KV.Instances {
			if kv.Id == id {
				kv.Status = client.DatabaseStatusSuspended
				kv.UpdatedAt = time.Now()
				w.WriteHeader(http.StatusAccepted)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// POST /key-value/{id}/resume — resume a suspended KV instance
	mux.HandleFunc("POST /key-value/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.KV.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		for _, kv := range s.KV.Instances {
			if kv.Id == id {
				kv.Status = client.DatabaseStatusAvailable
				kv.UpdatedAt = time.Now()
				w.WriteHeader(http.StatusAccepted)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /key-value/{id}/connection-info — retrieve connection strings for a KV instance
	mux.HandleFunc("GET /key-value/{id}/connection-info", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		for _, kv := range s.KV.Instances {
			if kv.Id == id {
				writeJSON(w, http.StatusOK, &client.KeyValueConnectionInfo{
					CliCommand:               "redis-cli -h fake-host -p 6379",
					InternalConnectionString: "redis://fake-internal",
					ExternalConnectionString: "rediss://fake-external",
				})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /postgres — list Postgres instances (supports ?name=, ?ownerId=,
	// and ?environmentId= filters)
	mux.HandleFunc("GET /postgres", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		name := r.URL.Query().Get("name")
		ownerID := r.URL.Query().Get("ownerId")
		envIDs := queryListValues(r, "environmentId")
		result := make([]client.PostgresWithCursor, 0, len(s.Postgres.Instances))
		for i, pg := range s.Postgres.Instances {
			if name != "" && pg.Name != name {
				continue
			}
			if ownerID != "" && pg.Owner.Id != ownerID {
				continue
			}
			if len(envIDs) > 0 {
				if pg.EnvironmentId == nil || !slices.Contains(envIDs, *pg.EnvironmentId) {
					continue
				}
			}
			result = append(result, client.PostgresWithCursor{
				Cursor:   client.Cursor(fmt.Sprintf("c%d", i)),
				Postgres: postgresListItem(pg),
			})
		}
		writeJSON(w, http.StatusOK, result)
	})

	// POST /postgres — create a new Postgres instance.
	// Tests can assert against s.Postgres.Instances.
	mux.HandleFunc("POST /postgres", func(w http.ResponseWriter, r *http.Request) {
		record(r)

		var body client.CreatePostgresJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if status, hasError := s.Postgres.nextError(); hasError {
			w.WriteHeader(status)
			return
		}

		owner, ok := s.ownerByID(body.OwnerId)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		region := pointers.ValueOrDefault(body.Region, client.Oregon)
		databaseName := pointers.ValueOrDefault(body.DatabaseName, body.Name+"_db")
		databaseUser := pointers.ValueOrDefault(body.DatabaseUser, "appuser")

		ipAllowList := []client.CidrBlockAndDescription{}
		if body.IpAllowList != nil {
			ipAllowList = *body.IpAllowList
		}

		replicas := client.ReadReplicas{}
		if body.ReadReplicas != nil {
			for _, r := range *body.ReadReplicas {
				replicas = append(replicas, client.ReadReplica{
					Id:   testids.RandomPostgresID(),
					Name: r.Name,
				})
			}
		}

		id := testids.RandomPostgresID()
		pg := &client.PostgresDetail{
			Id:                      id,
			Name:                    body.Name,
			Plan:                    body.Plan,
			Version:                 body.Version,
			Region:                  region,
			Owner:                   owner,
			Status:                  client.DatabaseStatusCreating,
			DatabaseName:            databaseName,
			DatabaseUser:            databaseUser,
			DiskSizeGB:              body.DiskSizeGB,
			DiskAutoscalingEnabled:  pointers.ValueOrDefault(body.EnableDiskAutoscaling, false),
			HighAvailabilityEnabled: pointers.ValueOrDefault(body.EnableHighAvailability, false),
			EnvironmentId:           body.EnvironmentId,
			IpAllowList:             ipAllowList,
			ReadReplicas:            replicas,
			ParameterOverrides:      body.ParameterOverrides,
			DashboardUrl:            "https://dashboard.render.com/d/" + id,
			CreatedAt:               time.Now(),
			UpdatedAt:               time.Now(),
		}

		s.Postgres.Instances = append(s.Postgres.Instances, pg)
		writeJSON(w, http.StatusCreated, pg)
	})

	// GET /postgres/{id} — retrieve a Postgres instance
	mux.HandleFunc("GET /postgres/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		if status, hasError := s.Postgres.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		for _, pg := range s.Postgres.Instances {
			if pg.Id == id {
				writeJSON(w, http.StatusOK, pg)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// GET /postgres/{id}/connection-info — retrieve connection strings for a Postgres instance
	mux.HandleFunc("GET /postgres/{id}/connection-info", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		for _, pg := range s.Postgres.Instances {
			if pg.Id == id {
				writeJSON(w, http.StatusOK, &client.PostgresConnectionInfo{
					PsqlCommand:              "PGPASSWORD=fake-password psql fake-internal",
					InternalConnectionString: "postgres://fake-internal",
					ExternalConnectionString: "postgres://fake-external",
					Password:                 "fake-password",
				})
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// DELETE /postgres/{id} — delete a Postgres instance
	mux.HandleFunc("DELETE /postgres/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		id := r.PathValue("id")
		if status, hasError := s.Postgres.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		for i, pg := range s.Postgres.Instances {
			if pg.Id == id {
				s.Postgres.Instances = slices.Delete(s.Postgres.Instances, i, i+1)
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// POST /postgres/{id}/suspend — suspend a Postgres instance
	mux.HandleFunc("POST /postgres/{id}/suspend", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.Postgres.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		for _, pg := range s.Postgres.Instances {
			if pg.Id == id {
				pg.Status = client.DatabaseStatusSuspended
				pg.UpdatedAt = time.Now()
				w.WriteHeader(http.StatusAccepted)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// POST /postgres/{id}/resume — resume a suspended Postgres instance
	mux.HandleFunc("POST /postgres/{id}/resume", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.Postgres.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		for _, pg := range s.Postgres.Instances {
			if pg.Id == id {
				pg.Status = client.DatabaseStatusAvailable
				pg.UpdatedAt = time.Now()
				w.WriteHeader(http.StatusAccepted)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// PATCH /postgres/{id} — update a Postgres instance
	mux.HandleFunc("PATCH /postgres/{id}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		if status, hasError := s.Postgres.nextError(); hasError {
			w.WriteHeader(status)
			return
		}
		id := r.PathValue("id")
		var body client.PostgresPATCHInput
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		idx := slices.IndexFunc(s.Postgres.Instances, func(pg *client.PostgresDetail) bool {
			return pg.Id == id
		})
		if idx == -1 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		pg := s.Postgres.Instances[idx]
		if body.Name != nil {
			pg.Name = *body.Name
		}
		if body.Plan != nil {
			pg.Plan = *body.Plan
		}
		if body.DiskSizeGB != nil {
			pg.DiskSizeGB = body.DiskSizeGB
		}
		if body.EnableDiskAutoscaling != nil {
			pg.DiskAutoscalingEnabled = *body.EnableDiskAutoscaling
		}
		if body.EnableHighAvailability != nil {
			pg.HighAvailabilityEnabled = *body.EnableHighAvailability
		}
		if body.ParameterOverrides != nil {
			pg.ParameterOverrides = body.ParameterOverrides
		}
		if body.IpAllowList != nil {
			pg.IpAllowList = *body.IpAllowList
		}
		pg.UpdatedAt = time.Now()
		writeJSON(w, http.StatusOK, pg)
	})

	registerServiceRoutes(mux, s, record)

	s.server = httptest.NewServer(mux)
	t.Cleanup(s.server.Close)
	return s
}
