package service

import (
	"context"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/project"
	"github.com/renderinc/render-cli/pkg/resource/util"
)

type Service struct {
	repo            *Repo
	environmentRepo *environment.Repo
	projectRepo     *project.Repo
}

func NewService(repo *Repo, environmentRepo *environment.Repo, projectRepo *project.Repo) *Service {
	return &Service{
		repo:            repo,
		environmentRepo: environmentRepo,
		projectRepo:     projectRepo,
	}
}

func (s *Service) ListServices(ctx context.Context, params *client.ListServicesParams) ([]*Model, error) {
	services, err := s.repo.ListServices(ctx, params)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	envs, err := s.allEnvironments(ctx, projects)
	if err != nil {
		return nil, err
	}

	var serviceModels []*Model

	for _, service := range services {
		model, err := s.hydrateServiceModelWithEnvs(service, projects, envs)
		if err != nil {
			return nil, err
		}
		serviceModels = append(serviceModels, model)
	}

	util.SortResources(serviceModels)
	return serviceModels, nil
}

func (s *Service) GetService(ctx context.Context, id string) (*Model, error) {
	service, err := s.repo.GetService(ctx, id)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	return s.hydrateServiceModel(ctx, service, projects)
}

func (s *Service) RestartService(ctx context.Context, id string) error {
	return s.repo.RestartService(ctx, id)
}

func (s *Service) hydrateServiceModel(ctx context.Context, service *client.Service, projects []*client.Project) (*Model, error) {
	model := &Model{Service: service}

	model.Project = s.projectForService(service, projects)

	if model.Project != nil {
		envs, err := s.allEnvironments(ctx, []*client.Project{model.Project})
		if err != nil {
			return nil, err
		}
		model.Environment = s.environmentForService(service, envs)
	}

	return model, nil
}

func (s *Service) hydrateServiceModelWithEnvs(service *client.Service, projects []*client.Project, envs []*client.Environment) (*Model, error) {
	model := &Model{Service: service}

	model.Project = s.projectForService(service, projects)
	model.Environment = s.environmentForService(service, envs)

	return model, nil
}

func (s *Service) environmentForService(svc *client.Service, envs []*client.Environment) *client.Environment {
	if svc.EnvironmentId == nil {
		return nil
	}

	for _, env := range envs {
		if *svc.EnvironmentId == env.Id {
			return env
		}
	}

	return nil
}

func (s *Service) projectForService(service *client.Service, projects []*client.Project) *client.Project {
	if service.EnvironmentId == nil {
		return nil
	}

	for _, proj := range projects {
		for _, envID := range proj.EnvironmentIds {
			if *service.EnvironmentId == envID {
				return proj
			}
		}
	}

	return nil
}

func (s *Service) allEnvironments(ctx context.Context, projects []*client.Project) ([]*client.Environment, error) {
	if len(projects) == 0 {
		return nil, nil
	}
	var projIDs []string
	for _, proj := range projects {
		projIDs = append(projIDs, proj.Id)
	}

	return s.environmentRepo.ListEnvironments(ctx, &client.ListEnvironmentsParams{ProjectId: projIDs})
}
