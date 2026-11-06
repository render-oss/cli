package sandboxgroup

import (
	"context"
	"errors"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/pointers"
)

// Repo wraps the generated OpenAPI client for sandbox-groups endpoints. Higher
// layers should call through Service; Repo is exposed only so tests can
// exercise the HTTP path in isolation.
type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

// List returns all sandbox groups owned by ownerID. Omitting pagination keeps
// this compatible with servers that do not support group pagination.
func (r *Repo) List(ctx context.Context, ownerID string) ([]*sandboxesclient.SandboxGroup, error) {
	params := &client.ListSandboxGroupsParams{
		OwnerId: ownerID,
	}

	resp, err := r.client.ListSandboxGroupsWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON200 == nil {
		return []*sandboxesclient.SandboxGroup{}, nil
	}
	out := make([]*sandboxesclient.SandboxGroup, 0, len(*resp.JSON200))
	for _, gwc := range *resp.JSON200 {
		g := gwc.SandboxGroup
		out = append(out, &g)
	}
	return out, nil
}

// Create creates a sandbox group. The API returns 409 when the name is taken
// and 400 when the workspace is at its group limit.
func (r *Repo) Create(ctx context.Context, body sandboxesclient.SandboxGroupPOST) (*sandboxesclient.SandboxGroup, error) {
	resp, err := r.client.CreateSandboxGroupWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create sandbox group: success response missing group body")
	}
	return resp.JSON201, nil
}

const (
	errorCodeDefaultSandboxGroupNotDeletable = "default_sandbox_group_not_deletable"
	errorCodeSandboxGroupSuspended           = "sandbox_group_suspended"
)

var (
	ErrDefaultGroupNotDeletable = errors.New(
		"the workspace's default sandbox group cannot be deleted; make another group the default first")
	ErrSuspendedGroupNotDeletable = errors.New("the sandbox group is suspended and cannot be deleted")
)

func (r *Repo) Get(ctx context.Context, id string) (*sandboxesclient.SandboxGroup, error) {
	resp, err := r.client.RetrieveSandboxGroupWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("get sandbox group: success response missing group body")
	}
	return resp.JSON200, nil
}

func (r *Repo) Delete(ctx context.Context, id string) error {
	resp, err := r.client.DeleteSandboxGroupWithResponse(ctx, id)
	if err != nil {
		return err
	}
	if resp.JSON409 == nil {
		return client.ErrorFromResponse(resp)
	}
	switch pointers.StringValue(resp.JSON409.Code) {
	case errorCodeDefaultSandboxGroupNotDeletable:
		return ErrDefaultGroupNotDeletable
	case errorCodeSandboxGroupSuspended:
		return ErrSuspendedGroupNotDeletable
	default:
		return client.ErrorFromResponse(resp)
	}
}
