package sandboximage

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

type ImportInput struct {
	SandboxGroupID   string
	Name             string
	Source           string
	RegistryUsername string
	RegistryPassword string
}

func (s *Service) Import(ctx context.Context, input ImportInput) (*sandboxesclient.SandboxBuild, error) {
	sandboxGroupID, err := s.groups.IDOrDefault(ctx, input.SandboxGroupID)
	if err != nil {
		return nil, err
	}
	body := client.CreateSandboxBuildJSONRequestBody{Name: input.Name, SourceImage: input.Source}
	if input.RegistryUsername != "" || input.RegistryPassword != "" {
		body.RegistryCredential = &struct {
			Password string `json:"password"`
			Username string `json:"username"`
		}{Password: input.RegistryPassword, Username: input.RegistryUsername}
	}
	return s.repo.Import(ctx, sandboxGroupID, body)
}

func (s *Service) Get(ctx context.Context, sandboxGroupID, imageID string) (*sandboxesclient.SandboxBuild, error) {
	sandboxGroupID, err := s.groups.IDOrDefault(ctx, sandboxGroupID)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, sandboxGroupID, imageID)
}

type ListInput struct {
	SandboxGroupID string
	Name           string
	Statuses       []string
}

func (s *Service) List(ctx context.Context, input ListInput) ([]*sandboxesclient.SandboxBuild, error) {
	sandboxGroupID, err := s.groups.IDOrDefault(ctx, input.SandboxGroupID)
	if err != nil {
		return nil, err
	}
	params := &client.ListSandboxBuildsParams{}
	if input.Name != "" {
		params.Name = &input.Name
	}
	if len(input.Statuses) > 0 {
		statuses := make([]sandboxesclient.SandboxBuildStatus, len(input.Statuses))
		for i, status := range input.Statuses {
			statuses[i] = sandboxesclient.SandboxBuildStatus(status)
		}
		params.Status = &statuses
	}
	return s.repo.ListForGroup(ctx, sandboxGroupID, params)
}
