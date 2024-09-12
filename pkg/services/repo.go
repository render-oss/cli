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

	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", s.token))

	res, err := s.client.Do(req)

	deployResponse, err := client.ParseListServicesResponse(res)
	if err != nil {
		return nil, err
	}

	if deployResponse.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", deployResponse.Status())
	}

	result := make([]*client.Service, 0, len(*deployResponse.JSON200))
	for _, deploy := range *deployResponse.JSON200 {
		result = append(result, deploy.Service)
	}

	return result, nil
}
