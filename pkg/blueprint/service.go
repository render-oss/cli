package blueprint

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"strings"

	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
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
	if i.Repo != nil {
		if _, err := normalizeGitRepoURL(*i.Repo); err != nil {
			errs = append(errs, errors.New("--repo must be a valid https:// or ssh:// URL"))
		}
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
	body, err := input.buildCreateRequest(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, body)
}

// buildCreateRequest validates input, resolves defaults,
// and builds the REST create request for the supplied workspace.
func (i CreateInput) buildCreateRequest(ctx context.Context, workspaceID string) (bptypes.CreateBlueprintRequest, error) {
	if err := i.Validate(false); err != nil {
		return bptypes.CreateBlueprintRequest{}, err
	}
	var repo string
	var err error
	if i.Repo == nil {
		if _, err := exec.LookPath("git"); err != nil {
			return bptypes.CreateBlueprintRequest{}, errors.New("cannot determine the repository because git is not on PATH; provide --repo, or install Git")
		}
		repo, err = getRemoteOrigin(ctx)
	} else {
		repo, err = normalizeGitRepoURL(*i.Repo)
	}
	if err != nil {
		return bptypes.CreateBlueprintRequest{}, err
	}
	branch := i.Branch
	if i.Repo == nil && branch == nil {
		localBranch, err := getLocalBranch(ctx)
		if err != nil {
			return bptypes.CreateBlueprintRequest{}, err
		}
		branch = &localBranch
	}
	source := bptypes.BlueprintSource{Repo: repo, Branch: branch, Path: i.Path}
	name := i.Name
	if name == nil {
		repoURL, err := url.Parse(source.Repo)
		if err != nil {
			return bptypes.CreateBlueprintRequest{}, fmt.Errorf("parse repository URL: %w", err)
		}
		name = new(strings.TrimSuffix(path.Base(repoURL.Path), ".git"))
		if *name == "" || *name == "." || *name == "/" {
			return bptypes.CreateBlueprintRequest{}, errors.New("could not determine the repository name; provide --name")
		}
	}

	if source.Path == nil {
		source.Path = new("render.yaml")
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
		OwnerId:           workspaceID,
		Name:              *name,
		Source:            source,
		AutoSync:          autoSync,
		ExistingResources: mode,
	}, nil
}
