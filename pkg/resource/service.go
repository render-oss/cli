package resource

import (
	"context"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/service"
)

type Resource interface {
	ID() string
	Name() string
	Environment() *client.Environment
	EnvironmentName() string
	Project() *client.Project
	ProjectName() string
	Type() string
}

type Service struct {
	serviceService  *service.Service
	environmentRepo *environment.Repo
	projectRepo     *project.Repo
}

func NewResourceService(serviceService *service.Service, environmentRepo *environment.Repo, projectRepo *project.Repo) *Service {
	return &Service{
		serviceService:  serviceService,
		environmentRepo: environmentRepo,
		projectRepo:     projectRepo,
	}
}

func (rs *Service) ListResources(ctx context.Context) ([]Resource, error) {
	services, err := rs.serviceService.ListServices(ctx)
	if err != nil {
		return nil, err
	}

	var resources []Resource

	for _, svc := range services {
		resources = append(resources, svc)
	}

	return resources, nil
}
