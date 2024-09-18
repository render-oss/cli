package project

import (
	"context"
	"fmt"

	"github.com/renderinc/render-cli/pkg/client"
)

func NewRepo(client *client.ClientWithResponses) *Repo {
	return &Repo{client: client}
}

type Repo struct {
	client *client.ClientWithResponses
}

func (p *Repo) ListProjects(ctx context.Context) ([]*client.Project, error) {
	resp, err := p.client.ListProjectsWithResponse(ctx, nil)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", resp.Status())
	}

	result := make([]*client.Project, 0, len(*resp.JSON200))
	for _, projectWithCursor := range *resp.JSON200 {
		result = append(result, &projectWithCursor.Project)
	}

	return result, nil
}
