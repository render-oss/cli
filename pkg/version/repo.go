package version

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	wfclient "github.com/render-oss/cli/pkg/client/workflows"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

func (r *Repo) ListVersions(ctx context.Context, workflowID string, params *client.ListWorkflowVersionsParams) (client.Cursor, []*wfclient.WorkflowVersion, error) {
	resp, err := r.client.ListWorkflowVersionsWithResponse(ctx, params)
	if err != nil {
		return "", nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return "", nil, err
	}

	result := make([]*wfclient.WorkflowVersion, 0, len(*resp.JSON200))
	for _, version := range *resp.JSON200 {
		result = append(result, &version.WorkflowVersion)
	}

	var cursor client.Cursor
	if len(*resp.JSON200) > 0 {
		cursor = (*resp.JSON200)[len(*resp.JSON200)-1].Cursor
	}

	return cursor, result, nil
}

func (r *Repo) GetVersion(ctx context.Context, workflowVersionID string) (*wfclient.WorkflowVersion, error) {
	resp, err := r.client.GetWorkflowVersionWithResponse(ctx, workflowVersionID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

type TriggerReleaseInput struct {
	CommitId *string
}

func (d *Repo) TriggerRelease(ctx context.Context, workflowID string, input TriggerReleaseInput) error {
	resp, err := d.client.DeployWorkflowWithResponse(ctx, workflowID)
	if err != nil {
		return err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return err
	}

	return nil
}
