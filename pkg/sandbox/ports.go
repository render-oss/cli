package sandbox

import (
	"context"
	"fmt"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
)

// HiddenPort is a port that is no longer exposed.
type HiddenPort struct {
	SandboxID string `json:"sandboxId"`
	Port      int    `json:"port"`
}

// ExposePort makes a sandbox port reachable at a public URL. Exposing a port
// that is already exposed returns it unchanged.
func (s *Service) ExposePort(
	ctx context.Context,
	sandboxID string,
	port int,
) (*sandboxclient.SandboxExposedPort, error) {
	return s.repo.ExposePort(ctx, sandboxID, port)
}

// HidePort removes a port's public URL. Hiding a port that is not exposed
// succeeds.
func (s *Service) HidePort(ctx context.Context, sandboxID string, port int) (*HiddenPort, error) {
	if err := s.repo.HidePort(ctx, sandboxID, port); err != nil {
		return nil, err
	}
	return &HiddenPort{SandboxID: sandboxID, Port: port}, nil
}

// ListPorts returns a sandbox's exposed ports and their URLs.
func (s *Service) ListPorts(ctx context.Context, sandboxID string) (sandboxclient.SandboxExposedPorts, error) {
	return s.repo.ListPorts(ctx, sandboxID)
}

func (r *Repo) ExposePort(ctx context.Context, sandboxID string, port int) (*sandboxclient.SandboxExposedPort, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.ExposeSandboxPortWithResponse(
		ctx,
		sandboxID,
		port,
		&client.ExposeSandboxPortParams{OwnerId: &workspace},
	)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil {
		return nil, fmt.Errorf("expose sandbox port: success response missing port body")
	}

	return resp.JSON200, nil
}

func (r *Repo) HidePort(ctx context.Context, sandboxID string, port int) error {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return err
	}

	resp, err := r.client.HideSandboxPortWithResponse(
		ctx,
		sandboxID,
		port,
		&client.HideSandboxPortParams{OwnerId: &workspace},
	)
	if err != nil {
		return err
	}

	return client.ErrorFromResponse(resp)
}

// ListPorts returns an empty list rather than nil, so JSON output is [].
func (r *Repo) ListPorts(ctx context.Context, sandboxID string) (sandboxclient.SandboxExposedPorts, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	resp, err := r.client.ListSandboxPortsWithResponse(
		ctx,
		sandboxID,
		&client.ListSandboxPortsParams{OwnerId: &workspace},
	)
	if err != nil {
		return nil, err
	}
	if err := client.ErrorFromResponse(resp); err != nil {
		return nil, err
	}
	if resp.JSON200 == nil || *resp.JSON200 == nil {
		return sandboxclient.SandboxExposedPorts{}, nil
	}

	return *resp.JSON200, nil
}
