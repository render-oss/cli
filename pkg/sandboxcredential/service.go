package sandboxcredential

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
	SandboxGroupID string
	CredentialID   string
	Domain         string
	Value          string
	Enabled        *bool
	Methods        []string
	Paths          []string
	Injection      sandboxesclient.SandboxCredentialInjection
}

type UpdateInput struct {
	SandboxGroupID string
	CredentialID   string
	Domain         *string
	Value          *string
	Enabled        *bool
	Methods        *[]string
	Paths          *[]string
	Injection      *sandboxesclient.SandboxCredentialInjection
}

func (s *Service) List(ctx context.Context, groupID string) ([]sandboxesclient.SandboxCredential, error) {
	groupID, err := s.groups.IDOrDefault(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, groupID)
}

func (s *Service) Get(ctx context.Context, groupID, id string) (*sandboxesclient.SandboxCredential, error) {
	groupID, err := s.groups.IDOrDefault(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, groupID, id)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*sandboxesclient.SandboxCredential, error) {
	groupID, err := s.groups.IDOrDefault(ctx, input.SandboxGroupID)
	if err != nil {
		return nil, err
	}
	body := client.CreateSandboxGroupCredentialJSONRequestBody{
		Id:        input.CredentialID,
		Domain:    input.Domain,
		Value:     input.Value,
		Enabled:   input.Enabled,
		Injection: input.Injection,
	}
	if input.Methods != nil {
		body.Methods = &input.Methods
	}
	if input.Paths != nil {
		body.Paths = &input.Paths
	}
	return s.repo.Create(ctx, groupID, body)
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (*sandboxesclient.SandboxCredential, error) {
	groupID, err := s.groups.IDOrDefault(ctx, input.SandboxGroupID)
	if err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, groupID, input.CredentialID, client.UpdateSandboxGroupCredentialJSONRequestBody{
		Domain:    input.Domain,
		Value:     input.Value,
		Enabled:   input.Enabled,
		Methods:   input.Methods,
		Paths:     input.Paths,
		Injection: input.Injection,
	})
}

func (s *Service) Delete(ctx context.Context, groupID, id string) error {
	groupID, err := s.groups.IDOrDefault(ctx, groupID)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, groupID, id)
}

func (s *Service) Versions(ctx context.Context, groupID, id string) ([]sandboxesclient.SandboxCredentialVersion, error) {
	groupID, err := s.groups.IDOrDefault(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return s.repo.Versions(ctx, groupID, id)
}

func (s *Service) Restore(ctx context.Context, groupID, id string, versionID int64) (*sandboxesclient.SandboxCredential, error) {
	groupID, err := s.groups.IDOrDefault(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return s.repo.Restore(ctx, groupID, id, client.RestoreSandboxGroupCredentialJSONRequestBody{VersionId: versionID})
}
