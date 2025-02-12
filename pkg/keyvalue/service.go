package keyvalue

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resource/util"
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

func (s *Service) ListKeyValue(ctx context.Context, params *client.ListKeyValueParams) ([]*Model, error) {
	kvs, err := s.repo.ListKeyValue(ctx, params)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	var keyValueModels []*Model

	for _, kv := range kvs {
		model, err := s.hydrateKeyValueModel(ctx, kv, projects)
		if err != nil {
			return nil, err
		}
		keyValueModels = append(keyValueModels, model)
	}

	util.SortResources(keyValueModels)
	return keyValueModels, nil
}

func (s *Service) GetKeyValue(ctx context.Context, id string) (*Model, error) {
	kv, err := s.repo.GetKeyValue(ctx, id)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	return s.hydrateKeyValueModel(ctx, keyValueFromKeyValueDetail(kv), projects)
}

func (s *Service) hydrateKeyValueModel(ctx context.Context, kv *client.KeyValue, projects []*client.Project) (*Model, error) {
	model := &Model{KeyValue: kv}

	var envs = make([]*client.Environment, 0)
	env, err := s.environmentForKeyValue(ctx, kv, envs)
	if err != nil {
		return nil, err
	}
	model.Environment = env

	model.Project = s.projectForKeyValue(kv, projects)
	return model, nil
}

func (s *Service) environmentForKeyValue(ctx context.Context, kv *client.KeyValue, envs []*client.Environment) (*client.Environment, error) {
	if kv.EnvironmentId == nil {
		return nil, nil
	}

	for _, env := range envs {
		if *kv.EnvironmentId == env.Id {
			return env, nil
		}
	}

	env, err := s.environmentRepo.GetEnvironment(ctx, *kv.EnvironmentId)
	if err != nil {
		return nil, err
	}

	envs = append(envs, env)
	return env, nil
}

func (s *Service) projectForKeyValue(kv *client.KeyValue, projects []*client.Project) *client.Project {
	if kv.EnvironmentId == nil {
		return nil
	}

	for _, proj := range projects {
		for _, envID := range proj.EnvironmentIds {
			if *kv.EnvironmentId == envID {
				return proj
			}
		}
	}

	return nil
}

func keyValueFromKeyValueDetail(detail *client.KeyValueDetail) *client.KeyValue {
	// Just set the fields that are necessary for the model interface
	return &client.KeyValue{
		Id:            detail.Id,
		EnvironmentId: detail.EnvironmentId,
		Name:          detail.Name,
	}
}
