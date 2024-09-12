package deploys

import (
	"fmt"
	"net/http"

	"github.com/renderinc/render-cli/pkg/client"
)

func NewDeployRepo(client *http.Client, server string, token string) *DeployRepo {
	return &DeployRepo{server: server, client: client, token: token}
}

type DeployRepo struct {
	server string
	client *http.Client
	token  string
}

func (d *DeployRepo) ListDeploysForService(serviceID string) ([]*client.Deploy, error) {
	req, err := client.NewListDeploysRequest(d.server, serviceID, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("authorization", fmt.Sprintf("Bearer %s", d.token))

	res, err := d.client.Do(req)

	deployResponse, err := client.ParseListDeploysResponse(res)
	if err != nil {
		return nil, err
	}

	if deployResponse.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", deployResponse.Status())
	}

	result := make([]*client.Deploy, 0, len(*deployResponse.JSON200))
	for _, deploy := range *deployResponse.JSON200 {
		result = append(result, deploy.Deploy)
	}

	return result, nil
}
