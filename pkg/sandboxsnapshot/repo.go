package sandboxsnapshot

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

func (r *Repo) Create(ctx context.Context, sandboxID string, body client.CreateSandboxSnapshotJSONRequestBody) (*sandboxesclient.SandboxSnapshot, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.CreateSandboxSnapshotWithResponse(ctx, sandboxID, &client.CreateSandboxSnapshotParams{OwnerId: &workspace}, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON202 == nil {
		return nil, fmt.Errorf("create sandbox snapshot: success response missing snapshot body")
	}
	return resp.JSON202, nil
}
