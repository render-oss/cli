package owner

import (
	"context"
	"fmt"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/pointers"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(client *client.ClientWithResponses) *Repo {
	return &Repo{client: client}
}

func (r *Repo) ListOwners(ctx context.Context) ([]*client.Owner, error) {
	resp, err := r.client.ListOwnersWithResponse(ctx, &client.ListOwnersParams{Limit: pointers.From(100)})
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	
	var owners []*client.Owner
	for _, ownerWithCursor := range *resp.JSON200 {
		owners = append(owners, ownerWithCursor.Owner)
	}

	return owners, nil
}

func (r *Repo) RetrieveOwner(ctx context.Context, id string) (*client.Owner, error) {
	resp, err := r.client.RetrieveOwnerWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", resp.Status())
	}

	return resp.JSON200, nil
}
