package postgres

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

func (s *Service) ListPostgres(ctx context.Context, params *client.ListPostgresParams) ([]*Model, error) {
	postgres, err := s.repo.ListPostgres(ctx, params)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var postgresModels []*Model

	for _, pg := range postgres {
		model, err := s.hydratePostgresModel(ctx, pg, projects)
		if err != nil {
			return nil, err
		}
		postgresModels = append(postgresModels, model)
	}

	return postgresModels, nil
}

func (s *Service) RestartPostgresDatabase(ctx context.Context, id string) error {
	return s.repo.RestartPostgresDatabase(ctx, id)
}

func (s *Service) hydratePostgresModel(ctx context.Context, postgres *client.Postgres, projects []*client.Project) (*Model, error) {
	model := &Model{postgres: postgres}

	var envs = make([]*client.Environment, 0)
	env, err := s.environmentForPostgres(ctx, postgres, envs)
	if err != nil {
		return nil, err
	}
	model.environment = env

	model.project = s.projectForPostgres(postgres, projects)
	return model, nil
}

func (s *Service) environmentForPostgres(ctx context.Context, pg *client.Postgres, envs []*client.Environment) (*client.Environment, error) {
	if pg.EnvironmentId == nil {
		return nil, nil
	}

	for _, env := range envs {
		if *pg.EnvironmentId == env.Id {
			return env, nil
		}
	}

	env, err := s.environmentRepo.GetEnvironment(ctx, *pg.EnvironmentId)
	if err != nil {
		return nil, err
	}

	envs = append(envs, env)
	return env, nil
}

func (s *Service) projectForPostgres(postgres *client.Postgres, projects []*client.Project) *client.Project {
	if postgres.EnvironmentId == nil {
		return nil
	}

	for _, proj := range projects {
		for _, envID := range proj.EnvironmentIds {
			if *postgres.EnvironmentId == envID {
				return proj
			}
		}
	}

	return nil
}
