package services

import (
	"context"
	"fmt"

	"github.com/renderinc/render-cli/pkg/client"
)

func NewServiceRepo(c *client.ClientWithResponses) *ServiceRepo {
	return &ServiceRepo{c: c}
}

type ServiceRepo struct {
	c *client.ClientWithResponses
}

func (s *ServiceRepo) ListServices(ctx context.Context) ([]*client.Service, error) {
	services, err := s.c.ListServicesWithResponse(ctx, nil)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(services); err != nil {
		return nil, err
	}

	result := make([]*client.Service, 0, len(*services.JSON200))
	for _, deploy := range *services.JSON200 {
		result = append(result, deploy.Service)
	}

	return result, nil
}

func (s *ServiceRepo) DeployService(ctx context.Context, svc *client.Service) (*client.Deploy, error) {
	deployResponse, err := s.c.CreateDeployWithResponse(ctx, svc.Id, client.CreateDeployJSONRequestBody{
		ClearCache: nil,
		CommitId:   nil,
		ImageUrl:   nil,
	},
	)
	if err != nil {
		return nil, err
	}

	if deployResponse.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %v", deployResponse.Status())
	}

	return deployResponse.JSON201, nil
}

func (s *ServiceRepo) CreateService(ctx context.Context, data client.CreateServiceJSONRequestBody) (*client.Service, error) {
	serviceResponse, err := s.c.CreateServiceWithResponse(ctx, data)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(serviceResponse); err != nil {
		return nil, err
	}

	return serviceResponse.JSON201.Service, nil
}

func (s *ServiceRepo) UpdateService(ctx context.Context, id string, data client.UpdateServiceJSONRequestBody) (*client.Service, error) {
	serviceResponse, err := s.c.UpdateServiceWithResponse(ctx, id, data)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(serviceResponse); err != nil {
		return nil, err
	}

	return serviceResponse.JSON200, nil
}

func (s *ServiceRepo) GetService(ctx context.Context, id string) (*client.Service, error) {
	serviceResponse, err := s.c.RetrieveServiceWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(serviceResponse); err != nil {
		return nil, err
	}

	return serviceResponse.JSON200, nil
}
