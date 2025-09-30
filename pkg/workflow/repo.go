package workflow

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
	"github.com/render-oss/cli/pkg/config"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{
		client: c,
	}
}

func (r *Repo) ListWorkflows(ctx context.Context, params *client.ListWorkflowsParams) ([]*wfclient.Workflow, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	params.OwnerId = &client.OwnerIdParam{workspace}

	resp, err := r.client.ListWorkflowsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	workflows := make([]*wfclient.Workflow, 0, len(*resp.JSON200))
	for _, workflow := range *resp.JSON200 {
		workflows = append(workflows, &workflow)
	}

	return workflows, nil
}

func (r *Repo) GetWorkflow(ctx context.Context, id string) (*wfclient.Workflow, error) {
	resp, err := r.client.GetWorkflowWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}
