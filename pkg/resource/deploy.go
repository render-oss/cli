package resource

import (
	"context"
	"fmt"
	"net/http"

	"github.com/renderinc/render-cli/pkg/client"
)

type DeployListInput struct {
	ServiceID string
}

func DeploysForInput(ctx context.Context, c *client.ClientWithResponses, input *DeployListInput) (*client.DeployList, error) {
	response, err := c.ListDeploysWithResponse(
		ctx,
		input.ServiceID,
		&client.ListDeploysParams{},
	)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode() {
	case http.StatusOK:
		return response.JSON200, nil
	default:
		return nil, fmt.Errorf("unexpected status code %d", response.StatusCode())
	}
}
