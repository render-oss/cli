package sandboxgroup

import (
	"context"
	"fmt"

	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
)

// Service holds business logic for sandbox groups: resolving the active
// workspace before delegating to the Repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
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
