package sandbox

import (
	"context"

	"github.com/render-oss/cli/pkg/client"
	sandboxclient "github.com/render-oss/cli/pkg/client/sandboxes"
	"github.com/render-oss/cli/pkg/config"
)

// Service holds the business logic for managing sandboxes: applying default
// filters, assembling request bodies, and resolving workspace scope before
// delegating to the Repo.
type Service struct {
	repo *Repo
}

func NewService(repo *Repo) *Service {
	return &Service{repo: repo}
}

// CreateInput describes the parameters for creating a sandbox. Empty/zero
// fields fall back to the API defaults.
type CreateInput struct {
	Plan    string
	Region  string
	Timeout int
}

// List returns sandboxes in the active workspace. When statuses is non-empty it
// filters by those statuses; otherwise terminated sandboxes are excluded unless
// all is true.
func (s *Service) List(ctx context.Context, statuses []string, all bool) ([]*sandboxclient.Sandbox, error) {
	params := &client.ListSandboxesParams{}

	if len(statuses) > 0 {
		filtered := make([]sandboxclient.SandboxStatus, len(statuses))
		for i, status := range statuses {
			filtered[i] = sandboxclient.SandboxStatus(status)
		}
		params.Status = &filtered
	} else if !all {
		// By default, exclude terminated sandboxes.
		params.Status = &[]sandboxclient.SandboxStatus{
			sandboxclient.SandboxStatusCreating,
			sandboxclient.SandboxStatusRunning,
			sandboxclient.SandboxStatusErrored,
		}
	}

	return s.repo.ListSandboxes(ctx, params)
}

// Create resolves the active workspace, builds the request body from input, and
// creates a sandbox. onEvent, when non-nil, is invoked for each streamed status
// update during creation.
func (s *Service) Create(ctx context.Context, input CreateInput, onEvent func(*sandboxclient.Sandbox)) (*sandboxclient.Sandbox, error) {
	workspace, err := config.WorkspaceID()
	if err != nil {
		return nil, err
	}

	body := client.CreateSandboxJSONRequestBody{OwnerId: workspace}

	if input.Plan != "" {
		plan := sandboxclient.SandboxPlan(input.Plan)
		body.Plan = &plan
	}
	if input.Region != "" {
		body.Region = &input.Region
	}
	if input.Timeout > 0 {
		body.TimeoutSeconds = &input.Timeout
	}

	return s.repo.CreateSandbox(ctx, body, onEvent)
}

// Get returns a single sandbox by ID.
func (s *Service) Get(ctx context.Context, id string) (*sandboxclient.Sandbox, error) {
	return s.repo.GetSandbox(ctx, id)
}

// ExecStream runs a command in a running sandbox, streams output events, and
// returns the remote process exit code.
func (s *Service) ExecStream(ctx context.Context, id string, command string, onOutput func(*ExecOutputEvent) error) (int, error) {
	return s.repo.ExecSandboxStream(ctx, id, command, onOutput)
}

// Terminate terminates a running sandbox.
func (s *Service) Terminate(ctx context.Context, id string) error {
	return s.repo.TerminateSandbox(ctx, id)
}
