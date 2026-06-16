package sandbox

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

func (r *Repo) ListSandboxes(ctx context.Context, params *client.ListSandboxesParams) ([]*sandboxclient.Sandbox, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	params.OwnerId = &client.OwnerIdParam{workspace}

	resp, err := r.client.ListSandboxesWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	sandboxes := make([]*sandboxclient.Sandbox, 0, len(*resp.JSON200))
	for _, swc := range *resp.JSON200 {
		sb := swc.Sandbox
		sandboxes = append(sandboxes, &sb)
	}

	return sandboxes, nil
}

func (r *Repo) CreateSandbox(
	ctx context.Context,
	body client.CreateSandboxJSONRequestBody,
	onEvent func(*sandboxclient.Sandbox),
) (*sandboxclient.Sandbox, error) {
	resp, err := r.client.CreateSandboxWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create sandbox: success response missing sandbox body")
	}

	if onEvent != nil {
		onEvent(resp.JSON201)
	}

	return resp.JSON201, nil
}

func (r *Repo) GetSandbox(ctx context.Context, id string) (*sandboxclient.Sandbox, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.RetrieveSandboxWithResponse(ctx, id, &client.RetrieveSandboxParams{OwnerId: workspace})
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) ExecSandbox(ctx context.Context, id string, command string) (*sandboxclient.SandboxExecSyncResponse, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.ExecSandboxSyncWithResponse(ctx, id,
		&client.ExecSandboxSyncParams{OwnerId: workspace},
		client.ExecSandboxSyncJSONRequestBody{Command: command},
	)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("exec sandbox: success response missing body")
	}

	return resp.JSON200, nil
}

func (r *Repo) TerminateSandbox(ctx context.Context, id string) error {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return err
	}

	resp, err := r.client.TerminateSandboxWithResponse(ctx, id, &client.TerminateSandboxParams{OwnerId: workspace})
	if err != nil {
		return err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return err
	}

	return nil
}
