package renderapi

import (
	"fmt"
	"net/http"

	"github.com/render-oss/cli/internal/testids"
	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/pointers"
)

// DeployResource holds the deploys of fake services. Each entry names the
// service that owns the deploy, because [client.Deploy] has no service field.
type DeployResource struct {
	Resource[*ServiceDeploy]
}

// ServiceDeploy is a deploy together with the ID of the service that owns it.
type ServiceDeploy struct {
	ServiceID string
	Deploy    *client.Deploy
}

// NewDeploy builds a deploy for serviceID. A missing ID gets a random deploy ID.
func NewDeploy(serviceID string, deploy client.Deploy) *ServiceDeploy {
	if deploy.Id == "" {
		deploy.Id = testids.RandomDeployID()
	}
	return &ServiceDeploy{ServiceID: serviceID, Deploy: &deploy}
}

// ForService returns the deploys of serviceID in seed order.
func (d *DeployResource) ForService(serviceID string) []*client.Deploy {
	var deploys []*client.Deploy
	for _, entry := range d.Instances {
		if entry.ServiceID == serviceID {
			deploys = append(deploys, entry.Deploy)
		}
	}
	return deploys
}

func registerDeployRoutes(mux *http.ServeMux, s *Server, record func(*http.Request)) {
	// GET /services/{id}/deploys - list the deploys of a service
	mux.HandleFunc("GET /services/{id}/deploys", func(w http.ResponseWriter, r *http.Request) {
		record(r)
		deploys := s.Deploys.ForService(r.PathValue("id"))
		result := make([]client.DeployWithCursor, 0, len(deploys))
		for i, deploy := range deploys {
			result = append(result, client.DeployWithCursor{
				Cursor: pointers.From(client.Cursor(fmt.Sprintf("c%d", i))),
				Deploy: deploy,
			})
		}
		writeJSON(w, http.StatusOK, result)
	})
}
