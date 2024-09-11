package resource

import (
	"context"
	"fmt"
	"net/http"

	"github.com/renderinc/render-cli/pkg/client"
)

type ServiceListInput struct {
	Name string
}

func ServicesForInput(ctx context.Context, c *client.ClientWithResponses, input *ServiceListInput) (*client.ServiceList, error) {
	response, err := c.ListServicesWithResponse(ctx, &client.ListServicesParams{Name: &client.NameParam{input.Name}})
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
