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

func (r *Repo) Get(ctx context.Context, sandboxGroupID, snapshotID string) (*sandboxesclient.SandboxSnapshot, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.RetrieveSandboxSnapshotWithResponse(ctx, sandboxGroupID, snapshotID, &client.RetrieveSandboxSnapshotParams{OwnerId: &workspace})
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get sandbox snapshot: success response missing snapshot body")
	}
	return resp.JSON200, nil
}

func (r *Repo) ListForGroup(ctx context.Context, sandboxGroupID string, statuses []sandboxesclient.SandboxSnapshotStatus) ([]*sandboxesclient.SandboxSnapshot, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	params := &client.ListSandboxSnapshotsParams{OwnerId: workspace}
	if len(statuses) > 0 {
		params.Status = &statuses
	}
	return listAll(ctx, params, func(ctx context.Context, params *client.ListSandboxSnapshotsParams) ([]*sandboxesclient.SandboxSnapshot, *client.Cursor, error) {
		resp, err := r.client.ListSandboxSnapshotsWithResponse(ctx, sandboxGroupID, params)
		if err != nil {
			return nil, nil, err
		}
		if err := client.ErrorFromResponse(resp); err != nil {
			return nil, nil, err
		}
		return unwrapPage(resp.JSON200)
	})
}

type pageParams interface {
	SetCursor(cursor *client.Cursor)
	SetLimit(int)
}

// encoding/json writes a nil slice as null; an empty list must print as [].
func listAll[P pageParams](ctx context.Context, params P, listPage func(context.Context, P) ([]*sandboxesclient.SandboxSnapshot, *client.Cursor, error)) ([]*sandboxesclient.SandboxSnapshot, error) {
	snapshots, err := client.ListAll(ctx, params, listPage)
	if err != nil {
		return nil, err
	}
	if snapshots == nil {
		return []*sandboxesclient.SandboxSnapshot{}, nil
	}
	return snapshots, nil
}

func unwrapPage(page *[]client.SandboxSnapshotWithCursor) ([]*sandboxesclient.SandboxSnapshot, *client.Cursor, error) {
	if page == nil || len(*page) == 0 {
		return nil, nil, nil
	}
	items := *page
	snapshots := make([]*sandboxesclient.SandboxSnapshot, 0, len(items))
	for _, item := range items {
		snapshot := item.Snapshot
		snapshots = append(snapshots, &snapshot)
	}
	return snapshots, &items[len(items)-1].Cursor, nil
}

func (r *Repo) Delete(ctx context.Context, sandboxGroupID, snapshotID string) error {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return err
	}

	resp, err := r.client.DeleteSandboxSnapshotWithResponse(ctx, sandboxGroupID, snapshotID, &client.DeleteSandboxSnapshotParams{OwnerId: &workspace})
	if err != nil {
		return err
	}
	return client.ErrorFromResponse(resp)
}
