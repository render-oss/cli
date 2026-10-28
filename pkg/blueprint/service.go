package blueprint

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"

	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
	"github.com/render-oss/cli/pkg/utils"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// CreateInput preserves omitted flags as nil before resolving creation defaults.
type CreateInput struct {
	Repo              *string                       `cli:"repo"`
	Branch            *string                       `cli:"branch"`
	Path              *string                       `cli:"path"`
	Name              *string                       `cli:"name"`
	AutoSync          *bool                         `cli:"auto-sync"`
	ExistingResources *bptypes.ExistingResourceMode `cli:"existing-resources"`
}

// Validate reports input errors without applying creation defaults.
func (i CreateInput) Validate(_isInteractive bool) error {
	var errs []error
	if i.Repo == nil || *i.Repo == "" {
		errs = append(errs, errors.New("--repo is required"))
	} else if repoURL, err := url.Parse(utils.NormalizeRemoteURL(*i.Repo)); err != nil ||
		(repoURL.Scheme != "https" && repoURL.Scheme != "ssh") || repoURL.Hostname() == "" {
		errs = append(errs, errors.New("--repo must be a valid https:// or ssh:// URL"))
	}
	if i.ExistingResources != nil && !i.ExistingResources.Valid() {
		errs = append(errs, errors.New("--existing-resources must be create_new or adopt"))
	}
	if i.Path != nil && *i.Path == "" {
		errs = append(errs, errors.New("--path must not be empty"))
	}
	if i.Branch != nil && *i.Branch == "" {
		errs = append(errs, errors.New("--branch must not be empty"))
	}
	if i.Name != nil && *i.Name == "" {
		errs = append(errs, errors.New("--name must not be empty"))
	}
	return errors.Join(errs...)
}

// Create validates and resolves input, then creates and plans the Blueprint in the given workspace.
func (s *Service) Create(ctx context.Context, workspaceID string, input CreateInput) (*bptypes.CreateBlueprintResponse, error) {
	body, err := input.buildCreateRequest(workspaceID)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, body)
}

// buildCreateRequest validates input, supplies defaults for omitted values,
// and builds the REST create request for the supplied workspace.
func (i CreateInput) buildCreateRequest(workspaceID string) (bptypes.CreateBlueprintRequest, error) {
	if err := i.Validate(false); err != nil {
		return bptypes.CreateBlueprintRequest{}, err
	}
	repo := utils.NormalizeRemoteURL(*i.Repo)
	name := i.Name
	if name == nil {
		repoURL, err := url.Parse(repo)
		if err != nil {
			return bptypes.CreateBlueprintRequest{}, fmt.Errorf("parse repository URL: %w", err)
		}
		name = new(strings.TrimSuffix(path.Base(repoURL.Path), ".git"))
		if *name == "" || *name == "." || *name == "/" {
			return bptypes.CreateBlueprintRequest{}, errors.New("could not determine the repository name; provide --name")
		}
	}

	blueprintPath := i.Path
	if blueprintPath == nil {
		blueprintPath = new("render.yaml")
	}
	autoSync := i.AutoSync
	if autoSync == nil {
		autoSync = new(true)
	}
	mode := i.ExistingResources
	if mode == nil {
		mode = new(bptypes.CreateNew)
	}
	return bptypes.CreateBlueprintRequest{
		OwnerId: workspaceID,
		Name:    *name,
		Source: bptypes.BlueprintSource{
			Repo: repo, Branch: i.Branch, Path: blueprintPath,
		},
		AutoSync:          autoSync,
		ExistingResources: mode,
	}, nil
}
