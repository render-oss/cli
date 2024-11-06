package redis

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

func (r *Repo) ListRedis(ctx context.Context, params *client.ListRedisParams) ([]*client.Redis, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	params.OwnerId = &client.OwnerIdParam{workspace}

	resp, err := r.client.ListRedisWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	redises := make([]*client.Redis, 0, len(*resp.JSON200))
	for _, redis := range *resp.JSON200 {
		redises = append(redises, &redis.Redis)
	}

	return redises, nil
}

func (r *Repo) GetRedis(ctx context.Context, id string) (*client.RedisDetail, error) {
	resp, err := r.client.RetrieveRedisWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) GetRedisConnectionInfo(ctx context.Context, id string) (*client.RedisConnectionInfo, error) {
	resp, err := r.client.RetrieveRedisConnectionInfoWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}
