package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/resolve"
	rstrings "github.com/render-oss/cli/pkg/strings"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/validate"
)

// ResolveInput describes a Postgres lookup by ID or name within optional
// active-workspace project/environment scope.
type ResolveInput struct {
	IDOrName            string
	ProjectIDOrName     *string
	EnvironmentIDOrName *string
}

type resolveScope struct {
	project *client.Project
	env     *client.Environment
}

func (s *Service) resolve(ctx context.Context, input ResolveInput) (*ResolvedPostgres, error) {
	scope, err := s.resolveScope(ctx, input)
	if err != nil {
		return nil, err
	}
	pg, err := s.resolveInScope(ctx, input.IDOrName, scope)
	if err != nil {
		return nil, err
	}
	resolved := &ResolvedPostgres{
		Postgres:    pg,
		Project:     scope.project,
		Environment: scope.env,
	}
	if err := s.enrichResolvedPostgres(ctx, resolved); err != nil {
		return nil, err
	}
	return resolved, nil
}

// enrichResolvedPostgres fills in related project/environment data when the
// lookup itself did not already resolve that context.
func (s *Service) enrichResolvedPostgres(ctx context.Context, resolved *ResolvedPostgres) error {
	if resolved.Postgres == nil || resolved.Postgres.EnvironmentId == nil {
		return nil
	}

	pg := postgresFromPostgresDetail(resolved.Postgres)
	if resolved.Environment == nil {
		env, err := s.environmentForPostgres(ctx, pg, nil)
		if err != nil {
			return err
		}
		resolved.Environment = env
	}
	if resolved.Project == nil && resolved.Environment != nil {
		project, err := s.projectRepo.GetProject(ctx, resolved.Environment.ProjectId)
		if err != nil {
			return err
		}
		resolved.Project = project
	}
	return nil
}

func (s *Service) resolveScope(ctx context.Context, input ResolveInput) (resolveScope, error) {
	if input.ProjectIDOrName == nil && input.EnvironmentIDOrName == nil {
		return resolveScope{}, nil
	}

	resolved, err := s.resolver.ResolveScopeInActiveWorkspace(ctx, resolve.ActiveWorkspaceScopeInput{
		ProjectIDOrName:     input.ProjectIDOrName,
		EnvironmentIDOrName: input.EnvironmentIDOrName,
	})
	if err != nil {
		return resolveScope{}, err
	}
	return resolveScope{project: resolved.Project, env: resolved.Environment}, nil
}

func (s *Service) resolveInScope(ctx context.Context, idOrName string, scope resolveScope) (*client.PostgresDetail, error) {
	inputLooksLikeID := validate.IsPostgresID(idOrName)
	if inputLooksLikeID {
		detail, err := s.repo.GetPostgres(ctx, idOrName)
		if err != nil && !errors.Is(err, ErrPostgresNotFound) {
			return nil, err
		}
		if err == nil {
			if err := scope.checkMatch(detail); err != nil {
				return nil, err
			}
			return detail, nil
		}
	}

	params := &client.ListPostgresParams{
		Name: &client.NameParam{idOrName},
	}
	environmentIDs, isScoped := scope.environmentIDs()
	if isScoped && len(environmentIDs) == 0 {
		return nil, scope.notFoundError(idOrName, inputLooksLikeID)
	}
	if isScoped {
		envParam := client.EnvironmentIdParam(environmentIDs)
		params.EnvironmentId = &envParam
	}

	matches, err := s.repo.ListPostgres(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, scope.notFoundError(idOrName, inputLooksLikeID)
	}
	if len(matches) > 1 {
		return nil, tui.UserFacingError{Message: scope.multipleMatchesMessage(idOrName)}
	}
	return s.repo.GetPostgres(ctx, matches[0].Id)
}

func (s resolveScope) environmentIDs() ([]string, bool) {
	if s.env != nil {
		return []string{s.env.Id}, true
	}
	if s.project != nil {
		return s.project.EnvironmentIds, true
	}
	return nil, false
}

func (s resolveScope) checkMatch(pg *client.PostgresDetail) error {
	if s.env == nil && s.project == nil {
		return nil
	}
	if s.env != nil && pg.EnvironmentId != nil && *pg.EnvironmentId == s.env.Id {
		return nil
	}
	if s.env != nil {
		return tui.UserFacingError{Message: fmt.Sprintf(
			"Postgres database %s is not in environment %s. Re-run without --environment, or pass the correct environment.",
			rstrings.ResourceLabel(pg.Name, pg.Id), postgresEnvironmentLabel(s.env),
		)}
	}
	if pg.EnvironmentId != nil {
		for _, envID := range s.project.EnvironmentIds {
			if *pg.EnvironmentId == envID {
				return nil
			}
		}
	}
	return tui.UserFacingError{Message: fmt.Sprintf(
		"Postgres database %s is not in project %s. Re-run without --project, or pass the correct project.",
		rstrings.ResourceLabel(pg.Name, pg.Id), postgresProjectLabel(s.project),
	)}
}

func (s resolveScope) notFoundError(idOrName string, inputLooksLikeID bool) error {
	if inputLooksLikeID {
		return tui.UserFacingError{Message: fmt.Sprintf("No Postgres database with ID '%s'.", idOrName)}
	}
	if s.env != nil {
		return tui.UserFacingError{Message: fmt.Sprintf(
			"No Postgres database named '%s' in environment %s.",
			idOrName, postgresEnvironmentLabel(s.env),
		)}
	}
	if s.project != nil {
		return tui.UserFacingError{Message: fmt.Sprintf(
			"No Postgres database named '%s' in project %s.",
			idOrName, postgresProjectLabel(s.project),
		)}
	}
	workspace := activeWorkspaceLabel()
	if workspace == "" {
		return tui.UserFacingError{Message: fmt.Sprintf("No Postgres database named '%s'.", idOrName)}
	}
	return tui.UserFacingError{Message: fmt.Sprintf(
		"No Postgres database named '%s' in workspace %s. To search another workspace, run `render workspace set <name|ID>`, or pass the Postgres database ID instead.",
		idOrName, workspace,
	)}
}

func (s resolveScope) multipleMatchesMessage(idOrName string) string {
	if s.env != nil {
		return fmt.Sprintf("Multiple Postgres databases found with name '%s' in environment %s. Please specify the Postgres database ID instead.", idOrName, postgresEnvironmentLabel(s.env))
	}
	if s.project != nil {
		return fmt.Sprintf(
			"Multiple Postgres databases found with name '%s' in project %s. Pass the Postgres database ID, or use --environment <id|name> to disambiguate.",
			idOrName, postgresProjectLabel(s.project),
		)
	}
	return fmt.Sprintf("Multiple Postgres databases found with name '%s'. Pass the Postgres database ID, or use --environment <id|name> to disambiguate.", idOrName)
}

func postgresEnvironmentLabel(env *client.Environment) string {
	if env == nil {
		return ""
	}
	return rstrings.ResourceLabel(env.Name, env.Id)
}

func postgresProjectLabel(project *client.Project) string {
	if project == nil {
		return ""
	}
	return rstrings.ResourceLabel(project.Name, project.Id)
}

func activeWorkspaceLabel() string {
	id, _ := config.WorkspaceID()
	name, _ := config.WorkspaceName()
	return rstrings.ResourceLabel(name, id)
}
