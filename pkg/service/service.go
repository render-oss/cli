package service

import (
	"context"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/environment"
	"github.com/renderinc/render-cli/pkg/project"
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

func (s *Service) ListServices(ctx context.Context) ([]*Model, error) {
	services, err := s.repo.ListServices(ctx)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var serviceModels []*Model
	var envs []*client.Environment

	for _, service := range services {
		model, err := s.hydrateServiceModel(ctx, service, projects, &envs)
		if err != nil {
			return nil, err
		}
		serviceModels = append(serviceModels, model)
	}

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

	var envs []*client.Environment
	return s.hydrateServiceModel(ctx, service, projects, &envs)
}

func (s *Service) RestartService(ctx context.Context, id string) error {
	return s.repo.RestartService(ctx, id)
}

func (s *Service) hydrateServiceModel(ctx context.Context, service *client.Service, projects []*client.Project, envs *[]*client.Environment) (*Model, error) {
	model := &Model{service: service}

	env, err := s.environmentForService(ctx, service, envs)
	if err != nil {
		return nil, err
	}
	model.environment = env

	model.project = s.projectForService(service, projects)
	return model, nil
}

func (s *Service) environmentForService(ctx context.Context, svc *client.Service, envs *[]*client.Environment) (*client.Environment, error) {
	if svc.EnvironmentId == nil {
		return nil, nil
	}

	for _, env := range *envs {
		if *svc.EnvironmentId == env.Id {
			return env, nil
		}
	}

	env, err := s.environmentRepo.GetEnvironment(ctx, *svc.EnvironmentId)
	if err != nil {
		return nil, err
	}

	*envs = append(*envs, env)
	return env, nil
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
