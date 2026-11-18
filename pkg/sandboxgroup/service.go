package sandboxgroup

import (
	"context"
	"fmt"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/pointers"
	"github.com/render-oss/cli/pkg/resolve"
)

// Service holds business logic for sandbox groups: resolving the active
// workspace before delegating to the Repo.
type Service struct {
	repo     *Repo
	resolver *resolve.Resolver
}

func NewService(repo *Repo, resolver *resolve.Resolver) *Service {
	return &Service{repo: repo, resolver: resolver}
}

// CreateInput names a new sandbox group.
type CreateInput struct {
	Name string
}

// List returns the sandbox groups for the active workspace.
func (s *Service) List(ctx context.Context) ([]*sandboxesclient.SandboxGroup, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, workspace)
}

// IDOrDefault returns sandboxGroupID if it is not empty, or the ID of the
// active workspace's default sandbox group if there is one.
func (s *Service) IDOrDefault(ctx context.Context, sandboxGroupID string) (string, error) {
	if sandboxGroupID != "" {
		return sandboxGroupID, nil
	}
	groups, err := s.List(ctx)
	if err != nil {
		return "", err
	}
	for _, group := range groups {
		if group.IsDefault {
			return group.Id, nil
		}
	}
	return "", fmt.Errorf("no default sandbox group found in the active workspace; specify --group")
}

// Create creates a sandbox group in the active workspace.
func (s *Service) Create(ctx context.Context, input CreateInput) (*sandboxesclient.SandboxGroup, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, sandboxesclient.SandboxGroupPOST{
		OwnerId: workspace,
		Name:    input.Name,
	})
}

func (s *Service) resolveEnvironmentID(ctx context.Context, envIDOrName *string) (*string, error) {
	if envIDOrName == nil {
		return nil, nil
	}

	scope, err := s.resolver.ResolveScopeInActiveWorkspace(ctx, resolve.ActiveWorkspaceScopeInput{
		EnvironmentIDOrName: envIDOrName,
	})
	if err != nil {
		return nil, err
	}

	return scope.EnvironmentID(), nil
}

func (s *Service) Get(ctx context.Context, id string) (*sandboxesclient.SandboxGroup, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) SetEnvironment(
	ctx context.Context,
	groupID string,
	environmentIDOrName string,
) (*sandboxesclient.SandboxGroup, error) {
	environmentID, err := s.resolveEnvironmentID(ctx, &environmentIDOrName)
	if err != nil {
		return nil, err
	}

	if err := s.repo.AddToEnvironment(ctx, *environmentID, groupID); err != nil {
		return nil, err
	}

	return s.repo.Get(ctx, groupID)
}

func (s *Service) ClearEnvironment(ctx context.Context, groupID string) (*sandboxesclient.SandboxGroup, error) {
	group, err := s.repo.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}

	environmentID := pointers.StringValue(group.EnvironmentId)
	if environmentID == "" {
		return group, nil
	}

	if err := s.repo.RemoveFromEnvironment(ctx, environmentID, groupID); err != nil {
		return nil, err
	}

	return s.repo.Get(ctx, groupID)
}

func (s *Service) SetDefault(ctx context.Context, id string) (*sandboxesclient.SandboxGroup, error) {
	return s.repo.SetDefault(ctx, id)
}
