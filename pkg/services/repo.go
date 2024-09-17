package services

import (
	"fmt"
	"net/http"

	"github.com/renderinc/render-cli/pkg/client"
)

func NewServiceRepo(client *http.Client, server string, token string) *ServiceRepo {
	return &ServiceRepo{server: server, client: client, token: token}
}

type ServiceRepo struct {
	server string
	client *http.Client
	token  string
}

func (s *ServiceRepo) ListServices() ([]*client.Service, error) {
	req, err := client.NewListServicesRequest(s.server, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.makeRequest(req)
	if err != nil {
		return nil, err
	}

	serviceResponse, err := client.ParseListServicesResponse(resp)
	if err != nil {
		return nil, err
	}

	if serviceResponse.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", serviceResponse.Status())
	}

	result := make([]*client.Service, 0, len(*serviceResponse.JSON200))
	for _, deploy := range *serviceResponse.JSON200 {
		result = append(result, deploy.Service)
	}

	return result, nil
}

func (s *ServiceRepo) DeployService(svc *client.Service) (*client.Deploy, error) {
	req, err := client.NewCreateDeployRequest(
		s.server,
		svc.Id,
		client.CreateDeployJSONRequestBody{
			ClearCache: nil,
			CommitId:   nil,
			ImageUrl:   nil,
		},
	)
	if err != nil {
		return nil, err
	}

	resp, err := s.makeRequest(req)
	if err != nil {
		return nil, err
	}

	deployResponse, err := client.ParseCreateDeployResponse(resp)
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
