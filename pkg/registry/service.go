package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/render-oss/cli/v2/pkg/client"
	"github.com/render-oss/cli/v2/pkg/validate"
)

type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

func (s *Service) FindOneRegistryCredentialByIDFromNameOrID(ctx context.Context, ownerID string, idOrName string) (string, error) {
	matches, err := s.FindRegistryCredentialsByIDOrName(ctx, ownerID, idOrName)
	if err != nil {
		return "", err
	}

	matchCount := 0
	if matches != nil {
		matchCount = len(*matches)
	}
	switch matchCount {
	case 0:
		return "", fmt.Errorf("no registry credential found with ID or name %q in workspace %q (exact match required)", idOrName, ownerID)
	case 1:
		return (*matches)[0].Id, nil
	default:
		return "", fmt.Errorf("multiple registry credentials found with ID or name %q in workspace %q; please use the credential ID", idOrName, ownerID)
	}
}

func (s *Service) FindRegistryCredentialsByIDOrName(ctx context.Context, ownerID string, idOrName string) (*[]client.RegistryCredential, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	query := strings.TrimSpace(idOrName)
	if query == "" {
		return nil, fmt.Errorf("registry credential ID or name is required")
	}
	ownerFilter := client.OwnerIdParam{ownerID}
	if looksLikeRegistryCredentialID(query) {
		return s.findRegistryCredentialsByID(ctx, query)
	}

	return s.findRegistryCredentialsByName(ctx, ownerFilter, query)
}

func looksLikeRegistryCredentialID(value string) bool {
	return validate.IsObjectID("rgc", value)
}

func (s *Service) findRegistryCredentialsByName(ctx context.Context, ownerFilter client.OwnerIdParam, query string) (*[]client.RegistryCredential, error) {
	nameFilter := []string{query}
	credentials, err := s.repo.ListRegistryCredentials(ctx, &client.ListRegistryCredentialsParams{
		OwnerId: &ownerFilter,
		Name:    &nameFilter,
	})
	if err != nil {
		return nil, err
	}

	if credentials == nil {
		return nil, nil
	}

	matches := make([]client.RegistryCredential, 0)
	for i := range *credentials {
		if (*credentials)[i].Name == query {
			matches = append(matches, (*credentials)[i])
		}
	}

	return &matches, nil
}

func (s *Service) findRegistryCredentialsByID(ctx context.Context, query string) (*[]client.RegistryCredential, error) {
	credential, err := s.repo.GetRegistryCredential(ctx, query)
	if err != nil {
		return nil, err
	}
	if credential == nil {
		none := []client.RegistryCredential{}
		return &none, nil
	}

	match := []client.RegistryCredential{*credential}
	return &match, nil
}
