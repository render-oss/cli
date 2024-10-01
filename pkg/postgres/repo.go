package postgres

import (
	"context"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/config"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{
		client: c,
	}
}

func (r *Repo) ListPostgres(ctx context.Context) ([]*client.Postgres, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.ListPostgresWithResponse(ctx, &client.ListPostgresParams{
		OwnerId: &client.OwnerIdParam{workspace},
	})
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	pgs := make([]*client.Postgres, 0, len(*resp.JSON200))
	for _, pg := range *resp.JSON200 {
		pgs = append(pgs, &pg.Postgres)
	}

	return pgs, nil
}

func (r *Repo) GetPostgres(ctx context.Context, id string) (*client.PostgresDetail, error) {
	resp, err := r.client.RetrievePostgresWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) GetPostgresConnectionInfo(ctx context.Context, id string) (*client.PostgresConnectionInfo, error) {
	resp, err := r.client.RetrievePostgresConnectionInfoWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) RestartPostgresDatabase(ctx context.Context, id string) error {
	resp, err := r.client.RestartPostgres(ctx, id)
	if err != nil {
		return err
	}

	return client.ErrorFromResponse(resp)
}
