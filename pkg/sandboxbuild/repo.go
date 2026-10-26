package sandboxbuild

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

func (r *Repo) Create(ctx context.Context, sandboxGroupID string, body client.CreateSandboxBuildJSONRequestBody) (*sandboxesclient.SandboxBuild, error) {
	resp, err := r.client.CreateSandboxBuildWithResponse(ctx, sandboxGroupID, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create sandbox build: success response missing build body")
	}
	return resp.JSON201, nil
}

func (r *Repo) Get(ctx context.Context, sandboxGroupID, buildID string) (*sandboxesclient.SandboxBuild, error) {
	resp, err := r.client.RetrieveSandboxBuildWithResponse(ctx, sandboxGroupID, buildID)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get sandbox build: success response missing build body")
	}
	return resp.JSON200, nil
}
