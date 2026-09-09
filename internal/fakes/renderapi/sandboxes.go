package renderapi

import (
	"time"

	"github.com/render-oss/cli/internal/testids"
	sandboxesclient "github.com/render-oss/cli/pkg/client/sandboxes"
)

type SandboxResource struct {
	Resource[*sandboxesclient.Sandbox]
}

func NewSandbox(sb sandboxesclient.Sandbox) *sandboxesclient.Sandbox {
	if sb.Id == "" {
		sb.Id = testids.RandomSandboxID()
	}
	if sb.Plan == "" {
		sb.Plan = sandboxesclient.Starter
	}
	if sb.Region == "" {
		sb.Region = "oregon"
	}
	if sb.Status == "" {
		sb.Status = sandboxesclient.SandboxStatusRunning
	}
	if sb.NetworkPolicy.Default == "" {
		sb.NetworkPolicy.Default = sandboxesclient.AllowAll
	}
	if sb.TimeoutSeconds == 0 {
		sb.TimeoutSeconds = 3600
	}
	if sb.CreatedAt.IsZero() {
		sb.CreatedAt = time.Now()
	}
	return &sb
}

func (s *Server) sandboxByID(id string) (*sandboxesclient.Sandbox, bool) {
	for _, sb := range s.Sandboxes.Instances {
		if sb.Id == id {
			return sb, true
		}
	}
	return nil, false
}
