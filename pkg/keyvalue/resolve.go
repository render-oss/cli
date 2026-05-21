package keyvalue

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	rstrings "github.com/render-oss/cli/pkg/strings"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/validate"
)

type resolveScope struct {
	project *client.Project
	env     *client.Environment
}

// Resolve looks up a Key Value instance by ID or name within an optional
// project/environment scope.
//
// If env is supplied, Key Value name lookup is narrowed to that environment,
// and an ID lookup is rejected unless the Key Value belongs to that
// environment. If only project is supplied, name lookup is narrowed to all
// environments in that project, and ID lookup is rejected unless the Key Value
// belongs to one of them. If both are supplied, env is the concrete Key Value
// scope; the caller is responsible for resolving env relative to project
// before calling Resolve.
func Resolve(
	ctx context.Context,
	idOrName string,
	project *client.Project,
	env *client.Environment,
) (*client.KeyValueDetail, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	repo := NewRepo(c)
	return resolveInScopeWithRepo(ctx, repo, idOrName, resolveScope{project: project, env: env})
}

func resolveWithRepo(ctx context.Context, repo *Repo, idOrName string, env *client.Environment) (*client.KeyValueDetail, error) {
	return resolveInScopeWithRepo(ctx, repo, idOrName, resolveScope{env: env})
}

func resolveInScopeWithRepo(
	ctx context.Context,
	repo *Repo,
	idOrName string,
	scope resolveScope,
) (*client.KeyValueDetail, error) {
	inputLooksLikeID := validate.IsKeyValueID(idOrName)
	if inputLooksLikeID {
		detail, err := repo.GetKeyValue(ctx, idOrName)
		if err != nil && !errors.Is(err, ErrKeyValueNotFound) {
			return nil, err
		}
		if err == nil {
			if err := scope.checkMatch(detail); err != nil {
				return nil, err
			}
			return detail, nil
		}
		// 404: ID-shaped but no such KV exists. Could still be a name
		// that happens to match the ID shape, so fall through to the
		// list lookup below.
	}

	matches := []*client.KeyValue{}
	environmentIDs, isScoped := scope.environmentIDs()
	if !isScoped || len(environmentIDs) > 0 {
		params := &client.ListKeyValueParams{
			Name: &client.NameParam{idOrName},
		}
		if isScoped {
			envParam := client.EnvironmentIdParam(environmentIDs)
			params.EnvironmentId = &envParam
		}
		var err error
		matches, err = repo.ListKeyValue(ctx, params)
		if err != nil {
			return nil, err
		}
	}
	if len(matches) == 0 {
		return nil, scope.notFoundError(idOrName, inputLooksLikeID)
	}
	if len(matches) > 1 {
		return nil, tui.UserFacingError{Message: scope.multipleMatchesMessage(idOrName)}
	}
	return repo.GetKeyValue(ctx, matches[0].Id)
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

// checkMatch returns a UserFacingError when an ID lookup resolves to a Key
// Value outside the explicitly requested scope. Without this guard, --project
// or --environment would silently be ignored on the ID path.
func (s resolveScope) checkMatch(kv *client.KeyValueDetail) error {
	if s.env == nil && s.project == nil {
		return nil
	}
	if s.env != nil && kv.EnvironmentId != nil && *kv.EnvironmentId == s.env.Id {
		return nil
	}
	if s.env != nil {
		return tui.UserFacingError{Message: fmt.Sprintf(
			"Key Value %s is not in environment %s. Re-run without --environment, or pass the correct environment.",
			rstrings.ResourceLabel(kv.Name, kv.Id), environmentLabel(s.env),
		)}
	}
	if kv.EnvironmentId != nil && slices.Contains(s.project.EnvironmentIds, *kv.EnvironmentId) {
		return nil
	}
	return tui.UserFacingError{Message: fmt.Sprintf(
		"Key Value %s is not in project %s. Re-run without --project, or pass the correct project.",
		rstrings.ResourceLabel(kv.Name, kv.Id), projectLabel(s.project),
	)}
}

// notFoundError tailors the message based on whether the input was an
// ID-shaped string or a name. Name lookup is implicitly scoped to the active
// workspace, so for name failures we surface that context and tell the user
// how to broaden the search. When an environment was supplied, the scope is
// further narrowed and surfaced in the message.
func (s resolveScope) notFoundError(idOrName string, inputLooksLikeID bool) error {
	if inputLooksLikeID {
		return tui.UserFacingError{Message: fmt.Sprintf("No Key Value with ID '%s'.", idOrName)}
	}
	if s.env != nil {
		return tui.UserFacingError{Message: fmt.Sprintf(
			"No Key Value named '%s' in environment %s.",
			idOrName, environmentLabel(s.env),
		)}
	}
	if s.project != nil {
		return tui.UserFacingError{Message: fmt.Sprintf(
			"No Key Value named '%s' in project %s.",
			idOrName, projectLabel(s.project),
		)}
	}
	workspace := activeWorkspaceLabel()
	if workspace == "" {
		return tui.UserFacingError{Message: fmt.Sprintf("No Key Value named '%s'.", idOrName)}
	}
	return tui.UserFacingError{Message: fmt.Sprintf(
		"No Key Value named '%s' in workspace %s. To search another workspace, run `render workspace set <name|ID>`, or pass the Key Value ID instead.",
		idOrName, workspace,
	)}
}

func (s resolveScope) multipleMatchesMessage(idOrName string) string {
	if s.env != nil {
		return fmt.Sprintf("Multiple Key Value instances found with name '%s' in environment %s. Please specify the Key Value ID instead.", idOrName, environmentLabel(s.env))
	}
	if s.project != nil {
		return fmt.Sprintf(
			"Multiple Key Value instances found with name '%s' in project %s. Pass the Key Value ID, or use --environment <id|name> to disambiguate.",
			idOrName, projectLabel(s.project),
		)
	}
	return fmt.Sprintf("Multiple Key Value instances found with name '%s'. Pass the Key Value ID, or use --environment <id|name> to disambiguate.", idOrName)
}

func environmentLabel(env *client.Environment) string {
	if env == nil {
		return ""
	}
	return rstrings.ResourceLabel(env.Name, env.Id)
}

func projectLabel(project *client.Project) string {
	if project == nil {
		return ""
	}
	return rstrings.ResourceLabel(project.Name, project.Id)
}

// activeWorkspaceLabel returns a human-friendly label for the active workspace.
// When RENDER_WORKSPACE is set, both lookups return the ID; rstrings.ResourceLabel
// dedupes that case.
func activeWorkspaceLabel() string {
	id, _ := config.WorkspaceID()
	name, _ := config.WorkspaceName()
	return rstrings.ResourceLabel(name, id)
}
