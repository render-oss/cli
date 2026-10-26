package sandboxgroup

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

// Repo wraps the generated OpenAPI client for sandbox-groups endpoints. Higher
// layers should call through Service; Repo is exposed only so tests can
// exercise the HTTP path in isolation.
type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

// List returns all sandbox groups owned by ownerID. Omitting pagination keeps
// this compatible with servers that do not support group pagination.
func (r *Repo) List(ctx context.Context, ownerID string) ([]*sandboxesclient.SandboxGroup, error) {
	params := &client.ListSandboxGroupsParams{
		OwnerId: ownerID,
	}

	resp, err := r.client.ListSandboxGroupsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return []*sandboxesclient.SandboxGroup{}, nil
	}
	out := make([]*sandboxesclient.SandboxGroup, 0, len(*resp.JSON200))
	for _, gwc := range *resp.JSON200 {
		g := gwc.SandboxGroup
		out = append(out, &g)
	}
	return out, nil
}
