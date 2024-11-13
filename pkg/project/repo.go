package project

import (
	"context"
	"fmt"

	"github.com/renderinc/cli/pkg/client"
	"github.com/renderinc/cli/pkg/config"
	"github.com/renderinc/cli/pkg/pointers"
)

func NewRepo(client *client.ClientWithResponses) *Repo {
	return &Repo{client: client}
}

type Repo struct {
	client *client.ClientWithResponses
}

func (p *Repo) ListProjects(ctx context.Context) ([]*client.Project, error) {
	params := &client.ListProjectsParams{}

	workspaceId, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	if workspaceId != "" {
		params.OwnerId = pointers.From([]string{workspaceId})
	}

	return client.ListAll(ctx, params, p.listPage)
}

func (p *Repo) listPage(ctx context.Context, params *client.ListProjectsParams) ([]*client.Project, *client.Cursor, error) {
	resp, err := p.client.ListProjectsWithResponse(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, nil, err
	}
	if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
		return nil, nil, nil
	}

	res := *resp.JSON200
	projects := make([]*client.Project, 0, len(*resp.JSON200))
	for _, projectWithCursor := range *resp.JSON200 {
		projects = append(projects, &projectWithCursor.Project)
	}

	return projects, &res[len(res)-1].Cursor, nil
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
