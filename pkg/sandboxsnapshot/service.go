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

func (s *Service) Create(ctx context.Context, input CreateInput) (*sandboxesclient.SandboxSnapshot, error) {
	body := client.CreateSandboxSnapshotJSONRequestBody{}
	if input.Kind != "" {
		kind := sandboxesclient.SandboxSnapshotKind(input.Kind)
		body.Kind = &kind
	}
	return s.repo.Create(ctx, input.SandboxID, body)
}

func (s *Service) Get(ctx context.Context, sandboxGroupID, snapshotID string) (*sandboxesclient.SandboxSnapshot, error) {
	if sandboxGroupID == "" {
		workspace, err := config.WorkspaceID()
		if err != nil {
			return nil, err
		}
		groups, err := s.groupRepo.List(ctx, workspace)
		if err != nil {
			return nil, err
		}
		for _, group := range groups {
			if group.IsDefault {
				sandboxGroupID = group.Id
				break
			}
		}
		if sandboxGroupID == "" {
			return nil, fmt.Errorf("no default sandbox group found in the active workspace; specify --group")
		}
	}
	return s.repo.Get(ctx, sandboxGroupID, snapshotID)
}
