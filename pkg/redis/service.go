package redis

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

func (s *Service) ListRedis(ctx context.Context, params *client.ListRedisParams) ([]*Model, error) {
	redis, err := s.repo.ListRedis(ctx, params)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var redisModels []*Model

	for _, pg := range redis {
		model, err := s.hydrateRedisModel(ctx, pg, projects)
		if err != nil {
			return nil, err
		}
		redisModels = append(redisModels, model)
	}

	return redisModels, nil
}

func (s *Service) GetRedis(ctx context.Context, id string) (*Model, error) {
	redis, err := s.repo.GetRedis(ctx, id)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	return s.hydrateRedisModel(ctx, redisFromRedisDetail(redis), projects)
}

func (s *Service) hydrateRedisModel(ctx context.Context, redis *client.Redis, projects []*client.Project) (*Model, error) {
	model := &Model{Redis: redis}

	var envs = make([]*client.Environment, 0)
	env, err := s.environmentForRedis(ctx, redis, envs)
	if err != nil {
		return nil, err
	}
	model.Environment = env

	model.Project = s.projectForRedis(redis, projects)
	return model, nil
}

func (s *Service) environmentForRedis(ctx context.Context, pg *client.Redis, envs []*client.Environment) (*client.Environment, error) {
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

func (s *Service) projectForRedis(redis *client.Redis, projects []*client.Project) *client.Project {
	if redis.EnvironmentId == nil {
		return nil
	}

	for _, proj := range projects {
		for _, envID := range proj.EnvironmentIds {
			if *redis.EnvironmentId == envID {
				return proj
			}
		}
	}

	return nil
}

func redisFromRedisDetail(detail *client.RedisDetail) *client.Redis {
	// Just set the fields that are necessary for the model interface
	return &client.Redis{
		Id:            detail.Id,
		EnvironmentId: detail.EnvironmentId,
		Name:          detail.Name,
	}
}
