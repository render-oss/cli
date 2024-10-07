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

// GetEnvironment retrieves an environment by ID.
// Note: We are not checking the workspace here because we currently only call this is from contexts
// where we've pulled the environment ID from a resource that was already checked. If this changes, we should
// fetch the project and check its workspace. For now, we will avoid the extra network call.
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

func (e *Repo) ListEnvironments(ctx context.Context, params *client.ListEnvironmentsParams) ([]*client.Environment, error) {
	resp, err := e.client.ListEnvironmentsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", resp.Status())
	}

	var envs []*client.Environment
	for _, env := range *resp.JSON200 {
		envs = append(envs, &env.Environment)
	}

	return envs, nil
}
