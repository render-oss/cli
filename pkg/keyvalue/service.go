package keyvalue

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/resource/util"
	kvtypes "github.com/render-oss/cli/pkg/types/keyvalue"
)

type Service struct {
	repo            *Repo
	environmentRepo *environment.Repo
	projectRepo     *project.Repo
	resolver        *resolve.Resolver
}

func NewService(repo *Repo, environmentRepo *environment.Repo, projectRepo *project.Repo, resolver *resolve.Resolver) *Service {
	return &Service{
		repo:            repo,
		environmentRepo: environmentRepo,
		projectRepo:     projectRepo,
		resolver:        resolver,
	}
}

// ResolveInput describes a Key Value lookup by ID or name within optional
// active-workspace project/environment scope.
type ResolveInput struct {
	IDOrName            string
	ProjectIDOrName     *string
	EnvironmentIDOrName *string
}

// ResolvedKeyValue carries a Key Value detail plus related resources needed to
// build user-facing output.
type ResolvedKeyValue struct {
	KeyValue    *client.KeyValueDetail
	Project     *client.Project
	Environment *client.Environment
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

// Resolve resolves a Key Value instance by ID or name within an optional
// active-workspace project/environment scope.
func (s *Service) Resolve(ctx context.Context, input ResolveInput) (*ResolvedKeyValue, error) {
	return s.resolve(ctx, input)
}

// Create applies defaults, resolves the requested scope, creates a Key Value,
// and returns the created resource with resolved project/environment context.
func (s *Service) Create(ctx context.Context, input kvtypes.KeyValueCreateInput) (*ResolvedKeyValue, error) {
	return s.create(ctx, input)
}

func (s *Service) GetConnectionInfo(ctx context.Context, id string) (*client.KeyValueConnectionInfo, error) {
	return s.repo.GetKeyValueConnectionInfo(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.DeleteKeyValue(ctx, id)
}

func (s *Service) Suspend(ctx context.Context, id string) error {
	return s.repo.SuspendKeyValue(ctx, id)
}

func (s *Service) Resume(ctx context.Context, id string) error {
	return s.repo.ResumeKeyValue(ctx, id)
}

func (s *Service) Update(ctx context.Context, input kvtypes.KeyValueUpdateInput) (*UpdateOutcome, error) {
	normalized, err := kvtypes.NormalizeAndValidateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	before, err := s.Resolve(ctx, ResolveInput{
		IDOrName:            normalized.IDOrName,
		EnvironmentIDOrName: normalized.EnvironmentIDOrName,
	})
	if err != nil {
		return nil, err
	}

	body, err := BuildUpdateRequest(normalized)
	if err != nil {
		return nil, err
	}

	after, err := s.repo.UpdateKeyValue(ctx, before.KeyValue.Id, body)
	if err != nil {
		return nil, err
	}
	return &UpdateOutcome{
		Before: before.KeyValue,
		After: &ResolvedKeyValue{
			KeyValue:    after,
			Project:     before.Project,
			Environment: before.Environment,
		},
	}, nil
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
