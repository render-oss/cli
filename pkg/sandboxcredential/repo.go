// Package sandboxcredential manages brokered credentials for sandbox groups.
package sandboxcredential

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

func (r *Repo) List(ctx context.Context, groupID string) ([]sandboxesclient.SandboxCredential, error) {
	resp, err := r.client.ListSandboxGroupCredentialsWithResponse(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("list sandbox credentials: success response missing body")
	}
	credentials := *resp.JSON200
	if credentials == nil {
		credentials = []sandboxesclient.SandboxCredential{}
	}
	return credentials, nil
}

func (r *Repo) Get(ctx context.Context, groupID, id string) (*sandboxesclient.SandboxCredential, error) {
	resp, err := r.client.RetrieveSandboxGroupCredentialWithResponse(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get sandbox credential: success response missing body")
	}
	return resp.JSON200, nil
}

func (r *Repo) Create(ctx context.Context, groupID string, body client.CreateSandboxGroupCredentialJSONRequestBody) (*sandboxesclient.SandboxCredential, error) {
	resp, err := r.client.CreateSandboxGroupCredentialWithResponse(ctx, groupID, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create sandbox credential: success response missing body")
	}
	return resp.JSON201, nil
}

func (r *Repo) Update(ctx context.Context, groupID, id string, body client.UpdateSandboxGroupCredentialJSONRequestBody) (*sandboxesclient.SandboxCredential, error) {
	resp, err := r.client.UpdateSandboxGroupCredentialWithResponse(ctx, groupID, id, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("update sandbox credential: success response missing body")
	}
	return resp.JSON200, nil
}

func (r *Repo) Delete(ctx context.Context, groupID, id string) error {
	resp, err := r.client.DeleteSandboxGroupCredentialWithResponse(ctx, groupID, id)
	if err != nil {
		return err
	}
	return client.ErrorFromResponse(resp)
}

func (r *Repo) Versions(ctx context.Context, groupID, id string) ([]sandboxesclient.SandboxCredentialVersion, error) {
	resp, err := r.client.ListSandboxGroupCredentialVersionsWithResponse(ctx, groupID, id)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("list sandbox credential versions: success response missing body")
	}
	versions := *resp.JSON200
	if versions == nil {
		versions = []sandboxesclient.SandboxCredentialVersion{}
	}
	return versions, nil
}

func (r *Repo) Restore(ctx context.Context, groupID, id string, body client.RestoreSandboxGroupCredentialJSONRequestBody) (*sandboxesclient.SandboxCredential, error) {
	resp, err := r.client.RestoreSandboxGroupCredentialWithResponse(ctx, groupID, id, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("restore sandbox credential: success response missing body")
	}
	return resp.JSON200, nil
}
