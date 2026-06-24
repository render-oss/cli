package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

func (r *Repo) ListSandboxes(ctx context.Context, params *client.ListSandboxesParams) ([]*sandboxclient.Sandbox, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	params.OwnerId = &client.OwnerIdParam{workspace}

	resp, err := r.client.ListSandboxesWithResponse(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	sandboxes := make([]*sandboxclient.Sandbox, 0, len(*resp.JSON200))
	for _, swc := range *resp.JSON200 {
		sb := swc.Sandbox
		sandboxes = append(sandboxes, &sb)
	}

	return sandboxes, nil
}

func (r *Repo) CreateSandbox(
	ctx context.Context,
	body client.CreateSandboxJSONRequestBody,
	onEvent func(*sandboxclient.Sandbox),
) (*sandboxclient.Sandbox, error) {
	resp, err := r.client.CreateSandboxWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	if resp.JSON201 == nil {
		return nil, fmt.Errorf("create sandbox: success response missing sandbox body")
	}

	if onEvent != nil {
		onEvent(resp.JSON201)
	}

	return resp.JSON201, nil
}

func (r *Repo) GetSandbox(ctx context.Context, id string) (*sandboxclient.Sandbox, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.RetrieveSandboxWithResponse(ctx, id, &client.RetrieveSandboxParams{OwnerId: workspace})
	if err != nil {
		return nil, err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}

	return resp.JSON200, nil
}

func (r *Repo) ExecSandboxStream(ctx context.Context, id string, command string, onOutput func(*ExecOutputEvent) error) (int, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return 0, err
	}

	resp, err := r.client.ExecSandboxSync(ctx, id,
		&client.ExecSandboxSyncParams{OwnerId: workspace},
		client.ExecSandboxSyncJSONRequestBody{Command: command},
		func(_ context.Context, req *http.Request) error {
			req.Header.Set("Accept", "text/event-stream")
			return nil
		},
	)
	if err != nil {
		return 0, err
	}
	if resp == nil || resp.Body == nil {
		return 0, fmt.Errorf("exec sandbox stream: success response missing body")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, errFromStreamResponse(resp)
	}

	return readSandboxExecStream(resp.Body, onOutput)
}

// errFromStreamResponse parses an error out of a raw streaming response body.
// client.ErrorFromResponse reflects over the generated *WithResponse structs and
// can't be used on the raw *http.Response returned by ExecSandboxSync, so this
// mirrors its behavior: map the standard auth codes to shared sentinels and
// surface the API's structured error message where present.
func errFromStreamResponse(resp *http.Response) error {
	if resp.StatusCode == http.StatusUnauthorized {
		return client.ErrUnauthorized
	}
	if resp.StatusCode == http.StatusForbidden {
		return client.ErrForbidden
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("received response code %d", resp.StatusCode)
	}

	var apiErr client.Error
	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Message != nil && *apiErr.Message != "" {
		return fmt.Errorf("received response code %d: %s", resp.StatusCode, *apiErr.Message)
	}

	if len(body) > 0 {
		return fmt.Errorf("received response code %d: %s", resp.StatusCode, body)
	}
	return fmt.Errorf("received response code %d", resp.StatusCode)
}

func (r *Repo) TerminateSandbox(ctx context.Context, id string) error {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return err
	}

	resp, err := r.client.TerminateSandboxWithResponse(ctx, id, &client.TerminateSandboxParams{OwnerId: workspace})
	if err != nil {
		return err
	}

	if err := client.ErrorFromResponse(resp); err != nil {
		return err
	}

	return nil
}
