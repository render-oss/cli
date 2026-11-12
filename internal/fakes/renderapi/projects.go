package renderapi

import "github.com/render-oss/cli/pkg/client"

// ProjectResource holds project fixtures and queued errors for the fake API.
type ProjectResource struct {
	Resource[*client.Project]
	getErrors  []int
	listErrors []int
}

// RespondWithErrorForGet queues an HTTP error for the next project retrieval.
func (r *ProjectResource) RespondWithErrorForGet(status int) {
	r.getErrors = append(r.getErrors, status)
}

// RespondWithErrorForList queues an HTTP error for the next project listing.
func (r *ProjectResource) RespondWithErrorForList(status int) {
	r.listErrors = append(r.listErrors, status)
}
