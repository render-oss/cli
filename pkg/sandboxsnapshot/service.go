package sandboxsnapshot

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
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
