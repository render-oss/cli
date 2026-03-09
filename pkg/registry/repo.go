package registry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/render-oss/cli/pkg/client"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{
		client: c,
	}
}

func (s *Repo) ListRegistryCredentials(ctx context.Context, params *client.ListRegistryCredentialsParams) (*[]client.RegistryCredential, error) {
	resp, err := s.client.ListRegistryCredentialsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (s *Repo) GetRegistryCredential(ctx context.Context, id string) (*client.RegistryCredential, error) {
	resp, err := s.client.RetrieveRegistryCredentialWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, nil
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("registry credential lookup failed for %q: empty response", id)
	}

	return resp.JSON200, nil
}
