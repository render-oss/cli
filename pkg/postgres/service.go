package postgres

import (
	"context"
	"maps"

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

// ListPostgres lists Postgres databases using already-resolved API params.
// Prefer List for command-facing project/environment selectors that still need
// active-workspace scope resolution.
func (s *Service) ListPostgres(ctx context.Context, params *client.ListPostgresParams) ([]*Model, error) {
	postgres, err := s.repo.ListPostgres(ctx, params)
	if err != nil {
		return nil, err
	}

	projects, err := s.projectRepo.ListProjects(ctx)
	if err != nil {
		return nil, err
	}

	postgresModels := []*Model{}

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

// List resolves active-workspace project/environment selectors into API params
// before listing Postgres databases.
func (s *Service) List(ctx context.Context, input pgtypes.ListPostgresInput) ([]*Model, error) {
	params := &client.ListPostgresParams{}

	if input.HasFilter() {
		envIDs, err := s.listEnvIDs(ctx, input)
		if err != nil {
			return nil, err
		}
		if len(envIDs) == 0 {
			return []*Model{}, nil
		}
		params.EnvironmentId = &envIDs
	}

	return s.ListPostgres(ctx, params)
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

func (s *Service) GetConnectionInfo(ctx context.Context, id string) (*client.PostgresConnectionInfo, error) {
	return s.repo.GetPostgresConnectionInfo(ctx, id)
}

// Resolve resolves a Postgres database by ID or name within an optional
// active-workspace project/environment scope.
func (s *Service) Resolve(ctx context.Context, input ResolveInput) (*client.PostgresDetail, error) {
	return s.resolve(ctx, input)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.DeletePostgres(ctx, id)
}

func (s *Service) SuspendPostgres(ctx context.Context, id string) error {
	return s.repo.SuspendPostgres(ctx, id)
}

func (s *Service) ResumePostgres(ctx context.Context, id string) error {
	return s.repo.ResumePostgres(ctx, id)
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

// Update resolves the target Postgres database (by ID or name, optionally
// narrowed by project/environment), builds the PATCH body, and applies it via
// the Render API. Returns both the pre- and post-update server state.
func (s *Service) Update(ctx context.Context, input pgtypes.UpdatePostgresInput) (*UpdateResult, error) {
	before, err := s.Resolve(ctx, ResolveInput{
		IDOrName:            input.IDOrName,
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
	if err != nil {
		return nil, err
	}

	body, err := BuildUpdateRequest(input)
	if err != nil {
		return nil, err
	}

	// The API treats parameterOverrides as a full replacement when supplied, so
	// merge the caller's entries into the existing server-side map to preserve
	// overrides the caller didn't mention (upsert, not replace).
	if body.ParameterOverrides != nil && before.ParameterOverrides != nil {
		merged := make(client.PostgresParameterOverrides)
		maps.Copy(merged, *before.ParameterOverrides)
		maps.Copy(merged, *body.ParameterOverrides)
		body.ParameterOverrides = &merged
	}

	after, err := s.repo.UpdatePostgres(ctx, before.Id, body)
	if err != nil {
		return nil, err
	}

	return &UpdateResult{Before: before, After: after}, nil
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

// listEnvIDs translates active-workspace project/environment selectors into
// environment IDs to filter on. A valid project with no environments returns
// an empty ID list, which callers should treat as an empty resource list rather
// than an invalid selector.
func (s *Service) listEnvIDs(ctx context.Context, input pgtypes.ListPostgresInput) ([]string, error) {
	scope, err := s.resolver.ResolveScopeInActiveWorkspace(ctx, resolve.ActiveWorkspaceScopeInput{
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
	if err != nil {
		return nil, err
	}
	if scope.Environment != nil {
		return []string{scope.Environment.Id}, nil
	}
	// A successful filtered scope without a single environment is a project
	// filter; use that project's environments as the candidate set.
	return scope.Project.EnvironmentIds, nil
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
