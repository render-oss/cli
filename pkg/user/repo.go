package user

import (
	"context"

	"github.com/renderinc/cli/pkg/client"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(client *client.ClientWithResponses) *Repo {
	return &Repo{client: client}
}

func (r *Repo) CurrentUser(ctx context.Context) (*client.User, error) {
	resp, err := r.client.GetUserWithResponse(ctx)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}
