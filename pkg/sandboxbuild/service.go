package sandboxbuild

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
	SandboxGroupID   string
	Name             string
	SourceImage      string
	RegistryUsername string
	RegistryPassword string
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*sandboxesclient.SandboxBuild, error) {
	sandboxGroupID, err := s.groups.IDOrDefault(ctx, input.SandboxGroupID)
	if err != nil {
		return nil, err
	}
	body := client.CreateSandboxBuildJSONRequestBody{Name: input.Name, SourceImage: input.SourceImage}
	if input.RegistryUsername != "" || input.RegistryPassword != "" {
		body.RegistryCredential = &struct {
			Password string `json:"password"`
			Username string `json:"username"`
		}{Password: input.RegistryPassword, Username: input.RegistryUsername}
	}
	return s.repo.Create(ctx, sandboxGroupID, body)
}

func (s *Service) Get(ctx context.Context, sandboxGroupID, buildID string) (*sandboxesclient.SandboxBuild, error) {
	sandboxGroupID, err := s.groups.IDOrDefault(ctx, sandboxGroupID)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, sandboxGroupID, buildID)
}
