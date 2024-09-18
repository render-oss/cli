package environment

import (
	"context"
	"fmt"

	"github.com/renderinc/render-cli/pkg/client"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(client *client.ClientWithResponses) *Repo {
	return &Repo{client: client}
}

func (e *Repo) GetEnvironment(ctx context.Context, id string) (*client.Environment, error) {
	resp, err := e.client.RetrieveEnvironmentWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", resp.Status())
	}

	return resp.JSON200, nil
}
