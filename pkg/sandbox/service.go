package sandbox

import (
	"context"
	"slices"

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
	SandboxGroupID string
	Plan           string
	Region         string
	Timeout        *int
	IdleTimeout    *int
	NetworkPolicy  string
	// EgressRules are the destinations reachable under an allow-list
	// NetworkPolicy.
	EgressRules  []sandboxclient.SandboxEgressRule
	Env          map[string]string
	SnapshotID   string
	SnapshotName *string
	// ImageID or ImageName starts the sandbox from an imported image. The API
	// calls images builds.
	ImageID   string
	ImageName *string
}

// allSandboxStatuses is every sandbox status. --all sends the full set so the
// server includes terminated sandboxes, which it otherwise excludes unless a
// status filter names them.
var allSandboxStatuses = []sandboxclient.SandboxStatus{
	sandboxclient.SandboxStatusCreating,
	sandboxclient.SandboxStatusRunning,
	sandboxclient.SandboxStatusSuspending,
	sandboxclient.SandboxStatusSuspended,
	sandboxclient.SandboxStatusResuming,
	sandboxclient.SandboxStatusErrored,
	sandboxclient.SandboxStatusTerminated,
}

// AllStatuses returns every sandbox status, in lifecycle order.
func AllStatuses() []sandboxclient.SandboxStatus {
	return slices.Clone(allSandboxStatuses)
}

// List returns sandboxes in the active workspace. When statuses is non-empty it
// filters by those statuses. Otherwise it sends no status filter, so the server
// applies its default (exclude terminated) — unless all is true, which requests
// every status to include terminated sandboxes too.
func (s *Service) List(ctx context.Context, statuses []string, all bool, groupID string) ([]*sandboxclient.Sandbox, error) {
	params := &client.ListSandboxesParams{}
	if groupID != "" {
		params.SandboxGroupId = &groupID
	}

	if len(statuses) > 0 {
		filtered := make([]sandboxclient.SandboxStatus, len(statuses))
		for i, status := range statuses {
			filtered[i] = sandboxclient.SandboxStatus(status)
		}
		params.Status = &filtered
	} else if all {
		params.Status = &allSandboxStatuses
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
	if input.SandboxGroupID != "" {
		body.SandboxGroupId = &input.SandboxGroupID
	}

	if input.Plan != "" {
		plan := sandboxclient.SandboxPlan(input.Plan)
		body.Plan = &plan
	}
	if input.Timeout != nil {
		body.TimeoutSeconds = input.Timeout
	}
	if input.IdleTimeout != nil {
		body.IdleTimeoutSeconds = input.IdleTimeout
	}
	if input.NetworkPolicy != "" {
		policyType := sandboxclient.SandboxNetworkPolicyType(input.NetworkPolicy)
		body.NetworkPolicy = &sandboxclient.SandboxNetworkPolicyPOST{Type: &policyType}
		if len(input.EgressRules) > 0 {
			body.NetworkPolicy.Rules = &input.EgressRules
		}
	}
	if len(input.Env) > 0 {
		body.Env = &input.Env
	}
	if input.SnapshotID != "" {
		body.SnapshotId = &input.SnapshotID
	}
	if input.SnapshotName != nil {
		body.SnapshotName = input.SnapshotName
	}
	if input.ImageID != "" {
		body.BuildId = &input.ImageID
	}
	if input.ImageName != nil {
		body.BuildName = input.ImageName
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
