package renderapi

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	clientjob "github.com/render-oss/cli/pkg/client/jobs"
	"github.com/render-oss/cli/pkg/pointers"
)

type jobRetrieveResponse struct {
	statusCode int
	status     *clientjob.JobStatus
}

// JobResource holds one-off job state and deterministic response queues.
type JobResource struct {
	Resource[*clientjob.Job]
	mu                sync.Mutex
	createErrors      []int
	retrieveResponses []jobRetrieveResponse
	createRequests    int
	retrieveRequests  int
	retrieved         chan struct{}
}

func newJobResource() *JobResource {
	return &JobResource{retrieved: make(chan struct{}, 16)}
}

// NewJob returns a one-off job with sensible defaults for zero-value fields.
func NewJob(value clientjob.Job) *clientjob.Job {
	if value.Id == "" {
		value.Id = "job-fake0000000000000000"
	}
	if value.ServiceId == "" {
		value.ServiceId = testids.RandomServiceID()
	}
	if value.PlanId == "" {
		value.PlanId = "plan-srv-006"
	}
	if value.CreatedAt.IsZero() {
		value.CreatedAt = time.Now().UTC()
	}
	if value.Status == nil {
		value.Status = pointers.From(clientjob.Pending)
	}
	return &value
}

// RespondCreateWith queues an HTTP failure for the next create request.
func (j *JobResource) RespondCreateWith(statusCode int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.createErrors = append(j.createErrors, statusCode)
}

// QueueStatuses returns these statuses on successive retrieve requests.
func (j *JobResource) QueueStatuses(statuses ...*clientjob.JobStatus) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, status := range statuses {
		j.retrieveResponses = append(j.retrieveResponses, jobRetrieveResponse{statusCode: http.StatusOK, status: status})
	}
}

// RespondRetrieveWith queues an HTTP failure for the next retrieve request.
func (j *JobResource) RespondRetrieveWith(statusCode int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.retrieveResponses = append(j.retrieveResponses, jobRetrieveResponse{statusCode: statusCode})
}

func (j *JobResource) CreateRequestCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.createRequests
}

func (j *JobResource) RetrieveRequestCount() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.retrieveRequests
}

func (j *JobResource) Retrieved() <-chan struct{} { return j.retrieved }

func (s *Server) registerJobRoutes(mux *http.ServeMux, record func(*http.Request)) {
	mux.HandleFunc("POST /services/{serviceId}/jobs", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		s.Jobs.mu.Lock()
		defer s.Jobs.mu.Unlock()
		s.Jobs.createRequests++
		if len(s.Jobs.createErrors) > 0 {
			statusCode := s.Jobs.createErrors[0]
			s.Jobs.createErrors = s.Jobs.createErrors[1:]
			message := http.StatusText(statusCode)
			writeJSON(w, statusCode, client.Error{Message: &message})
			return
		}

		var body client.PostJobJSONRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		job := NewJob(clientjob.Job{
			Id:           "job-fake0000000000000000",
			ServiceId:    r.PathValue("serviceId"),
			StartCommand: body.StartCommand,
			PlanId:       pointers.ValueOrDefault(body.PlanId, "plan-srv-006"),
		})
		s.Jobs.Instances = append(s.Jobs.Instances, job)
		writeJSON(w, http.StatusCreated, job)
	})

	mux.HandleFunc("GET /services/{serviceId}/jobs/{jobId}", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		s.Jobs.mu.Lock()
		defer s.Jobs.mu.Unlock()
		s.Jobs.retrieveRequests++

		var found *clientjob.Job
		for _, candidate := range s.Jobs.Instances {
			if candidate.Id == r.PathValue("jobId") && candidate.ServiceId == r.PathValue("serviceId") {
				found = candidate
				break
			}
		}
		if found == nil {
			message := "job not found"
			writeJSON(w, http.StatusNotFound, client.Error{Message: &message})
			return
		}

		if len(s.Jobs.retrieveResponses) > 0 {
			response := s.Jobs.retrieveResponses[0]
			s.Jobs.retrieveResponses = s.Jobs.retrieveResponses[1:]
			if response.statusCode != http.StatusOK {
				message := http.StatusText(response.statusCode)
				writeJSON(w, response.statusCode, client.Error{Message: &message})
				return
			}
			found.Status = response.status
		}

		copy := *found
		writeJSON(w, http.StatusOK, &copy)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		s.Jobs.retrieved <- struct{}{}
	})
}
