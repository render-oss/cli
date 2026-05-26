package postgres

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/environment"
	"github.com/render-oss/cli/pkg/project"
	"github.com/render-oss/cli/pkg/resolve"
	"github.com/render-oss/cli/pkg/resource/util"
	pgtypes "github.com/render-oss/cli/pkg/types/postgres"
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

	util.SortResources(postgresModels)

	return postgresModels, nil
}

func (s *Service) GetPostgres(ctx context.Context, id string) (*Model, error) {
	postgres, err := s.repo.GetPostgres(ctx, id)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	return s.hydratePostgresModel(ctx, postgresFromPostgresDetail(postgres), projects)
}

func (s *Service) RestartPostgresDatabase(ctx context.Context, id string) error {
	return s.repo.RestartPostgresDatabase(ctx, id)
}

// Create applies defaults, resolves workspace/project/environment scope,
// and calls the Postgres create endpoint. The non-interactive flag path
// and (eventually) the interactive wizard both go through here.
func (s *Service) Create(ctx context.Context, input pgtypes.CreatePostgresInput) (*client.PostgresDetail, error) {
	scope, err := s.resolver.ResolveScope(ctx, resolve.ScopeInput{
		WorkspaceIDOrName:   input.WorkspaceIDOrName,
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
	if err != nil {
		return nil, err
	}

	environmentID := scope.EnvironmentID()
	if environmentID == nil && input.ProjectIDOrName != nil {
		environmentID, err = s.resolver.ResolveEnvironmentID(ctx, scope.Project, nil, scope.WorkspaceID)
		if err != nil {
			return nil, err
		}
	}

	body, err := BuildCreateRequest(buildRequestInput(input, scope.WorkspaceID, environmentID))
	if err != nil {
		return nil, err
	}

	return s.repo.CreatePostgres(ctx, body)
}

func (s *Service) hydratePostgresModel(ctx context.Context, postgres *client.Postgres, projects []*client.Project) (*Model, error) {
	model := &Model{Postgres: postgres}

	var envs = make([]*client.Environment, 0)
	env, err := s.environmentForPostgres(ctx, postgres, envs)
	if err != nil {
		return nil, err
	}
	model.Environment = env

	model.Project = s.projectForPostgres(postgres, projects)
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

func postgresFromPostgresDetail(detail *client.PostgresDetail) *client.Postgres {
	// Just set the fields that are necessary for the model interface
	return &client.Postgres{
		Id:            detail.Id,
		EnvironmentId: detail.EnvironmentId,
		Name:          detail.Name,
	}
}
