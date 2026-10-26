package sandboxsnapshot

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/sandboxgroup"
)

type Service struct {
	repo   *Repo
	groups *sandboxgroup.Service
}

func NewService(repo *Repo, groups *sandboxgroup.Service) *Service {
	return &Service{repo: repo, groups: groups}
}

type CreateInput struct {
	SandboxID string
	Kind      string
	Name      *string
}

type ListInput struct {
	SandboxGroupID string
	Statuses       []string
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*sandboxesclient.SandboxSnapshot, error) {
	body := client.CreateSandboxSnapshotJSONRequestBody{}
	if input.Kind != "" {
		kind := sandboxesclient.SandboxSnapshotKind(input.Kind)
		body.Kind = &kind
	}
	if input.Name != nil {
		body.Name = input.Name
	}
	return s.repo.Create(ctx, input.SandboxID, body)
}

func (s *Service) Get(ctx context.Context, sandboxGroupID, snapshotID string) (*sandboxesclient.SandboxSnapshot, error) {
	sandboxGroupID, err := s.resolveGroupID(ctx, sandboxGroupID)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, sandboxGroupID, snapshotID)
}

func (s *Service) List(ctx context.Context, input ListInput) ([]*sandboxesclient.SandboxSnapshot, error) {
	sandboxGroupID, err := s.resolveGroupID(ctx, input.SandboxGroupID)
	if err != nil {
		return nil, err
	}
	statuses := make([]sandboxesclient.SandboxSnapshotStatus, len(input.Statuses))
	for i, status := range input.Statuses {
		statuses[i] = sandboxesclient.SandboxSnapshotStatus(status)
	}
	return s.repo.ListForGroup(ctx, sandboxGroupID, statuses)
}

func (s *Service) resolveGroupID(ctx context.Context, sandboxGroupID string) (string, error) {
	return s.groups.IDOrDefault(ctx, sandboxGroupID)
}

func (s *Service) Delete(ctx context.Context, sandboxGroupID, snapshotID string) error {
	return s.repo.Delete(ctx, sandboxGroupID, snapshotID)
}
