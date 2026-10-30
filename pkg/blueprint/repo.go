package blueprint

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	bptypes "github.com/render-oss/cli/pkg/client/blueprints"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

// Create returns the API result, including a saved Blueprint when planning fails.
// The request must include the workspace's OwnerId.
func (r *Repo) Create(ctx context.Context, body bptypes.CreateBlueprintRequest) (*bptypes.CreateBlueprintResponse, error) {
	resp, err := r.client.CreateBlueprintWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create blueprint: response missing creation result (status %d)", resp.StatusCode())
	}
	return resp.JSON201, nil
}

// Approve asks the API to approve the Blueprint's newest sync and returns the
// sync from its response.
func (r *Repo) Approve(ctx context.Context, blueprintID string, body bptypes.ApproveBlueprintRequest) (*bptypes.ApprovedSync, error) {
	resp, err := r.client.ApproveBlueprintWithResponse(ctx, blueprintID, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("approve blueprint: response missing sync (status %d)", resp.StatusCode())
	}
	return resp.JSON200, nil
}
