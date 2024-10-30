package project

import (
	"context"
	"fmt"

	"github.com/renderinc/render-cli/pkg/client"
	"github.com/renderinc/render-cli/pkg/config"
	"github.com/renderinc/render-cli/pkg/pointers"
)

func NewRepo(client *client.ClientWithResponses) *Repo {
	return &Repo{client: client}
}

type Repo struct {
	client *client.ClientWithResponses
}

func (p *Repo) ListProjects(ctx context.Context) ([]*client.Project, error) {
	params := &client.ListProjectsParams{
		Limit: pointers.From(100),
	}

	workspaceId, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	if workspaceId != "" {
		params.OwnerId = pointers.From([]string{workspaceId})
	}

	resp, err := p.client.ListProjectsWithResponse(ctx, params)
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

func (p *Repo) GetProject(ctx context.Context, id string) (*client.Project, error) {
	resp, err := p.client.RetrieveProjectWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("unexpected response: %v", resp.Status())
	}

	return resp.JSON200, nil
}
