package resource

import (
	"context"
	"fmt"

	"github.com/renderinc/render-cli/pkg/client"
)

func Rollback(ctx context.Context, c *client.ClientWithResponses, serviceID, deployID string) (*client.Deploy, error) {
	resp, err := c.RollbackDeployWithResponse(ctx, serviceID, client.RollbackDeployJSONRequestBody{DeployId: deployID})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 201 {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode())
	}

	return resp.JSON201, nil
}
