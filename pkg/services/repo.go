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

	if services.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", services.Status())
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

func (s *ServiceRepo) makeRequest(req *http.Request) (*http.Response, error) {
	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", s.token))
	return s.client.Do(req)
}

func (s *ServiceRepo) CreateService(data client.CreateServiceJSONRequestBody) (*client.Service, error) {
	req, err := client.NewCreateServiceRequest(s.server, data)
	if err != nil {
		return nil, err
	}

	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", s.token))

	res, err := s.client.Do(req)

	serviceResponse, err := client.ParseCreateServiceResponse(res)
	if err != nil {
		return nil, err
	}

	if serviceResponse.JSON201 == nil {
		return nil, fmt.Errorf("unexpected response: %v, %s", serviceResponse.Status(), *serviceResponse.JSON400.Message)
	}

	return serviceResponse.JSON201.Service, nil
}

func (s *ServiceRepo) UpdateService(id string, data client.UpdateServiceJSONRequestBody) (*client.Service, error) {
	req, err := client.NewUpdateServiceRequest(s.server, id, data)
	if err != nil {
		return nil, err
	}

	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", s.token))

	res, err := s.client.Do(req)

	serviceResponse, err := client.ParseUpdateServiceResponse(res)
	if err != nil {
		return nil, err
	}

	if serviceResponse.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", serviceResponse.Status())
	}

	return serviceResponse.JSON200, nil
}

func (s *ServiceRepo) GetService(id string) (*client.Service, error) {
	req, err := client.NewRetrieveServiceRequest(s.server, id)
	if err != nil {
		return nil, err
	}

	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", s.token))

	res, err := s.client.Do(req)

	serviceResponse, err := client.ParseRetrieveServiceResponse(res)
	if err != nil {
		return nil, err
	}

	if serviceResponse.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", serviceResponse.Status())
	}

	return serviceResponse.JSON200, nil
}
