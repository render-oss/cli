package sandboxsnapshot

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
	"github.com/render-oss/cli/pkg/sandboxgroup"
)

type Service struct {
	repo      *Repo
	groupRepo *sandboxgroup.Repo
}

func NewService(repo *Repo, groupRepo *sandboxgroup.Repo) *Service {
	return &Service{repo: repo, groupRepo: groupRepo}
}

type CreateInput struct {
	SandboxID string
	Kind      string
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
	if sandboxGroupID != "" {
		return sandboxGroupID, nil
	}
	workspace, err := config.WorkspaceID()
	if err != nil {
		return "", err
	}
	groups, err := s.groupRepo.List(ctx, workspace)
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

func (s *Service) Delete(ctx context.Context, sandboxGroupID, snapshotID string) error {
	return s.repo.Delete(ctx, sandboxGroupID, snapshotID)
}
