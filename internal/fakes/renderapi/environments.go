package renderapi

import "github.com/render-oss/cli/pkg/client"

// EnvironmentResource holds environment fixtures and queued errors for the fake API.
type EnvironmentResource struct {
	Resource[*client.Environment]
	getErrors  []int
	listErrors []int
}

// RespondWithErrorForGet queues an HTTP error for the next environment retrieval.
func (r *EnvironmentResource) RespondWithErrorForGet(status int) {
	r.getErrors = append(r.getErrors, status)
}

// RespondWithErrorForList queues an HTTP error for the next environment listing.
func (r *EnvironmentResource) RespondWithErrorForList(status int) {
	r.listErrors = append(r.listErrors, status)
}
