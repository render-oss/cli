package instance

import (
	"context"
	"fmt"
	"sort"

	"github.com/render-oss/cli/pkg/client"
)

type Repo struct {
	client *client.ClientWithResponses
}

func NewRepo(c *client.ClientWithResponses) *Repo {
	return &Repo{client: c}
}

// ListInstancesForService returns instances for a service, sorted by creation time (newest first)
func (r *Repo) ListInstancesForService(ctx context.Context, serviceID string) ([]*client.ServiceInstance, error) {
	resp, err := r.client.ListInstancesWithResponse(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to list instances: %s", resp.Status())
	}

	if resp.JSON200 == nil {
		return nil, fmt.Errorf("empty response")
	}

	// Convert to pointers and sort by creation time (newest first)
	instances := make([]*client.ServiceInstance, len(*resp.JSON200))
	for i := range *resp.JSON200 {
		instances[i] = &(*resp.JSON200)[i]
	}

	sort.Slice(instances, func(i, j int) bool {
		return instances[i].CreatedAt.After(instances[j].CreatedAt)
	})

	return instances, nil
}
