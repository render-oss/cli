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
	"github.com/render-oss/cli/pkg/validate"
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

// UpdateInput preserves omitted flags as nil so only supplied settings change.
type UpdateInput struct {
	BlueprintID string  `cli:"arg:0"`
	Name        *string `cli:"name"`
	Branch      *string `cli:"branch"`
	Path        *string `cli:"path"`
	AutoSync    *bool   `cli:"auto-sync"`
}

// Validate requires a Blueprint ID and at least one setting to change.
func (i UpdateInput) Validate(_isInteractive bool) error {
	var errs []error
	if !validate.IsBlueprintID(i.BlueprintID) {
		errs = append(errs, fmt.Errorf("invalid Blueprint ID %q: Blueprint IDs start with exs- followed by 20 lowercase letters or digits", i.BlueprintID))
	}
	if i.Name == nil && i.Branch == nil && i.Path == nil && i.AutoSync == nil {
		errs = append(errs, errors.New("provide at least one of --name, --branch, --path, or --auto-sync"))
	}
	if i.Name != nil && *i.Name == "" {
		errs = append(errs, errors.New("--name must not be empty"))
	}
	if i.Branch != nil && *i.Branch == "" {
		errs = append(errs, errors.New("--branch must not be empty"))
	}
	if i.Path != nil && *i.Path == "" {
		errs = append(errs, errors.New("--path must not be empty"))
	}
	return errors.Join(errs...)
}

// ApproveInput identifies the Blueprint whose newest sync to approve. An
// omitted --manual-env-vars-file remains nil so approval can fall back to the
// file that create and plan write.
type ApproveInput struct {
	BlueprintID       string  `cli:"arg:0"`
	ManualEnvVarsFile *string `cli:"manual-env-vars-file"`
}

// Validate reports input errors without reading the manual env vars file.
func (i ApproveInput) Validate(_isInteractive bool) error {
	var errs []error
	if !validate.IsBlueprintID(i.BlueprintID) {
		errs = append(errs, fmt.Errorf("invalid Blueprint ID %q: Blueprint IDs start with exs- followed by 20 lowercase letters or digits", i.BlueprintID))
	}
	if i.ManualEnvVarsFile != nil && *i.ManualEnvVarsFile == "" {
		errs = append(errs, errors.New("--manual-env-vars-file must not be empty"))
	}
	return errors.Join(errs...)
}

// PlanInput identifies the Blueprint to plan a sync for.
type PlanInput struct {
	BlueprintID string `cli:"arg:0"`
}

// Validate rejects anything that is not a Blueprint ID, such as a sync ID.
func (i PlanInput) Validate(_isInteractive bool) error {
	if !validate.IsBlueprintID(i.BlueprintID) {
		return fmt.Errorf("invalid Blueprint ID %q: Blueprint IDs start with exs- followed by 20 lowercase letters or digits", i.BlueprintID)
	}
	return nil
}

// Create validates and resolves input, then creates and plans the Blueprint in the given workspace.
func (s *Service) Create(ctx context.Context, workspaceID string, input CreateInput) (*bptypes.CreateBlueprintResponse, error) {
	body, err := input.buildCreateRequest(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, body)
}

// Approve approves the Blueprint's newest sync. A nil manualEnvVars omits
// manually-supplied environment variables from the request.
func (s *Service) Approve(ctx context.Context, blueprintID string, manualEnvVars []bptypes.SuppliedManualEnvVar) (*bptypes.ApprovedSync, error) {
	var body bptypes.ApproveBlueprintRequest
	if manualEnvVars != nil {
		body.ManualEnvVars = &manualEnvVars
	}
	return s.repo.Approve(ctx, blueprintID, body)
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
