// Package sandboximage manages the images sandboxes start from. The API still
// calls them builds, in its paths, types, and error codes.
package sandboximage

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

func (r *Repo) Import(ctx context.Context, sandboxGroupID string, body client.CreateSandboxBuildJSONRequestBody) (*sandboxesclient.SandboxBuild, error) {
	resp, err := r.client.CreateSandboxBuildWithResponse(ctx, sandboxGroupID, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("import sandbox image: success response missing image body")
	}
	return resp.JSON201, nil
}

func (r *Repo) Get(ctx context.Context, sandboxGroupID, imageID string) (*sandboxesclient.SandboxBuild, error) {
	resp, err := r.client.RetrieveSandboxBuildWithResponse(ctx, sandboxGroupID, imageID)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get sandbox image: success response missing image body")
	}
	return resp.JSON200, nil
}

// ListForGroup returns every image in the group matching the filters, newest
// first, following cursors until the last page.
func (r *Repo) ListForGroup(ctx context.Context, sandboxGroupID string, params *client.ListSandboxBuildsParams) ([]*sandboxesclient.SandboxBuild, error) {
	images, err := client.ListAll(ctx, params, func(ctx context.Context, params *client.ListSandboxBuildsParams) ([]*sandboxesclient.SandboxBuild, *client.Cursor, error) {
		resp, err := r.client.ListSandboxBuildsWithResponse(ctx, sandboxGroupID, params)
		if err != nil {
			return nil, nil, err
		}
		if err := client.ErrorFromResponse(resp); err != nil {
			return nil, nil, err
		}
		if resp.JSON200 == nil || len(*resp.JSON200) == 0 {
			return nil, nil, nil
		}
		items := *resp.JSON200
		page := make([]*sandboxesclient.SandboxBuild, 0, len(items))
		for _, item := range items {
			image := item.Build
			page = append(page, &image)
		}
		return page, &items[len(items)-1].Cursor, nil
	})
	if err != nil {
		return nil, err
	}
	// encoding/json writes a nil slice as null; an empty list must print as [].
	if images == nil {
		return []*sandboxesclient.SandboxBuild{}, nil
	}
	return images, nil
}
