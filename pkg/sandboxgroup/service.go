package sandboxgroup

import (
	"context"

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

// List returns the sandbox groups for the active workspace. Alpha guarantees
// zero or one item.
func (s *Service) List(ctx context.Context) ([]*sandboxesclient.SandboxGroup, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, workspace)
}
