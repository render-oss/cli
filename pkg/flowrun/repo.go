package flowrun

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	workflows "github.com/render-oss/cli/pkg/client/workflows"
)

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

type Repo struct {
	client *client.ClientWithResponses
}

func (r *Repo) GetFlowRun(ctx context.Context, flowRunID string) (*workflows.FlowRunDetails, error) {
	resp, err := r.client.GetFlowRunWithResponse(ctx, flowRunID)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %s", resp.Status())
	}

	return resp.JSON200, nil
}
