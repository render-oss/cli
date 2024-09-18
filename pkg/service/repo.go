package service

import (
	"context"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/pointers"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{
		client: c,
	}
}

func (s *Repo) ListServices(ctx context.Context) ([]*client.Service, error) {
	params := &client.ListServicesParams{Limit: pointers.From(100)}
	resp, err := s.client.ListServicesWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	services := make([]*client.Service, 0, len(*resp.JSON200))
	for _, serviceWithCursor := range *resp.JSON200 {
		services = append(services, serviceWithCursor.Service)
	}

	return services, nil
}

func (s *Repo) DeployService(ctx context.Context, svc *client.Service) (*client.Deploy, error) {
	resp, err := s.client.CreateDeployWithResponse(ctx, svc.Id, client.CreateDeployJSONRequestBody{
		ClearCache: nil,
		CommitId:   nil,
		ImageUrl:   nil,
	})
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON201, nil
}

func (s *Repo) CreateService(ctx context.Context, data client.CreateServiceJSONRequestBody) (*client.Service, error) {
	resp, err := s.client.CreateServiceWithResponse(ctx, data)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON201.Service, nil
}

func (s *Repo) UpdateService(ctx context.Context, id string, data client.UpdateServiceJSONRequestBody) (*client.Service, error) {
	resp, err := s.client.UpdateServiceWithResponse(ctx, id, data)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (s *Repo) GetService(ctx context.Context, id string) (*client.Service, error) {
	resp, err := s.client.RetrieveServiceWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}
