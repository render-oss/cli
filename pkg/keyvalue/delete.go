package keyvalue

import (
	"context"
	"errors"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	"github.com/render-oss/cli/pkg/config"
	rstrings "github.com/render-oss/cli/pkg/strings"
	"github.com/render-oss/cli/pkg/tui"
	"github.com/render-oss/cli/pkg/validate"
)

// DeleteResult is the shape returned to callers (and serialized to JSON/YAML)
// describing the outcome of a delete attempt. Deleted is false when the caller
// only fetched the target (e.g. for a CLI-level preview); true after the
// resource has been removed.
type DeleteResult struct {
	KeyValue *client.KeyValueDetail `json:"keyValue"`
	Deleted  bool                   `json:"deleted"`
}

// Resolve looks up a Key Value instance by ID (red-...) or by name. If the
// input matches the ID shape, the ID lookup is tried first; on failure (or
// for non-ID input) it falls back to a name-filtered list. When env is
// supplied, name lookup is narrowed to that environment, and an ID lookup
// that returns a KV outside the environment is rejected. Returns a
// tui.UserFacingError when zero or multiple matches are found.
func Resolve(ctx context.Context, idOrName string, env *client.Environment) (*client.KeyValueDetail, error) {
	c, err := client.NewDefaultClient()
	if err != nil {
		return nil, err
	}
	return resolveWithRepo(ctx, NewRepo(c), idOrName, env)
}

func resolveWithRepo(ctx context.Context, repo *Repo, idOrName string, env *client.Environment) (*client.KeyValueDetail, error) {
	inputLooksLikeID := validate.IsKeyValueID(idOrName)
	if inputLooksLikeID {
		detail, err := repo.GetKeyValue(ctx, idOrName)
		if err != nil && !errors.Is(err, ErrKeyValueNotFound) {
			return nil, err
		}
		if err == nil {
			if err := checkEnvironmentMatch(detail, env); err != nil {
				return nil, err
			}
			return detail, nil
		}
		// 404: ID-shaped but no such KV exists. Could still be a name
		// that happens to match the ID shape, so fall through to the
		// list lookup below.
	}

	params := &client.ListKeyValueParams{
		Name: &client.NameParam{idOrName},
	}
	if env != nil {
		params.EnvironmentId = &client.EnvironmentIdParam{env.Id}
	}
	matches, err := repo.ListKeyValue(ctx, params)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, notFoundError(idOrName, inputLooksLikeID, env)
	}
	if len(matches) > 1 {
		return nil, tui.UserFacingError{Message: multipleMatchesMessage(idOrName, env)}
	}
	return repo.GetKeyValue(ctx, matches[0].Id)
}

// checkEnvironmentMatch returns a UserFacingError when an ID lookup resolves
// to a Key Value that does not belong to the explicitly requested environment.
// Without this guard, --environment would silently be ignored on the ID path
// and the user could delete something they didn't expect.
func checkEnvironmentMatch(kv *client.KeyValueDetail, env *client.Environment) error {
	if env == nil {
		return nil
	}
	if kv.EnvironmentId != nil && *kv.EnvironmentId == env.Id {
		return nil
	}
	return tui.UserFacingError{Message: fmt.Sprintf(
		"Key Value %s is not in environment %s. Re-run without --environment, or pass the correct environment.",
		rstrings.ResourceLabel(kv.Name, kv.Id), environmentLabel(env),
	)}
}

func multipleMatchesMessage(idOrName string, env *client.Environment) string {
	if env != nil {
		return fmt.Sprintf("Multiple Key Value instances found with name '%s' in environment %s. Please specify the Key Value ID instead.", idOrName, environmentLabel(env))
	}
	return fmt.Sprintf("Multiple Key Value instances found with name '%s'. Pass the Key Value ID, or use --environment <id|name> to disambiguate.", idOrName)
}

func environmentLabel(env *client.Environment) string {
	if env == nil {
		return ""
	}
	return rstrings.ResourceLabel(env.Name, env.Id)
}

// notFoundError tailors the message based on whether the input was an
// ID-shaped string or a name. Name lookup is implicitly scoped to the active
// workspace, so for name failures we surface that context and tell the user
// how to broaden the search. When an environment was supplied, the scope is
// further narrowed and surfaced in the message.
func notFoundError(idOrName string, inputLooksLikeID bool, env *client.Environment) error {
	if inputLooksLikeID {
		return tui.UserFacingError{Message: fmt.Sprintf("No Key Value with ID '%s'.", idOrName)}
	}
	if env != nil {
		return tui.UserFacingError{Message: fmt.Sprintf(
			"No Key Value named '%s' in environment %s.",
			idOrName, environmentLabel(env),
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

// activeWorkspaceLabel returns a human-friendly label for the active workspace.
// When RENDER_WORKSPACE is set, both lookups return the ID; rstrings.ResourceLabel
// dedupes that case.
func activeWorkspaceLabel() string {
	id, _ := config.WorkspaceID()
	name, _ := config.WorkspaceName()
	return rstrings.ResourceLabel(name, id)
}

// Delete removes the Key Value instance with the given ID via the Render API.
func Delete(ctx context.Context, id string) error {
	c, err := client.NewDefaultClient()
	if err != nil {
		return err
	}
	return NewRepo(c).DeleteKeyValue(ctx, id)
}
